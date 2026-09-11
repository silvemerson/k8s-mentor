// Package rag implementa a Fase 2: RAG sobre runbooks/postmortems locais.
// Indexa arquivos Markdown num vector DB (collection separada da memória de
// incidentes) e recupera os trechos mais relevantes pra um diagnóstico.
package rag

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/silvemerson/k8s-mentor/internal/ollama"
	"github.com/silvemerson/k8s-mentor/internal/vectordb"
)

const Collection = "runbooks"

type Store struct {
	embeddings *ollama.Client
	vectors    *vectordb.Client
	collection string
}

func New(embeddings *ollama.Client, vectors *vectordb.Client) *Store {
	return &Store{embeddings: embeddings, vectors: vectors, collection: Collection}
}

// Chunk é uma seção de um runbook — o nível de granularidade que embedamos e
// buscamos, em vez do arquivo inteiro (o documento todo diluiria demais o
// vetor; um erro específico geralmente casa com UMA seção, não o runbook todo).
type Chunk struct {
	Path    string
	Heading string
	Text    string
}

// IndexDir varre recursivamente um diretório por arquivos .md, quebra cada um
// em seções por heading (título de nível 1 ou 2) e grava os embeddings no
// Qdrant. IDs determinísticos (path+heading) tornam reindexar idempotente:
// o mesmo trecho sobrescreve o ponto anterior em vez de duplicar.
func (s *Store) IndexDir(ctx context.Context, dir string) (int, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("varrer %q: %w", dir, err)
	}

	var chunks []Chunk
	for _, file := range files {
		fileChunks, err := chunkMarkdown(file)
		if err != nil {
			return 0, err
		}
		chunks = append(chunks, fileChunks...)
	}
	if len(chunks) == 0 {
		return 0, fmt.Errorf("nenhum runbook (.md) encontrado em %q", dir)
	}

	points := make([]vectordb.Point, 0, len(chunks))
	for i, chunk := range chunks {
		vector, err := s.embeddings.Embed(ctx, embeddingText(chunk))
		if err != nil {
			return 0, fmt.Errorf("gerar embedding de %s#%s: %w", chunk.Path, chunk.Heading, err)
		}
		if i == 0 {
			if err := s.vectors.EnsureCollection(ctx, s.collection, len(vector)); err != nil {
				return 0, fmt.Errorf("preparar collection: %w", err)
			}
		}
		payload := map[string]any{"path": chunk.Path, "heading": chunk.Heading, "text": chunk.Text}
		points = append(points, vectordb.Point{ID: chunkID(chunk), Vector: vector, Payload: payload})
	}
	if err := s.vectors.Upsert(ctx, s.collection, points); err != nil {
		return 0, fmt.Errorf("gravar trechos no Qdrant: %w", err)
	}
	return len(points), nil
}

type Match struct {
	Score   float64 `json:"score"`
	Path    string  `json:"path"`
	Heading string  `json:"heading"`
	Text    string  `json:"text"`
}

// Retrieve busca os trechos de runbook mais relevantes pro texto de consulta
// (tipicamente mensagem + explicação do diagnóstico).
func (s *Store) Retrieve(ctx context.Context, query string, limit int, threshold float64) ([]Match, error) {
	vector, err := s.embeddings.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gerar embedding da consulta: %w", err)
	}
	matches, err := s.vectors.Search(ctx, s.collection, vector, limit, threshold)
	if err != nil {
		return nil, fmt.Errorf("buscar runbooks: %w", err)
	}
	out := make([]Match, 0, len(matches))
	for _, match := range matches {
		out = append(out, Match{
			Score:   match.Score,
			Path:    stringPayload(match.Payload, "path"),
			Heading: stringPayload(match.Payload, "heading"),
			Text:    stringPayload(match.Payload, "text"),
		})
	}
	return out, nil
}

// chunkMarkdown quebra um arquivo por headings de nível 1 (#) ou 2 (##). O
// título mais recente vira o "heading" do trecho seguinte; conteúdo antes do
// primeiro heading (raro) fica sem heading, mas ainda é indexado.
func chunkMarkdown(path string) ([]Chunk, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir %q: %w", path, err)
	}
	defer file.Close()

	var chunks []Chunk
	var heading string
	var buf strings.Builder
	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text != "" {
			chunks = append(chunks, Chunk{Path: path, Heading: heading, Text: text})
		}
		buf.Reset()
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "## "):
			flush()
			heading = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		case strings.HasPrefix(line, "# "):
			flush()
			heading = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		default:
			buf.WriteString(line)
			buf.WriteString("\n")
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ler %q: %w", path, err)
	}
	return chunks, nil
}

func embeddingText(chunk Chunk) string {
	return fmt.Sprintf("documento: %s\nseção: %s\n%s", filepath.Base(chunk.Path), chunk.Heading, chunk.Text)
}

func chunkID(chunk Chunk) uint64 {
	sum := sha256.Sum256([]byte(chunk.Path + "\x00" + chunk.Heading))
	id := binary.BigEndian.Uint64(sum[:8])
	if id == 0 {
		return 1
	}
	return id
}

func stringPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}
