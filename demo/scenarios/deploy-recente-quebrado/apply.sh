#!/usr/bin/env bash
# Aplica a v1 (ok), espera ficar Available, e só então aplica a v2 (quebrada)
# — cria um intervalo de tempo real entre "antes" e "depois", em vez de já
# nascer quebrado. Ajuste WAIT_SECONDS se quiser mais tempo de intervalo
# durante a demo ao vivo (padrão: só o suficiente pra v1 estabilizar).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }

WAIT_SECONDS="${WAIT_SECONDS:-20}"

kubectl apply -f v1-ok.yaml
kubectl wait --timeout=60s -n demo --for=condition=Available deployment/payment-worker
echo "payment-worker (v1, ok) rodando — aguardando ${WAIT_SECONDS}s antes da v2..."
sleep "${WAIT_SECONDS}"
kubectl apply -f v2-broken.yaml
echo "payment-worker (v2, quebrada) aplicada — deve entrar em CrashLoopBackOff em instantes."
