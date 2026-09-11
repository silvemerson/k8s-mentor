#!/usr/bin/env bash
# Aplica a CRD, espera ficar Established, só então cria a instância —
# evita a corrida de tentar criar um DemoWorkload antes da API do cluster
# reconhecer o tipo (kubectl reclamaria de "no matches for kind").
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Trava de segurança: NUNCA depende do contexto kubectl corrente — ver nota em
# scripts/setup-cluster.sh.
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }

kubectl apply -f crd.yaml
kubectl wait --timeout=30s --for=condition=Established crd/demoworkloads.k8smentor.dev
kubectl apply -f instance.yaml
