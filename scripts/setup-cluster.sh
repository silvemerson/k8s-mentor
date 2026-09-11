#!/usr/bin/env bash
# Sobe toda a infra do k8s-mentor, na ordem certa:
#   0. Rede Docker fixa "kind" (subnet previsível, compartilhável com outros
#      clusters Kind da mesma máquina — padrão de
#      https://github.com/silvemerson/kind-infra-lab)
#   1. Ollama + NFS server via docker-compose, anexados a essa rede
#   2. Cluster Kind de 3 nodes
#   3. MetalLB (LoadBalancer real dentro do Kind)
#   4. NFS client provisioner (storageClass "nfs-client", usado pelo Qdrant)
#   5. Gateway API (CRDs + NGINX Gateway Fabric + Gateway do k8s-mentor)
#   6. Prometheus mínimo (correlação de métricas — Fase 4)
#   7. Chart Helm do k8s-mentor (Qdrant + agente, se agent.enabled=true)
#
# Manifests do MetalLB e da Gateway API (CRDs) são lidos de
# deploy/kind/vendor/ (vendorizados no repo), não baixados ao vivo — depois
# de bater um 429/503 do raw.githubusercontent.com no meio de um teste real,
# ficou claro que essa é exatamente a dependência de rede que o projeto
# promete eliminar. NGINX Gateway Fabric ainda vem via `helm ... oci://`
# (ghcr.io) — sem vendorizar por enquanto; garanta rede estável nesse passo
# ou baixe/cacheie a imagem com antecedência antes de uma demo ao vivo.
#
# Idempotente: pode rodar de novo, pula/atualiza o que já existe.
#
# Pré-requisito: modelos já baixados no Ollama que for usado (ver docs/DEMO.md)
# — este script NÃO baixa modelo de geração, só garante o de embedding no
# container Ollama (próprio ou reaproveitado — ver OLLAMA_CONTAINER abaixo).
#
# Variáveis opcionais:
#   SUBNET           (default 172.20.0.0/16)      — subnet da rede Docker fixa "kind"
#   GATEWAY          (default 172.20.0.1)         — gateway dessa rede
#   OLLAMA_CONTAINER (default k8s-mentor-ollama)  — nome de um container Ollama
#     JÁ EXISTENTE na máquina pra reaproveitar (ex: outro projeto seu já usando
#     a porta 11434). Se for diferente do default, este script NÃO sobe o
#     serviço "ollama" do docker-compose — só conecta esse container na rede
#     "kind" (se ainda não estiver) e usa o IP dele.
#   EMBEDDING_MODEL  (default nomic-embed-text)   — modelo de embedding a garantir
#   NFS_IP / OLLAMA_IP (default 172.20.0.100 / .101) — IPs FIXOS pra esses
#     dois containers na rede "kind". Sem isso, o Docker pode realocar os
#     IPs quando os containers reiniciam (ex: depois da máquina ficar dias
#     parada) — e aí o storageClass nfs-client e o Service "ollama" do chart
#     apontam pro IP errado, silenciosamente, até alguém notar um
#     MountVolume.SetUp/Connection refused. Fixar resolve isso de vez.
#     Faixa .100/.101 escolhida de propósito longe de dois lugares que
#     também disputam IP baixo nessa rede compartilhada: os nodes de outros
#     clusters Kind (kind cria containers de node com IP dinâmico, tipo
#     .2-.10) e os pools do MetalLB (.220+). Já bati de frente com o
#     "sparta" de outro projeto pegando .5/.6 antes da gente — se isso
#     acontecer nessa faixa também, mude NFS_IP/OLLAMA_IP.
#
# Uso: ./scripts/setup-cluster.sh
#   OLLAMA_CONTAINER=slm-ollama ./scripts/setup-cluster.sh   # reaproveitando outro container
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Trava de segurança: NUNCA depende do contexto corrente — se o host tiver
# outros clusters configurados (produção, homolog...), um contexto errado
# ali é como um comando acaba rodando no cluster errado sem avisar. Sempre
# mira explicitamente no Kind local deste projeto — em AMBOS `kubectl` E
# `helm`, que têm resolução de contexto INDEPENDENTE uma da outra (achado
# do jeito ruim: o `kubectl` daqui já estava travado há sessões, mas os
# `helm upgrade --install` deste script sempre confiaram no current-context
# ambiente, sem ninguém perceber, até um deles tentar instalar num cluster
# GKE de produção real porque o contexto corrente da máquina tinha mudado).
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-k8s-mentor-demo}"
kubectl() { command kubectl --context "${KUBE_CONTEXT}" "$@"; }
helm() { command helm --kube-context "${KUBE_CONTEXT}" "$@"; }

