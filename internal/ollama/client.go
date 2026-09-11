// Package ollama implementa o cliente HTTP do Ollama usado pelo agente.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client conversa somente com uma instância local do Ollama.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

func New(baseURL, model string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), model: model, http: &http.Client{Timeout: 90 * time.Second}}
}

// Embed gera um vetor para um texto usando a API atual do Ollama.
func (c *Client) Embed(ctx context.Context, input string) ([]float64, error) {
	var response struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	if err := c.do(ctx, "/api/embed", map[string]any{"model": c.model, "input": input}, &response); err != nil {
		return nil, err
	}
	if len(response.Embeddings) != 1 || len(response.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("resposta de embedding inválida do Ollama")
	}
	return response.Embeddings[0], nil
}

// Chat gera uma resposta sem streaming. A Fase 1 ainda não a usa para tomar
// decisões; ela deixa o cliente pronto para a síntese das fases seguintes.
func (c *Client) Chat(ctx context.Context, messages []Message) (string, error) {
	var response struct {
		Message Message `json:"message"`
	}
	if err := c.do(ctx, "/api/chat", map[string]any{"model": c.model, "messages": messages, "stream": false}, &response); err != nil {
		return "", err
	}
	return response.Message.Content, nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *Client) do(ctx context.Context, path string, payload any, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("codificar requisição para Ollama: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("criar requisição para Ollama: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("chamar Ollama: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("ler resposta do Ollama: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Ollama respondeu %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	if err := json.Unmarshal(responseBody, destination); err != nil {
		return fmt.Errorf("decodificar resposta do Ollama: %w", err)
	}
	return nil
}
