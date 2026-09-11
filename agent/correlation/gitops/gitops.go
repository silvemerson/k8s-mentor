// Package gitops implementa a Fase 3: correlação temporal entre um erro e o
// histórico de rollout do Deployment dono do recurso. Não depende de um
// ArgoCD/Flux de verdade — lê o histórico nativo do Kubernetes (ReplicaSets),
// que já existe em qualquer cluster, e opcionalmente enriquece com um
// annotation de commit (ver internal/k8s.CommitAnnotation) quando presente,
// do jeito que uma ferramenta de GitOps real também faria.
package gitops

import (
	"context"
	"time"

	"github.com/silvemerson/k8s-mentor/internal/k8s"
	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
)

// Window é o intervalo, antes do erro, em que um deploy é tratado como
// possível causa raiz (Suspect: true). Fora dessa janela ainda devolvemos o
// deploy mais próximo encontrado — só não o marcamos como suspeito.
const Window = 30 * time.Minute

type Result struct {
	Correlated bool       `json:"correlated"`
	Suspect    bool       `json:"suspect_recent_deploy"`
	Deployment string     `json:"deployment,omitempty"`
	ReplicaSet string     `json:"replica_set,omitempty"`
	Revision   string     `json:"revision,omitempty"`
	Commit     string     `json:"commit,omitempty"`
	DeployedAt *time.Time `json:"deployed_at,omitempty"` // ponteiro só pra omitempty funcionar quando zero-value (time.Time{} não conta como "vazio" pro encoding/json)
	GapSeconds float64    `json:"gap_seconds,omitempty"`

	// Revisão imediatamente anterior à correlacionada — candidata natural
	// pra rollback (Fase 5, agent/decision). Vazio se não houver revisão
	// mais antiga no histórico (ex: é a primeira revisão do Deployment).
	PreviousReplicaSet string `json:"previous_replica_set,omitempty"`
	PreviousRevision   string `json:"previous_revision,omitempty"`
}

// Correlate cruza o timestamp do evento com o histórico de rollout do
// Deployment dono do recurso, achando a revisão implantada mais próxima no
// tempo (antes ou depois do erro — a mais próxima em módulo). Kinds sem
// Deployment associado, ou sem histórico visível (RBAC insuficiente, recurso
// sem owner, etc.), resultam em Correlated: false — não é erro, nem todo
// recurso tem rollout pra correlacionar.
func Correlate(ctx context.Context, client *k8s.Client, event k8sgpt.Event) (Result, error) {
	deploymentName, history, err := client.RolloutHistory(ctx, event.Namespace, event.Kind, event.Name)
	if err != nil {
		return Result{}, err
	}
	if len(history) == 0 {
		return Result{}, nil
	}

	// history vem ordenado do mais recente pro mais antigo (ver
	// internal/k8s.RolloutHistory) — guardamos o índice, não só o valor,
	// porque o item logo depois dele nessa ordem é a revisão anterior.
	closestIdx := 0
	closestGap := event.Timestamp.Sub(history[0].CreatedAt).Abs()
	for i, revision := range history[1:] {
		gap := event.Timestamp.Sub(revision.CreatedAt).Abs()
		if gap < closestGap {
			closestIdx = i + 1
			closestGap = gap
		}
	}
	closest := history[closestIdx]

	gap := event.Timestamp.Sub(closest.CreatedAt)
	deployedAt := closest.CreatedAt
	result := Result{
		Correlated: true,
		Suspect:    gap >= 0 && gap <= Window,
		Deployment: deploymentName,
		ReplicaSet: closest.ReplicaSet,
		Revision:   closest.Revision,
		Commit:     closest.Commit,
		DeployedAt: &deployedAt,
		GapSeconds: gap.Seconds(),
	}
	if closestIdx+1 < len(history) {
		previous := history[closestIdx+1]
		result.PreviousReplicaSet = previous.ReplicaSet
		result.PreviousRevision = previous.Revision
	}
	return result, nil
}
