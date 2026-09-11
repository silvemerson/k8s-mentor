// Package metrics implementa a Fase 4: correlação com métricas do
// Prometheus. Em vez de só reportar o estado no momento da falha (ex: "o pod
// tomou OOMKilled com 512Mi de limite"), busca a janela de tempo anterior ao
// erro e detecta tendência de crescimento — a diferença entre "estourou do
// nada" e "vinha subindo continuamente".
package metrics

import (
	"context"
	"fmt"

	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
	"github.com/silvemerson/k8s-mentor/internal/prometheus"
	"time"
)

// Window é quanto tempo antes do erro olhamos pra trás. Step é o intervalo
// entre amostras pedido ao Prometheus — 5s pra caber várias amostras mesmo
// num ciclo de crash curto (o cenário de demo estoura em ~60s).
const (
	Window       = 15 * time.Minute
	Step         = 5 * time.Second
	MinSamples   = 3    // menos que isso não dá pra falar de tendência com confiança
	GrowthMargin = 0.20 // crescimento >20% no intervalo observado conta como "subindo"
)

type Result struct {
	Correlated          bool    `json:"correlated"`
	Growing             bool    `json:"growing_trend"`
	SlopeBytesPerSecond float64 `json:"slope_bytes_per_second,omitempty"`
	FirstValueBytes     float64 `json:"first_value_bytes,omitempty"`
	LastValueBytes      float64 `json:"last_value_bytes,omitempty"`
	SampleCount         int     `json:"sample_count,omitempty"`
	WindowSeconds       float64 `json:"window_seconds,omitempty"`
}

// Correlate busca container_memory_working_set_bytes do pod na janela
// anterior ao evento e ajusta uma reta por mínimos quadrados sobre os
// pontos. Só Pods têm essa métrica de forma direta — outros kinds resultam
// em Correlated: false, sem erro. Amostras insuficientes (poucos pontos, ou
// o container nunca chegou a rodar tempo suficiente) também resultam em
// Correlated: false — não é erro, é dado insuficiente pra afirmar tendência.
func Correlate(ctx context.Context, client *prometheus.Client, event k8sgpt.Event) (Result, error) {
	if event.Kind != "Pod" {
		return Result{}, nil
	}

	query := fmt.Sprintf(
		`container_memory_working_set_bytes{namespace=%q, pod=%q, image!~".*pause.*"}`,
		event.Namespace, event.Name,
	)
	samples, err := client.QueryRange(ctx, query, event.Timestamp.Add(-Window), event.Timestamp, Step)
	if err != nil {
		return Result{}, fmt.Errorf("consultar Prometheus: %w", err)
	}
	if len(samples) < MinSamples {
		return Result{}, nil
	}

	slope := leastSquaresSlope(samples)
	first := samples[0].Value
	last := samples[len(samples)-1].Value
	windowSeconds := samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp).Seconds()

	// Crescimento relativo a partir de "first" quebra exatamente no caso mais
	// óbvio de vazamento: container recém-reiniciado, memória começando perto
	// de zero. "first > 0" barrava esse caso inteiro, reportando
	// growing=false mesmo com a memória subindo até o limite e OOMKilled.
	// Em vez disso, projeta o quanto a reta ajustada cresceu ao longo da
	// janela observada (slope * duração) e compara contra o valor final —
	// não depende de "first" ser um baseline confiável.
	projectedGrowth := slope * windowSeconds
	growing := slope > 0 && last > 0 && projectedGrowth/last > GrowthMargin

	return Result{
		Correlated:          true,
		Growing:             growing,
		SlopeBytesPerSecond: slope,
		FirstValueBytes:     first,
		LastValueBytes:      last,
		SampleCount:         len(samples),
		WindowSeconds:       windowSeconds,
	}, nil
}

// leastSquaresSlope ajusta uma reta y = a + b*x pelos mínimos quadrados,
// com x em segundos desde a primeira amostra, e devolve b (a inclinação —
// bytes por segundo, no nosso caso).
func leastSquaresSlope(samples []prometheus.Sample) float64 {
	n := float64(len(samples))
	t0 := samples[0].Timestamp
	var sumX, sumY, sumXY, sumXX float64
	for _, s := range samples {
		x := s.Timestamp.Sub(t0).Seconds()
		y := s.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denominator
}
