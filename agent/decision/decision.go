// Package decision implementa a Fase 5: ação supervisionada. Sugere uma
// remediação com base em regras claras sobre os sinais já coletados pelos
// outros pilares — não é o LLM "adivinhando" uma ação. E, mais importante:
// este pacote só SUGERE. A execução de verdade mora em internal/k8s
// (RollbackDeployment/RestartDeployment) e só é alcançável pelo subcomando
// `k8s-mentor apply-action`, que exige aprovação explícita (--yes) — nunca
// pelo `analyze`, que é estritamente leitura.
package decision

import (
	"fmt"

	"github.com/silvemerson/k8s-mentor/agent/correlation/gitops"
	"github.com/silvemerson/k8s-mentor/agent/correlation/metrics"
	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
)

type Suggestion struct {
	Type       string `json:"type"` // "rollback" | "restart" | "none"
	Reason     string `json:"reason"`
	Namespace  string `json:"namespace,omitempty"`
	Deployment string `json:"deployment,omitempty"`
	ReplicaSet string `json:"replica_set,omitempty"` // alvo do rollback, quando type == "rollback"
	Revision   string `json:"revision,omitempty"`
	Command    string `json:"equivalent_command,omitempty"` // só documentação — apply-action NÃO executa isso via shell
}

// Suggest decide qual ação propor, se alguma, a partir dos resultados de
// correlação já calculados (podem ser nil se as flags correspondentes não
// foram passadas pro analyze). Prioridade: um deploy recente suspeito é uma
// causa raiz mais acionável (rollback resolve de verdade) do que um
// vazamento de memória (restart só é paliativo) — por isso checado primeiro.
func Suggest(event k8sgpt.Event, gitopsResult *gitops.Result, metricsResult *metrics.Result) Suggestion {
	deployment := ""
	if gitopsResult != nil && gitopsResult.Correlated {
		deployment = gitopsResult.Deployment
	}

	if gitopsResult != nil && gitopsResult.Correlated && gitopsResult.Suspect && gitopsResult.PreviousReplicaSet != "" {
		return Suggestion{
			Type: "rollback",
			Reason: fmt.Sprintf(
				"deploy recente (revisão %s, %.0fs antes do erro) é a causa mais provável — voltar pra revisão %s",
				gitopsResult.Revision, gitopsResult.GapSeconds, gitopsResult.PreviousRevision,
			),
			Namespace:  event.Namespace,
			Deployment: deployment,
			ReplicaSet: gitopsResult.PreviousReplicaSet,
			Revision:   gitopsResult.PreviousRevision,
			Command:    fmt.Sprintf("kubectl rollout undo deployment/%s -n %s --to-revision=%s", deployment, event.Namespace, gitopsResult.PreviousRevision),
		}
	}

	if metricsResult != nil && metricsResult.Correlated && metricsResult.Growing {
		if deployment == "" {
			return Suggestion{
				Type:   "none",
				Reason: "memória em crescimento sustentado, mas o Deployment não foi resolvido (rode --with-gitops junto de --with-metrics) — sem alvo seguro pra sugerir restart",
			}
		}
		return Suggestion{
			Type:       "restart",
			Reason:     "memória em crescimento sustentado (padrão de vazamento) — reiniciar é só paliativo, não corrige a causa raiz; ver runbook (--with-runbooks) pra correção de verdade",
			Namespace:  event.Namespace,
			Deployment: deployment,
			Command:    fmt.Sprintf("kubectl rollout restart deployment/%s -n %s", deployment, event.Namespace),
		}
	}

	return Suggestion{Type: "none", Reason: "nenhum sinal coletado sustenta uma sugestão de ação segura"}
}
