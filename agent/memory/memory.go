// Package memory implementa a memória vetorial de incidentes da Fase 1.
package memory

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
	"github.com/silvemerson/k8s-mentor/internal/ollama"
	"github.com/silvemerson/k8s-mentor/internal/vectordb"
)

const Collection = "incidents"

// SimilarityThreshold foi recalibrado de 0.80 pra 0.85 depois de testar
// contra saída REAL do K8sGPT (não texto forjado à mão): com 0.80, um
// "Deployment com 0 réplicas disponíveis" e um "DemoWorkload em status
// Failed" — recursos completamente diferentes, sem relação nenhuma — davam
// 0.80-0.82 de similaridade, inflando "recorrência" com falsos positivos.
// Recorrências genuínas do mesmo tipo de recurso ficaram em 0.85-0.94. O
// efeito colateral aceito: "mesmo padrão de erro em outro serviço" (ex: dois
// serviços diferentes em CrashLoopBackOff) também cai abaixo do corte —
// trade-off deliberado a favor de precisão ("isso" recorrendo) sobre recall
// ("algo parecido com isso" recorrendo).
const SimilarityThreshold = 0.85

type Store struct {
	embeddings *ollama.Client
	vectors    *vectordb.Client
	collection string
}

func New(embeddings *ollama.Client, vectors *vectordb.Client) *Store {
	return &Store{embeddings: embeddings, vectors: vectors, collection: Collection}
}

type History struct {
	Recurrence int               `json:"recurrence"`
	Similar    []SimilarIncident `json:"similar_incidents"`
}
type SimilarIncident struct {
	Score     float64 `json:"score"`
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Namespace string  `json:"namespace,omitempty"`
	Message   string  `json:"message"`
	Timestamp string  `json:"timestamp"`
}

// Remember busca incidentes semelhantes antes de registrar o evento atual.
// Assim, recurrence descreve ocorrências anteriores, não inclui esta execução.
func (s *Store) Remember(ctx context.Context, event k8sgpt.Event, withHistory bool) (History, error) {
	vector, err := s.embeddings.Embed(ctx, embeddingText(event))
	if err != nil {
		return History{}, fmt.Errorf("gerar embedding: %w", err)
	}
	if err := s.vectors.EnsureCollection(ctx, s.collection, len(vector)); err != nil {
		return History{}, fmt.Errorf("preparar collection: %w", err)
	}
	history := History{}
	if withHistory {
		matches, err := s.vectors.Search(ctx, s.collection, vector, 20, SimilarityThreshold)
		if err != nil {
			return History{}, fmt.Errorf("consultar memória: %w", err)
		}
		for _, match := range matches {
			if event.Namespace != "" && stringPayload(match.Payload, "namespace") != event.Namespace {
				continue
			}
			history.Similar = append(history.Similar, SimilarIncident{Score: match.Score, Kind: stringPayload(match.Payload, "kind"), Name: stringPayload(match.Payload, "name"), Namespace: stringPayload(match.Payload, "namespace"), Message: stringPayload(match.Payload, "message"), Timestamp: stringPayload(match.Payload, "timestamp")})
		}
		history.Recurrence = len(history.Similar)
	}
	payload := map[string]any{"source": event.Source, "kind": event.Kind, "name": event.Name, "namespace": event.Namespace, "message": event.Message, "explanation": event.Explanation, "timestamp": event.Timestamp.UTC().Format("2006-01-02T15:04:05Z")}
	if err := s.vectors.Upsert(ctx, s.collection, []vectordb.Point{{ID: eventID(event), Vector: vector, Payload: payload}}); err != nil {
		return History{}, fmt.Errorf("registrar incidente: %w", err)
	}
	return history, nil
}

func embeddingText(event k8sgpt.Event) string {
	return strings.Join([]string{"fonte: " + event.Source, "recurso: " + event.Kind, "namespace: " + event.Namespace, "diagnóstico: " + event.Message, "explicação: " + event.Explanation}, "\n")
}
func eventID(event k8sgpt.Event) uint64 {
	sum := sha256.Sum256([]byte(event.Source + "\x00" + event.Kind + "\x00" + event.Name + "\x00" + event.Namespace + "\x00" + event.Message + "\x00" + event.Timestamp.UTC().Format("2006-01-02T15:04:05.999999999Z")))
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
