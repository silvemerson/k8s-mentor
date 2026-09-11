// Package prometheus contém o cliente HTTP mínimo pra API de consulta do
// Prometheus — só o suficiente pra Fase 4 (correlação com métricas) buscar
// uma série temporal antes de um erro.
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

type Sample struct {
	Timestamp time.Time
	Value     float64
}

// QueryRange busca uma série temporal via /api/v1/query_range. Quando o
// PromQL casa com mais de uma série (cada reinício de container ganha um
// cgroup/id novo no cAdvisor, então uma janela larga cobrindo vários ciclos
// de crash acaba trazendo uma série por instância), devolve a série cuja
// ÚLTIMA amostra é mais próxima do fim da janela — a instância que estava
// rodando de fato mais perto do evento observado. Escolher pela série com
// mais pontos (heurística anterior) podia pegar um ciclo antigo e mais
// "cheio" de amostras, mesmo sem relação direta com a falha analisada —
// esse era exatamente o tipo de erro que uma janela de 15min sobre um
// cenário que crasha a cada ~60-90s expunha na prática.
func (c *Client) QueryRange(ctx context.Context, promql string, start, end time.Time, step time.Duration) ([]Sample, error) {
	values := url.Values{}
	values.Set("query", promql)
	values.Set("start", strconv.FormatInt(start.Unix(), 10))
	values.Set("end", strconv.FormatInt(end.Unix(), 10))
	values.Set("step", strconv.FormatFloat(step.Seconds(), 'f', -1, 64))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/query_range?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("criar requisição pro Prometheus: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chamar Prometheus: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Result []struct {
				Values [][2]any `json:"values"` // [timestamp float64, value string]
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decodificar resposta do Prometheus: %w", err)
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("consulta ao Prometheus falhou: %s", body.Error)
	}

	var mostRecent []Sample
	for _, series := range body.Data.Result {
		samples := make([]Sample, 0, len(series.Values))
		for _, point := range series.Values {
			ts, ok := point[0].(float64)
			if !ok {
				continue
			}
			raw, ok := point[1].(string)
			if !ok {
				continue
			}
			value, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			samples = append(samples, Sample{Timestamp: time.Unix(int64(ts), 0), Value: value})
		}
		if len(samples) == 0 {
			continue
		}
		if len(mostRecent) == 0 || samples[len(samples)-1].Timestamp.After(mostRecent[len(mostRecent)-1].Timestamp) {
			mostRecent = samples
		}
	}
	sort.Slice(mostRecent, func(i, j int) bool { return mostRecent[i].Timestamp.Before(mostRecent[j].Timestamp) })
	return mostRecent, nil
}
