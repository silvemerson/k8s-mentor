#!/usr/bin/env bash
# Apaga a collection de memória do k8s-mentor no Qdrant (incidents) — útil
# antes de um ensaio final da demo, pra "já aconteceu N vezes" bater
# exatamente com o histórico sintético (demo/synthetic-data/incidents.json),
# sem contaminação de testes anteriores. A collection é recriada
# automaticamente na próxima análise.
#
# Uso: ./scripts/reset-memory.sh
#   NAMESPACE=k8s-mentor COLLECTION=incidents ./scripts/reset-memory.sh
set -euo pipefail

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }

NAMESPACE="${NAMESPACE:-k8s-mentor}"
COLLECTION="${COLLECTION:-incidents}"

echo "Apagando collection '${COLLECTION}' no Qdrant (namespace ${NAMESPACE})..."
kubectl run reset-memory -n "${NAMESPACE}" --rm -i --restart=Never --image=curlimages/curl --timeout=30s -- \
  curl -sS -X DELETE "http://qdrant:6333/collections/${COLLECTION}"

echo
echo "✅ Memória limpa. Rode ./scripts/seed-synthetic-memory.sh de novo antes da demo."
