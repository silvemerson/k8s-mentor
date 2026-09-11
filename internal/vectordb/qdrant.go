// Package vectordb contém a integração HTTP mínima com o Qdrant.
package vectordb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

type Point struct {
	ID      uint64         `json:"id"`
	Vector  []float64      `json:"vector"`
	Payload map[string]any `json:"payload"`
}
type Match struct {
	ID      uint64         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

// EnsureCollection cria a collection somente quando ela ainda não existe.
func (c *Client) EnsureCollection(ctx context.Context, name string, dimensions int) error {
	if dimensions <= 0 {
		return fmt.Errorf("dimensão do vetor deve ser positiva")
	}
	status, err := c.request(ctx, http.MethodGet, "/collections/"+url.PathEscape(name), nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("consultar collection %q: status HTTP %d", name, status)
	}
	status, err = c.request(ctx, http.MethodPut, "/collections/"+url.PathEscape(name), map[string]any{"vectors": map[string]any{"size": dimensions, "distance": "Cosine"}}, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("criar collection %q: status HTTP %d", name, status)
	}
	return nil
}

func (c *Client) Upsert(ctx context.Context, collection string, points []Point) error {
	status, err := c.request(ctx, http.MethodPut, "/collections/"+url.PathEscape(collection)+"/points?wait=true", map[string]any{"points": points}, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("gravar pontos no Qdrant: status HTTP %d", status)
	}
	return nil
}

func (c *Client) Search(ctx context.Context, collection string, vector []float64, limit int, threshold float64) ([]Match, error) {
	// A API de query do Qdrant (/points/query) envolve os pontos em
	// result.points, diferente da antiga /points/search que devolvia
	// result como array direto. Ver https://qdrant.tech/documentation/concepts/search/#query-api.
	var response struct {
		Result struct {
			Points []Match `json:"points"`
		} `json:"result"`
	}
	status, err := c.request(ctx, http.MethodPost, "/collections/"+url.PathEscape(collection)+"/points/query", map[string]any{"query": vector, "limit": limit, "with_payload": true, "score_threshold": threshold}, &response)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("buscar pontos no Qdrant: status HTTP %d", status)
	}
	return response.Result.Points, nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any, destination any) (int, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, fmt.Errorf("criar requisição para Qdrant: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("chamar Qdrant: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return resp.StatusCode, err
	}
	if destination != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(responseBody, destination); err != nil {
			return resp.StatusCode, fmt.Errorf("decodificar resposta do Qdrant: %w", err)
		}
	}
	return resp.StatusCode, nil
}
