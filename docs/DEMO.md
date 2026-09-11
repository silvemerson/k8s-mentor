# Roteiro e Checklist da Demo — k8s-mentor

> Roteiro de demo para a palestra no TDC São Paulo. Os 5 pilares e os 4 cenários abaixo estão implementados e validados contra um cluster Kind real — ver [Roadmap no README](../README.md#roadmap). A talk usa prints capturados manualmente (não demo ao vivo) — a seção [Comandos pra capturar prints](#comandos-pra-capturar-prints) tem o passo a passo exato.

## Sumário

- [Narrativa da demo](#narrativa-da-demo)
- [Cenários de falha a injetar](#cenários-de-falha-a-injetar)
- [Comandos pra capturar prints](#comandos-pra-capturar-prints)
- [Checklist de setup pré-palestra](#checklist-de-setup-pré-palestra)
- [Checklist no dia (poucas horas antes)](#checklist-no-dia-poucas-horas-antes)
- [Plano de fallback](#plano-de-fallback)

## Narrativa da demo

1. Cluster **Kind** sobe com múltiplos problemas propositalmente injetados.
2. **K8sGPT roda sozinho primeiro** — resolve/explica bem os casos "fáceis" (~70% do total), mostrando que a ferramenta é legitimamente boa no que se propõe a fazer.
3. Um caso mais sutil ou recorrente aparece — K8sGPT erra, fica raso, ou simplesmente não reconhece o recurso (CRD customizado).
4. **k8s-mentor entra em cena**: mesmo caso, agora passando pela camada de memória + RAG + correlação. O diagnóstico muda de "o pod tomou OOMKilled" para "isso já aconteceu 5 vezes este mês, o padrão sugere memory leak, aqui está o runbook e o deploy que provavelmente causou".
5. Fecha reforçando o papel do SRE no loop: o k8s-mentor sugere, não decide — a ação de remediação só acontece com aprovação explícita no palco.

## Cenários de falha a injetar

Cada cenário é um manifest (ou script) versionado em [`demo/scenarios/`](../demo/scenarios/), aplicável isoladamente e revertível.

| # | Cenário | O que o K8sGPT faz | O que o k8s-mentor adiciona | Status |
|---|---|---|---|---|
| 1 | **OOMKilled "fácil"** — limite de memória obviamente subdimensionado ([`oomkilled-facil/`](../demo/scenarios/oomkilled-facil/), `polinux/stress` com `limits.memory: 16Mi`) | Explica corretamente: "aumente o limite de memória" | (não é o foco — serve para mostrar o K8sGPT acertando) | ✅ implementado e testado contra cluster real |
| 2 | **OOMKilled recorrente/sutil** — limite plausível (64Mi), mas app com crescimento gradual de memória simulado ([`oomkilled-recorrente/`](../demo/scenarios/oomkilled-recorrente/), script Python de ~1MB/s) que estoura o limite ciclicamente (~60s pra primeiro OOMKilled) | Explica o evento isolado, sem contexto de recorrência | Detecta a recorrência via memória (Fase 1 ✅, `recurrence: 5` testado), puxa o runbook certo via RAG (Fase 2 ✅, [`checkout-service-memory-leak.md`](../demo/runbooks/checkout-service-memory-leak.md), scores 0.75–0.80) **e** confirma a tendência de crescimento via Prometheus (Fase 4 ✅, `analyze --with-metrics`: `growing_trend: true`, 15MB→27MB em ~6.5min) — os 3 pilares juntos nesse cenário são o momento em que o diagnóstico se aprofunda de verdade (passo 4 da narrativa) | ✅ cenário pronto · memória + RAG + métricas validados |
| 3 | **CRD customizado não reconhecido** — CRD fictício `DemoWorkload` ([`crd-nao-reconhecido/`](../demo/scenarios/crd-nao-reconhecido/)) em `status.phase: Failed`, fora dos analyzers padrão do K8sGPT | Não reconhece o recurso ou dá explicação genérica/rasa | RAG sobre runbook interno específico do CRD (Fase 2 ✅, [`demoworkload-crd.md`](../demo/runbooks/demoworkload-crd.md), scores 0.78) | ✅ cenário pronto · pilar de RAG validado |
| 4 | **Erro correlacionado com deploy recente** — v1 ok → v2 quebrada (typo em env var) ([`deploy-recente-quebrado/`](../demo/scenarios/deploy-recente-quebrado/)), com intervalo de tempo real entre as duas via `apply.sh` | Explica o crash isolado, sem saber que houve deploy minutos antes | Correlação GitOps (Fase 3 ✅) identifica o ReplicaSet/revisão/commit exatos que quebraram, com `suspect_recent_deploy: true` **e** ação supervisionada (Fase 5 ✅) sugere rollback pra revisão anterior — `apply-action --yes` executa de verdade e o pod volta a `1/1 Running`. **Este é o cenário-clímax da demo: mostra os 5 pilares na mesma falha, terminando com o humano aprovando a correção** | ✅ cenário pronto · todos os pilares validados de ponta a ponta |
| 5 | *(buffer/opcional)* **Falso positivo controlado** — um caso onde o histórico de memória existe mas a causa raiz é diferente da vez anterior | K8sGPT trata como igual às vezes anteriores | k8s-mentor identifica que o padrão de métricas é diferente desta vez e evita concluir por analogia simples — reforça a mensagem de "sugestão, não certeza" | ☐ opcional, se sobrar tempo de preparo |

Orquestração: [`demo/scenarios/apply-all.sh`](../demo/scenarios/apply-all.sh) monta o estado completo (idempotente); [`demo/scenarios/revert-all.sh`](../demo/scenarios/revert-all.sh) desfaz. Os cenários 3 e 4 têm `apply.sh` próprio (esperam a CRD ficar `Established` / criam um intervalo de tempo real entre v1 e v2, respectivamente) — `apply-all.sh` já chama os dois na ordem certa.

**Histórico sintético:** [`demo/synthetic-data/incidents.json`](../demo/synthetic-data/incidents.json) tem 5 ocorrências passadas do `checkout-service` (namespace `demo`, mesmo nome do cenário 2) espalhadas ao longo de agosto. [`scripts/seed-synthetic-memory.sh`](../scripts/seed-synthetic-memory.sh) grava tudo no Qdrant com os timestamps corretos (flag `--timestamp` do `k8s-mentor analyze`). Testado de ponta a ponta: rodar `analyze --with-history` contra a falha real do cenário 2 retorna `recurrence: 5`, com os 5 incidentes sintéticos ranqueados por similaridade semântica (scores 0.94–0.98), não por match exato de texto.

Antes de um ensaio final (pra recorrência bater certinho com os dados sintéticos, sem contaminação de testes anteriores): `./scripts/reset-memory.sh && ./scripts/seed-synthetic-memory.sh`.

**Runbooks (RAG):** [`demo/runbooks/`](../demo/runbooks/) tem dois runbooks reais em Markdown — um sobre o memory leak do `checkout-service` (cenário 2), outro sobre o `DemoWorkload` (cenário 3). Indexe antes da demo com:

```bash
kubectl port-forward -n k8s-mentor svc/qdrant 6333:6333 &
go run ./cmd/k8s-mentor ingest-runbooks --dir demo/runbooks \
  --ollama-url http://localhost:11434 --qdrant-url http://localhost:6333
```

Testado de ponta a ponta com `analyze --with-runbooks`: cada cenário recupera **só** os trechos do runbook correto (nenhuma contaminação cruzada entre os dois), por similaridade semântica — não por palavra-chave.

## Comandos pra capturar prints

Validado do zero contra o cluster real. Roda tudo direto do host via `go run`
(mais simples que empacotar num Pod pra um teste pontual) — precisa de
`kubectl port-forward` pro Qdrant e o Prometheus, já que eles só existem
dentro do cluster.

### 0. Antes de tudo

```bash
kubectl config use-context kind-k8s-mentor-demo   # NUNCA rode nada sem checar o contexto antes
kubectl get pods -n k8s-mentor -n demo             # confirma que Qdrant/agente/cenários estão de pé
```

Se algo estiver `CrashLoopBackOff`/`Unknown` fora do esperado (os cenários
1, 2 e 4 devem estar quebrados — é de propósito), rode
`./scripts/setup-cluster.sh` (idempotente) antes de seguir.

Em dois terminais separados, deixe rodando (a demo inteira depende disso):

```bash
kubectl port-forward -n k8s-mentor svc/qdrant 6333:6333
kubectl port-forward -n monitoring svc/prometheus-server 9090:80
```

Se a memória/RAG estiver vazia (Qdrant recém-criado), semeia antes:

```bash
./scripts/seed-synthetic-memory.sh
go run ./cmd/k8s-mentor ingest-runbooks --dir demo/runbooks \
  --ollama-url http://localhost:11434 --qdrant-url http://localhost:6333
```

### Momento 1 — K8sGPT sozinho

```bash
k8sgpt analyze --namespace demo --backend ollama --explain -o json \
  --kubecontext kind-k8s-mentor-demo | python3 -m json.tool
```

O cenário 2 (`checkout-service`) crasha e recupera em ciclos de ~60s — se
não aparecer na primeira tentativa, espere uns segundos e rode de novo. O
cenário 1 (`image-resizer`) fica em `CrashLoopBackOff` de forma estável,
mais fácil de capturar sem pressa de timing.

### Momento 2 — k8s-mentor com memória + RAG + métricas

```bash
k8sgpt analyze --namespace demo --backend ollama --explain -o json \
  --kubecontext kind-k8s-mentor-demo \
  | go run ./cmd/k8s-mentor analyze --with-history --with-runbooks --with-metrics \
      --qdrant-url http://localhost:6333 \
      --prometheus-url http://localhost:9090 \
      --ollama-url http://localhost:11434 \
  | python3 -m json.tool
```

`growing_trend` só fica `true` se a captura pegar o pod no meio de um ciclo
de crescimento — se cair `false` com poucas amostras, espere ~30s (o pod
recém-reiniciado ainda crescendo) e rode de novo.

### Momento 3 — Ação supervisionada (sugerir → aprovar → confirmar)

Recria o cenário 4 fresco (garante `suspect_recent_deploy: true`, que exige
um deploy recente — se o `payment-worker` já estiver quebrado há mais de
30min, a suspeita não dispara):

```bash
kubectl delete deployment payment-worker -n demo --ignore-not-found
./demo/scenarios/deploy-recente-quebrado/apply.sh
```

Espere o pod entrar em `CrashLoopBackOff` (`kubectl get pods -n demo -l app=payment-worker -w`), depois:

```bash
POD=$(kubectl get pods -n demo -l app=payment-worker --no-headers | awk '$2=="0/1"{print $1; exit}')

k8sgpt analyze --namespace demo --backend ollama --explain -o json --kubecontext kind-k8s-mentor-demo \
  | jq --arg pod "$POD" '.results |= map(select(.name == ("demo/" + $pod)))' \
  > /tmp/pw.json

K8S_MENTOR_KUBE_CONTEXT=kind-k8s-mentor-demo go run ./cmd/k8s-mentor analyze \
  --with-gitops --with-action --qdrant-url http://localhost:6333 --ollama-url http://localhost:11434 \
  < /tmp/pw.json | tee /tmp/pw-analyzed.json | python3 -m json.tool
# print aqui: mostra gitops.suspect_recent_deploy=true e action.type=rollback

jq '.incidents[0].action' /tmp/pw-analyzed.json > /tmp/suggestion.json

K8S_MENTOR_KUBE_CONTEXT=kind-k8s-mentor-demo go run ./cmd/k8s-mentor apply-action --input /tmp/suggestion.json
# print aqui: "Nada foi executado. Rode de novo com --yes pra aplicar."

K8S_MENTOR_KUBE_CONTEXT=kind-k8s-mentor-demo go run ./cmd/k8s-mentor apply-action --input /tmp/suggestion.json --yes
# print aqui: "✅ Ação aplicada."

kubectl get pods -n demo -l app=payment-worker
kubectl logs -n demo -l app=payment-worker --tail=3
# print aqui: pod 1/1 Running, log "payment-worker conectado a postgres://..."
```

`K8S_MENTOR_KUBE_CONTEXT` é obrigatório rodando fora do cluster (trava de
segurança proposital, ver `internal/k8s`) — sem isso, `--with-gitops` e
`apply-action` recusam rodar em vez de confiar no contexto ambiente.

## Checklist de setup pré-palestra

Preparação a ser feita com antecedência (dias antes, não no dia):

- [ ] **Kind, Docker, Helm e kubectl instalados e testados** na máquina de demo (`kind version`, `helm version`), imagem do node do Kind já puxada localmente (evita depender de internet no palco).
- [ ] **`./scripts/setup-cluster.sh` sobe tudo do zero em poucos minutos**, testado sem cache pelo menos uma vez: rede Docker fixa → Ollama + NFS server (docker-compose) → cluster Kind de 3 nodes → MetalLB → NFS provisioner → Gateway API/NGINX Gateway Fabric → chart Helm do k8s-mentor.
- [ ] **`./scripts/teardown-cluster.sh` também testado**, pra garantir que dá pra resetar o ambiente rápido entre ensaios (ou se algo travar no meio da demo ao vivo).
- [ ] **K8sGPT instalado**, configurado com `--backend ollama` (`k8sgpt auth list` deve mostrar só `ollama` em Active — sem provider de nuvem configurado, o que quebraria o pitch "100% local" se fosse usado sem querer) e testado contra o cluster de demo. `k8sgpt auth update --backend ollama --baseurl http://localhost:11434 --model <modelo>` corrige se precisar.
- [ ] **Ollama já com os modelos baixados** (`ollama pull <modelo-gerador>` e `ollama pull nomic-embed-text` para embeddings — **não pode depender de download no palco**; `setup-cluster.sh` só sobe o servidor, não baixa modelos).
- [ ] **Modelo validado na máquina de demo real** — não só na máquina de desenvolvimento. Testar `llama3.1:8b` e `qwen2.5:7b` (ou equivalentes) e escolher o que roda com latência aceitável dado a RAM disponível no laptop do dia.
- [ ] **Pool de IPs do MetalLB conferido** ([`deploy/kind/loadbalancer/metallb-config.yaml`](../deploy/kind/loadbalancer/metallb-config.yaml)) — se a máquina de demo já roda outro cluster Kind (ex: um homelab pessoal) na mesma rede Docker fixa, confirme que as faixas de IP não colidem.
- [ ] **Contexto do `kubectl` E do `helm` conferidos antes de qualquer script** (`kubectl config current-context`) — se a máquina de demo tiver outros clusters configurados (produção, homolog de cliente...), um contexto errado é perigoso. Os dois têm resolução de contexto **independente** — travar um não trava o outro. `scripts/setup-cluster.sh` já sobrescreve as duas ferramentas (`kubectl` e `helm`) pra sempre mirar `KUBE_CONTEXT` explícito, nunca o corrente; isso só foi descoberto depois de um `helm upgrade --install` quase rodar contra um cluster GKE de produção real porque só o `kubectl` estava travado. Se for rodar `helm`/`kubectl` manualmente fora dos scripts, sempre passe `--kube-context`/`--context` explícito.
- [ ] **IPs fixos do Ollama/NFS conferidos após qualquer pausa longa** (`./scripts/setup-cluster.sh` já faz isso sozinho, idempotente) — se a máquina ficar dias sem rodar nada, os containers Docker podem voltar com outro IP na rede `kind`, e aí o `storageClass nfs-client` e o Service `ollama` do chart apontam pro lugar errado até alguém rodar o setup de novo.
- [ ] **Prometheus escaneando de verdade antes do cenário 2 ao vivo** — a correlação de métricas (Fase 4) precisa de amostras reais na janela anterior ao erro; como o ciclo de OOMKilled do `checkout-service` dura só ~60s, o Prometheus do cluster de demo já sobe com `scrape_interval: 5s` (não o padrão de 1m) — se algum dia trocar essa infra, não esqueça desse ajuste, ou a tendência de crescimento não vai ter amostra suficiente pra detectar.
- [ ] **Rollback do cenário 4 ensaiado ao vivo pelo menos uma vez** — `apply-action --yes` é a única ação de escrita do agente; ensaie o fluxo completo (`analyze --with-gitops --with-action` → conferir a sugestão → `apply-action` sem `--yes` → `apply-action --yes`) antes do palco pra não hesitar na hora. Se o deploy quebrado do cenário já tiver mais de 30min (`gap_seconds` > 1800), `suspect_recent_deploy` vira `false` e a sugestão de rollback some — reaplique `demo/scenarios/deploy-recente-quebrado/apply.sh` pouco antes de mostrar esse trecho ao vivo.
- [ ] **Qdrant sobe com storage NFS funcionando** — validar com o smoke test opcional em [`deploy/kind/nfs/pvc-nfs-test.yaml`](../deploy/kind/nfs/pvc-nfs-test.yaml) antes de confiar nele pra memória de incidentes.
- [ ] **Dados sintéticos de "memória" pré-carregados** ([`demo/synthetic-data/`](../demo/synthetic-data/)) — histórico de incidentes fake mas plausível, carregado via [`scripts/seed-synthetic-memory.sh`](../scripts/), para não depender de gerar histórico real ao vivo (impossível em cluster recém-criado) nem de qualquer chamada de rede.
- [ ] **Runbooks de exemplo prontos** ([`demo/runbooks/`](../demo/runbooks/)) cobrindo especificamente os cenários 2, 3 e 4 acima, indexados no RAG previamente.
- [ ] **Os 4 cenários de falha testados individualmente e em conjunto**, do zero, pelo menos 2x, idealmente por outra pessoa além do apresentador (para pegar passos implícitos/"óbvios só pra quem já fez").
- [ ] **Tempo total do fluxo cronometrado** (setup do cluster → K8sGPT → k8s-mentor → aprovação de ação) e ajustado ao tempo de slot da palestra.
- [ ] **Gravação em vídeo da demo completa rodando com sucesso**, como fallback (ver [Plano de fallback](#plano-de-fallback)).

## Checklist no dia (poucas horas antes)

- [ ] Rodar o fluxo completo do zero na máquina que será usada no palco (não em outra).
- [ ] Confirmar que **não há dependência de Wi-Fi do evento** em nenhum passo — Ollama local, Kind local, imagens já em cache, dados sintéticos já carregados localmente.
- [ ] Fechar aplicações que competem por RAM (o LLM local + Kind + Docker já usam bastante).
- [ ] Deixar o terminal com fonte grande, tema de alto contraste, e histórico de comandos limpo (usar um script/snippet manager em vez de digitar ao vivo, para evitar erro de digitação no palco).
- [ ] Ter o vídeo de fallback aberto e pronto para tocar em outra aba/janela, sem precisar buscar o arquivo na hora.

## Plano de fallback

Wi-Fi de evento é sempre um risco — mesmo com toda a stack rodando localmente, imprevistos de hardware/tempo acontecem. Mitigação:

1. **Gravar a demo completa com sucesso** com antecedência (screen recording, roteiro idêntico ao que será apresentado ao vivo) e ter esse vídeo disponível offline, pronto para tocar sem depender de upload/streaming.
2. Preferir sempre **rodar ao vivo** — o vídeo é fallback, não substituto — mas ter um ponto de corte decidido antecipadamente ("se o cluster não subir em X minutos, seguimos com o vídeo") para não perder o timing da palestra.
3. Ter os **outputs de texto** (JSON do K8sGPT, resposta do k8s-mentor) de uma execução bem-sucedida salvos como arquivo, para poder mostrar/ler mesmo se absolutamente nada mais funcionar ao vivo.
