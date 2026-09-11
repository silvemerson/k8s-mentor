#!/usr/bin/env bash
# Derruba o que scripts/setup-cluster.sh cria: o cluster Kind do k8s-mentor e
# os containers docker-compose (Ollama + NFS server).
#
# NÃO remove a rede Docker fixa "kind" — ela pode estar sendo usada por
# outros clusters Kind seus na mesma máquina (ex: kind-infra-lab). Se quiser
# removê-la mesmo assim, faça manualmente: docker network rm kind
#
# Se você reaproveitou um container Ollama externo (OLLAMA_CONTAINER=... no
# setup), ele NÃO é afetado por este `docker compose down` — só desconecta
# manualmente da rede "kind" se quiser: docker network disconnect kind <container>
#
# Idempotente — pode rodar de novo mesmo que só parte da infra exista.
#
# Uso: ./scripts/teardown-cluster.sh
set -uo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

CLUSTER_NAME="k8s-mentor-demo"

echo "== Derrubando cluster Kind '${CLUSTER_NAME}' =="
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  kind delete cluster --name "${CLUSTER_NAME}"
else
  echo "cluster '${CLUSTER_NAME}' não existe, pulando."
fi

echo "== Derrubando Ollama + NFS server (docker-compose) =="
docker compose -f deploy/docker-compose/docker-compose.yaml down

echo
echo "Infra limpa. A pasta deploy/docker-compose/nfs-data (dados do NFS) foi mantida de propósito."
echo "A rede Docker 'kind' também foi mantida — remova manualmente se necessário: docker network rm kind"
