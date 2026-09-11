#!/usr/bin/env bash
# Remove tudo que apply-all.sh cria. Idempotente — pode rodar de novo mesmo
# que só parte dos cenários exista. Mantém o namespace "demo" (remova
# manualmente com `kubectl delete ns demo` se quiser tudo mesmo).
#
# Uso: ./demo/scenarios/revert-all.sh
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }

kubectl delete -f oomkilled-facil/manifest.yaml --ignore-not-found
kubectl delete -f oomkilled-recorrente/manifest.yaml --ignore-not-found
kubectl delete -f crd-nao-reconhecido/instance.yaml --ignore-not-found
kubectl delete -f crd-nao-reconhecido/crd.yaml --ignore-not-found
kubectl delete -f deploy-recente-quebrado/v2-broken.yaml --ignore-not-found
kubectl delete -f deploy-recente-quebrado/v1-ok.yaml --ignore-not-found

echo
echo "Cenários removidos. Namespace 'demo' mantido — kubectl delete ns demo se quiser removê-lo também."
