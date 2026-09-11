# Runbook: DemoWorkload em status Failed (ImageRegistryUnreachable)

## Sintomas

Um recurso `DemoWorkload` (CRD interno, grupo `k8smentor.dev`) aparece com
`status.phase: Failed` e `status.reason` contendo
`ImageRegistryUnreachable`. Ferramentas de diagnóstico genéricas como o
K8sGPT não têm um analyzer pra esse CRD — ele não é um tipo nativo do
Kubernetes, então o erro passa despercebido a menos que alguém confira o
recurso manualmente com `kubectl get demoworkloads`.

## Causa raiz

O controller do `DemoWorkload` tenta resolver o hostname do registry interno
(`internal-registry.example.com`) definido em `spec.image` antes de agendar
os pods de verdade. Esse hostname só existe no DNS interno da VPN da empresa
— se o node do cluster (ou, no caso de um cluster de laptop/homelab, a
própria máquina) não estiver na VPN, a resolução de DNS falha e o controller
marca o recurso como `Failed` com esse motivo específico.

## Como confirmar que é esse problema

- `status.reason` menciona explicitamente `ImageRegistryUnreachable` e o
  hostname do registry.
- `kubectl exec` num pod qualquer do cluster e rodar
  `nslookup internal-registry.example.com` falha (NXDOMAIN ou timeout).
- Não há erro correspondente em nenhum Pod/Deployment "normal" — porque os
  pods de verdade nunca chegaram a ser criados; o controller falhou antes
  disso.

## Correção

Conectar a VPN da empresa (ou, em ambiente de demo/dev, apontar
`spec.image` pra um registry público acessível, tipicamente
`ghcr.io/<org>/<imagem>`) e recriar o `DemoWorkload` — o controller
reprocessa o recurso automaticamente no próximo reconcile.

## Prevenção

O controller poderia fazer essa checagem de DNS num `webhook` de admissão em
vez de só falhar depois, no reconcile — feedback mais rápido pra quem está
aplicando o manifesto. Ainda não implementado; ticket aberto no board do
time de plataforma.