SUBNET="${SUBNET:-172.20.0.0/16}"
GATEWAY="${GATEWAY:-172.20.0.1}"
NETWORK_NAME="kind"
CLUSTER_NAME="k8s-mentor-demo"
NFS_CONTAINER="k8s-mentor-nfs-server"
OLLAMA_CONTAINER="${OLLAMA_CONTAINER:-k8s-mentor-ollama}"
EMBEDDING_MODEL="${EMBEDDING_MODEL:-nomic-embed-text}"
NFS_IP="${NFS_IP:-172.20.0.100}"
OLLAMA_IP="${OLLAMA_IP:-172.20.0.101}"

# Garante que $1 esteja na rede "kind" com o IP $2 — desconecta e reconecta
# se estiver com outro IP (ou desconectado). Idempotente: não faz nada se já
# estiver correto.
pin_container_ip() {
  local container="$1" ip="$2" current
  current="$(docker inspect "${container}" --format "{{(index .NetworkSettings.Networks \"${NETWORK_NAME}\").IPAddress}}" 2>/dev/null || true)"
  if [ "${current}" = "${ip}" ]; then
    return 0
  fi
  echo "  fixando IP de '${container}' em ${ip} (estava '${current:-<não conectado>}')"
  docker network disconnect "${NETWORK_NAME}" "${container}" 2>/dev/null || true
  docker network connect --ip "${ip}" "${NETWORK_NAME}" "${container}"
}

echo "== 0. Rede Docker fixa =="
if docker network ls --format '{{.Name}}' | grep -qx "${NETWORK_NAME}"; then
  echo "rede '${NETWORK_NAME}' já existe, pulando (pode ser compartilhada com outros clusters Kind seus)."
else
  docker network create --subnet="${SUBNET}" --gateway="${GATEWAY}" "${NETWORK_NAME}"
fi

echo "== 1. Ollama =="
if [ "${OLLAMA_CONTAINER}" = "k8s-mentor-ollama" ]; then
  echo "subindo container próprio do k8s-mentor (docker-compose)"
  docker compose -f deploy/docker-compose/docker-compose.yaml up -d ollama
else
  echo "reaproveitando container existente '${OLLAMA_CONTAINER}'"
  if ! docker inspect "${OLLAMA_CONTAINER}" >/dev/null 2>&1; then
    echo "❌ container '${OLLAMA_CONTAINER}' não existe. Ajuste OLLAMA_CONTAINER ou suba-o antes." >&2
    exit 1
  fi
fi
pin_container_ip "${OLLAMA_CONTAINER}" "${OLLAMA_IP}"

echo "== 1b. Modelo de embedding (${EMBEDDING_MODEL}) =="
if docker exec "${OLLAMA_CONTAINER}" ollama list 2>/dev/null | awk '{print $1}' | grep -qx "${EMBEDDING_MODEL}:latest\|${EMBEDDING_MODEL}"; then
  echo "'${EMBEDDING_MODEL}' já presente em '${OLLAMA_CONTAINER}'."
else
  docker exec "${OLLAMA_CONTAINER}" ollama pull "${EMBEDDING_MODEL}"
fi

echo "== 1c. NFS server (docker-compose, fora do cluster) =="
docker compose -f deploy/docker-compose/docker-compose.yaml up -d nfs-server
pin_container_ip "${NFS_CONTAINER}" "${NFS_IP}"
echo "nfs-server em ${NFS_IP} / ollama (${OLLAMA_CONTAINER}) em ${OLLAMA_IP}"

echo "== 2. Cluster Kind '${CLUSTER_NAME}' (3 nodes) =="
if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "cluster '${CLUSTER_NAME}' já existe, pulando."
else
  kind create cluster --name "${CLUSTER_NAME}" --config deploy/kind/cluster.yaml
fi

echo "== 3. MetalLB =="
kubectl apply -f deploy/kind/vendor/metallb-native.yaml
kubectl wait --namespace metallb-system --for=condition=ready pod -l app=metallb --timeout=120s
kubectl wait --namespace metallb-system --for=condition=ready pod -l component=speaker --timeout=90s
kubectl apply -f deploy/kind/loadbalancer/metallb-config.yaml

echo "== 4. NFS client provisioner (storage do Qdrant) =="
helm repo add nfs https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner >/dev/null 2>&1 || true
helm repo update >/dev/null
kubectl create ns nfs-provisioner --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install nfs-provisioner nfs/nfs-subdir-external-provisioner \
  --namespace nfs-provisioner \
  --set nfs.server="${NFS_IP}" \
  --set nfs.path="/" \
  --set storageClass.name=nfs-client \
  --set storageClass.defaultClass=false

echo "== 5. Gateway API (CRDs + NGINX Gateway Fabric) =="
kubectl apply -f deploy/kind/vendor/gateway-api-standard-install.yaml
kubectl create ns nginx-gateway --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install ngf oci://ghcr.io/nginx/charts/nginx-gateway-fabric --namespace nginx-gateway
kubectl wait --timeout=5m -n nginx-gateway deployment/ngf-nginx-gateway-fabric --for=condition=Available
kubectl apply -f deploy/kind/gateway-api/gateway.yaml
kubectl wait --timeout=90s -n default --for=condition=Programmed gateway/k8s-mentor-gateway

echo "== 6. Prometheus (correlação de métricas — Fase 4) =="
# Minimalista de propósito: sem Alertmanager/Grafana/node-exporter/kube-state-metrics
# (não usamos nada disso hoje), sem PVC (retenção curta, dado descartável de
# demo) e scrape_interval de 5s — bem mais frequente que o padrão (1m), pra
# caber várias amostras mesmo num ciclo de crash curto (o cenário
# oomkilled-recorrente estoura em ~60s; com scrape de 1m dava só 0-1 amostra
# por ciclo, insuficiente pra detectar tendência). scrape_timeout também
# fixado em 5s: o chart tem que ser <= scrape_interval, e o default do
# chart mudou pra um valor maior numa atualização — sem fixar os dois
# juntos, um `helm repo update` novo pode voltar a quebrar isso.
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts >/dev/null 2>&1 || true
helm repo update >/dev/null
kubectl create ns monitoring --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install prometheus prometheus-community/prometheus \
  --namespace monitoring \
  --set server.persistentVolume.enabled=false \
  --set server.retention=6h \
  --set server.global.scrape_interval=5s \
  --set server.global.scrape_timeout=5s \
  --set alertmanager.enabled=false \
  --set prometheus-pushgateway.enabled=false \
  --set prometheus-node-exporter.enabled=false \
  --set kube-state-metrics.enabled=false \
  --set server.resources.requests.cpu=100m \
  --set server.resources.requests.memory=256Mi \
  --set server.resources.limits.cpu=500m \
  --set server.resources.limits.memory=512Mi
kubectl wait --timeout=90s -n monitoring --for=condition=Ready pod -l app.kubernetes.io/name=prometheus

echo "== 7. k8s-mentor (Qdrant + agente) via Helm =="
helm upgrade --install k8s-mentor deploy/helm/k8s-mentor -n k8s-mentor --create-namespace \
  --set ollama.external.host="${OLLAMA_IP}"

echo
echo "✅ Infra no ar."
echo "   Ollama:  http://localhost:11434 (host) / http://${OLLAMA_IP}:11434 (dentro da rede kind)"
echo "   Cluster: '${CLUSTER_NAME}' — kubectl -n k8s-mentor get pods"
echo "   Pra derrubar tudo depois: ./scripts/teardown-cluster.sh"
