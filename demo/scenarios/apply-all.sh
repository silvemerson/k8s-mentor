#!/usr/bin/env bash
# Monta o estado completo da demo: os 4 cenários de falha injetados no
# namespace "demo". Idempotente — pode rodar de novo sem duplicar nada.
#
# Uso: ./demo/scenarios/apply-all.sh
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }
export KUBE_CONTEXT # os apply.sh de cenários chamados abaixo herdam a mesma trava

kubectl apply -f namespace.yaml

echo "== Cenário 1: OOMKilled fácil =="
kubectl apply -f oomkilled-facil/manifest.yaml

echo "== Cenário 2: OOMKilled recorrente/sutil =="
kubectl apply -f oomkilled-recorrente/manifest.yaml

echo "== Cenário 3: CRD não reconhecido =="
./crd-nao-reconhecido/apply.sh

echo "== Cenário 4: erro correlacionado com deploy recente =="
./deploy-recente-quebrado/apply.sh

echo
echo "✅ Cenários aplicados. Acompanhe com: kubectl get pods -n demo -w"
