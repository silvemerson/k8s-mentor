// Package k8s contém o acesso mínimo à API do Kubernetes usado pelo
// k8s-mentor — hoje, só o suficiente pra correlação GitOps (Fase 3) resolver
// o histórico de rollout de um Deployment.
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Convenção própria do k8s-mentor: uma ferramenta de GitOps real (ArgoCD,
// Flux, ou um pipeline de CI simples) anota o POD TEMPLATE do Deployment
// (não o Deployment em si) com o commit/versão que está sendo implantado.
// É o pod template que fica "congelado" em cada ReplicaSet — por isso é ali,
// e não numa annotation solta no Deployment, que dá pra recuperar qual
// commit estava valendo em cada revisão passada.
const CommitAnnotation = "k8s-mentor.dev/commit"

type Client struct {
	clientset *kubernetes.Clientset
}

// New resolve a configuração do cluster automaticamente: usa a config
// in-cluster quando rodando como Pod (o caso normal em produção/demo — e o
// único caminho imune ao problema abaixo, já que fala sempre com a API do
// próprio cluster onde o Pod está, sem depender de contexto nenhum).
//
// Fora do cluster (uso local do CLI), NUNCA confiamos silenciosamente no
// "current-context" do kubeconfig do host: esse arquivo pode ter clusters de
// produção/homolog de verdade configurados, e um current-context errado ali
// faria a correlação GitOps ler dados do cluster errado sem nenhum aviso.
// Por isso exigimos K8S_MENTOR_KUBE_CONTEXT explícito nesse caso.
func New() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = kubeconfigConfig()
		if err != nil {
			return nil, fmt.Errorf("carregar config do Kubernetes (nem in-cluster nem kubeconfig): %w", err)
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("criar cliente do Kubernetes: %w", err)
	}
	return &Client{clientset: clientset}, nil
}

func kubeconfigConfig() (*rest.Config, error) {
	kubeContext := os.Getenv("K8S_MENTOR_KUBE_CONTEXT")
	if kubeContext == "" {
		return nil, fmt.Errorf("fora do cluster, defina K8S_MENTOR_KUBE_CONTEXT explicitamente — nunca confiamos no current-context do kubeconfig por padrão (pode apontar pra um cluster de produção)")
	}
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath()
	overrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

func kubeconfigPath() string {
	if path := os.Getenv("KUBECONFIG"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// Revision é uma entrada do histórico de rollout de um Deployment — um
// ReplicaSet, na prática, já que é isso que o Kubernetes usa pra versionar.
type Revision struct {
	ReplicaSet string
	Revision   string // annotation "deployment.kubernetes.io/revision", posta pelo controller
	Commit     string // CommitAnnotation no pod template, se presente
	CreatedAt  time.Time
}

// RolloutHistory resolve o Deployment dono do recurso (hoje só Pod e
// Deployment são suportados como "kind" de entrada — outros tipos retornam
// tudo vazio, sem erro, já que não há rollout de Deployment pra
// correlacionar) e devolve o nome do Deployment junto com o histórico de
// ReplicaSets, do mais recente pro mais antigo.
func (c *Client) RolloutHistory(ctx context.Context, namespace, kind, name string) (string, []Revision, error) {
	deploymentName, err := c.resolveDeploymentName(ctx, namespace, kind, name)
	if err != nil {
		return "", nil, err
	}
	if deploymentName == "" {
		return "", nil, nil
	}

	replicaSets, err := c.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("listar ReplicaSets em %q: %w", namespace, err)
	}

	var revisions []Revision
	for _, rs := range replicaSets.Items {
		if !ownedBy(rs.OwnerReferences, "Deployment", deploymentName) {
			continue
		}
		revisions = append(revisions, Revision{
			ReplicaSet: rs.Name,
			Revision:   rs.Annotations["deployment.kubernetes.io/revision"],
			Commit:     rs.Spec.Template.Annotations[CommitAnnotation],
			CreatedAt:  rs.CreationTimestamp.Time,
		})
	}
	sort.Slice(revisions, func(i, j int) bool {
		return revisions[i].CreatedAt.After(revisions[j].CreatedAt)
	})
	return deploymentName, revisions, nil
}

// RollbackDeployment copia o spec.template do ReplicaSet alvo de volta pro
// Deployment — o mesmo mecanismo que `kubectl rollout undo` usa por baixo
// dos panos, simplificado (sem reescrever o annotation de change-cause).
//
// ÚNICA operação de escrita do agente junto com RestartDeployment — e as
// duas só são alcançáveis pelo subcomando `apply-action`, nunca por
// `analyze`, que é estritamente leitura. A separação existe de propósito:
// sugerir uma ação (Fase 5) é uma coisa, executá-la é outra, com aprovação
// explícita no meio.
func (c *Client) RollbackDeployment(ctx context.Context, namespace, deployment, toReplicaSet string) error {
	rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, toReplicaSet, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("buscar ReplicaSet %q: %w", toReplicaSet, err)
	}
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{"template": rs.Spec.Template},
	})
	if err != nil {
		return fmt.Errorf("montar patch de rollback: %w", err)
	}
	if _, err := c.clientset.AppsV1().Deployments(namespace).Patch(ctx, deployment, types.StrategicMergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("aplicar rollback em %q: %w", deployment, err)
	}
	return nil
}

// RestartDeployment aciona um rollout novo sem mudar a imagem — equivalente
// a `kubectl rollout restart` — via o mesmo annotation que o próprio kubectl
// usa (kubectl.kubernetes.io/restartedAt), pra reconhecimento consistente
// por outras ferramentas que olharem o histórico depois.
func (c *Client) RestartDeployment(ctx context.Context, namespace, deployment string) error {
	patch, err := json.Marshal(map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("montar patch de restart: %w", err)
	}
	if _, err := c.clientset.AppsV1().Deployments(namespace).Patch(ctx, deployment, types.StrategicMergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("reiniciar %q: %w", deployment, err)
	}
	return nil
}

func (c *Client) resolveDeploymentName(ctx context.Context, namespace, kind, name string) (string, error) {
	switch kind {
	case "Deployment":
		return name, nil
	case "Pod":
		pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("buscar pod %q: %w", name, err)
		}
		rsName := ownerName(pod.OwnerReferences, "ReplicaSet")
		if rsName == "" {
			return "", nil
		}
		rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, rsName, metav1.GetOptions{})
		if err != nil {
			// O ReplicaSet do pod pode já ter sido deletado (rollout mais recente
			// substituiu por um novo) — não é erro fatal pra correlação, só
			// significa que não conseguimos subir até o Deployment por essa via.
			return "", nil //nolint:nilerr
		}
		return ownerName(rs.OwnerReferences, "Deployment"), nil
	default:
		return "", nil
	}
}

func ownedBy(refs []metav1.OwnerReference, kind, name string) bool {
	return ownerName(refs, kind) == name
}

func ownerName(refs []metav1.OwnerReference, kind string) string {
	for _, ref := range refs {
		if ref.Kind == kind {
			return ref.Name
		}
	}
	return ""
}
