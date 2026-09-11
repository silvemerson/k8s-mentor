# Manifests vendorizados

Cópias locais de manifests de terceiros usados por [`scripts/setup-cluster.sh`](../../../scripts/setup-cluster.sh), pra não depender de rede externa (GitHub raw/releases) durante o setup — inclusive numa demo ao vivo.

| Arquivo | Origem | Versão |
|---|---|---|
| `metallb-native.yaml` | https://github.com/metallb/metallb/blob/v0.14.9/config/manifests/metallb-native.yaml | v0.14.9 |
| `gateway-api-standard-install.yaml` | https://github.com/kubernetes-sigs/gateway-api/releases/tag/v1.6.1 (standard channel) | v1.6.1 |

## Como atualizar

```bash
curl -sL "https://raw.githubusercontent.com/metallb/metallb/vX.Y.Z/config/manifests/metallb-native.yaml" \
  -o deploy/kind/vendor/metallb-native.yaml

curl -sL "https://github.com/kubernetes-sigs/gateway-api/releases/download/vX.Y.Z/standard-install.yaml" \
  -o deploy/kind/vendor/gateway-api-standard-install.yaml
```

Atualize a tabela acima com a nova versão depois de trocar os arquivos.
