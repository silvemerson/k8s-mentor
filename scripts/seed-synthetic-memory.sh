#!/usr/bin/env bash
# Popula a memória do k8s-mentor (Qdrant) com o histórico sintético de
# incidentes (demo/synthetic-data/incidents.json), com timestamps no passado
# — pra "isso já aconteceu 5 vezes este mês" funcionar na demo sem depender
# de rodar `analyze` repetidas vezes ao vivo nem esperar recorrências reais.
#
# Cada incidente vira um Pod efêmero (--rm) rodando a própria imagem do
# agente com o comando trocado pra `analyze --timestamp <data>`, dentro do
# namespace do k8s-mentor (mesma rede que Qdrant/Ollama). Idempotente: os IDs
# no Qdrant são determinísticos (hash dos campos do evento), então rodar de
# novo com os mesmos dados só sobrescreve os mesmos pontos, não duplica.
#
# Pré-requisitos: jq, kubectl (contexto apontando pro cluster certo), imagem
# k8s-mentor/agent:dev já carregada no Kind (kind load docker-image ...).
#
# Uso: ./scripts/seed-synthetic-memory.sh
#   NAMESPACE=k8s-mentor FILE=demo/synthetic-data/incidents.json ./scripts/seed-synthetic-memory.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }

NAMESPACE="${NAMESPACE:-k8s-mentor}"
FILE="${FILE:-demo/synthetic-data/incidents.json}"
IMAGE="${IMAGE:-k8s-mentor/agent:dev}"

if ! command -v jq >/dev/null 2>&1; then
  echo "❌ jq não encontrado. Instale jq pra rodar este script." >&2
  exit 1
fi

count="$(jq 'length' "${FILE}")"
echo "Semeando ${count} incidentes sintéticos de ${FILE} em '${NAMESPACE}'..."

for ((i = 0; i < count; i++)); do
  ts="$(jq -r ".[${i}].timestamp" "${FILE}")"
  payload="$(jq -c "[.[${i}].k8sgpt]" "${FILE}")"
  name="$(jq -r ".[${i}].k8sgpt.name" "${FILE}")"
  echo "  [$((i + 1))/${count}] ${ts} — ${name}"
  echo "${payload}" | kubectl run "seed-incident-${i}" -n "${NAMESPACE}" --rm -i --restart=Never \
    --image="${IMAGE}" --image-pull-policy=Never \
    --command -- /k8s-mentor analyze \
    --ollama-url http://ollama:11434 --qdrant-url http://qdrant:6333 --timestamp "${ts}" \
    >/dev/null
done

echo
echo "✅ ${count} incidentes sintéticos gravados na memória do k8s-mentor."
