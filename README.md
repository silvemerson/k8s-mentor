# k8s-mentor

**A memória local do seu K8sGPT.**

> Ferramentas de diagnóstico do Kubernetes te dizem *o que* quebrou. O k8s-mentor te ajuda a entender *por que quebrou de novo* — e o que fazer a respeito, com você no controle.

[![Build](https://img.shields.io/badge/build-local-lightgrey)](#)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](./LICENSE)
[![Status](https://img.shields.io/badge/status-alpha%20%2F%20Fase%206-brightgreen)](#roadmap)
[![Ollama](https://img.shields.io/badge/LLM-100%25%20local%20(Ollama)-informational)](#stack-t%C3%A9cnica)
[![Talk](https://img.shields.io/badge/TDC-São%20Paulo-red)](#motivação--talk)

> **Status do projeto:** os 5 pilares — memória de incidentes, RAG de runbooks, correlação GitOps, correlação com Prometheus e ação supervisionada — mais a execução periódica automática (Fase 6) estão implementados e testados de ponta a ponta contra um cluster Kind real, incluindo um rollback de verdade aplicado e confirmado, e uma notificação real via `CronJob`. O pipe `k8sgpt analyze -o json | k8s-mentor analyze` (documentado no Quickstart) já foi validado com o K8sGPT de verdade rodando, não só com JSON forjado — isso pegou vários bugs reais de parsing e recalibrou os thresholds de similaridade contra dados reais (ver `internal/k8sgpt` e `agent/memory`). Veja o [Roadmap](#roadmap) para o que ainda falta de polimento (síntese via LLM, `git init`, gravação da demo).

---

## Sumário

- [O Problema](#o-problema)
- [O que é o k8s-mentor](#o-que-é-o-k8s-mentor)
- [Como funciona](#como-funciona)
- [Quickstart](#quickstart)
- [Stack técnica](#stack-técnica)
- [Roadmap](#roadmap)
- [Motivação / Talk](#motivação--talk)
- [Contribuindo](#contribuindo)
- [Licença](#licença)

---

## O Problema

Ferramentas como o [K8sGPT](https://k8sgpt.ai/) fazem um trabalho excelente ao escanear recursos do Kubernetes — Pods, Services, PVCs, HPAs, Ingresses — e traduzir erros crípticos (`CrashLoopBackOff`, `ImagePullBackOff`, `OOMKilled`) em explicações legíveis por humanos. Isso resolve uma fatia enorme dos incidentes do dia a dia.

Mas essa fatia não é 100%. Na prática, um SRE/plantonista esbarra em limitações estruturais desse modelo de análise:

- **Sem memória entre execuções.** Cada análise é *stateless*. O K8sGPT não sabe que esse mesmo `OOMKilled` já aconteceu 4 vezes esse mês, no mesmo namespace, sempre às sextas-feiras.
- **Sem contexto institucional.** Ele não conhece o runbook que o time escreveu depois do último postmortem, nem sabe que "esse erro específico" tem uma causa raiz documentada e uma correção conhecida.
- **Sem correlação temporal.** Ele não cruza o timestamp do erro com o deploy que subiu 12 minutos antes, o `ArgoCD sync` mais recente, ou o `git log` do manifesto que mudou o `limits.memory`.
- **Sem correlação com métricas.** Ele não observa a tendência — memória subindo linearmente nas últimas 6 horas antes do `OOMKilled` — porque isso não está no estado atual do recurso, está no Prometheus.
- **Cobertura limitada a analyzers pré-definidos.** CRDs customizados, operators internos e recursos fora do conjunto de analyzers conhecidos ficam sem diagnóstico, ou recebem um diagnóstico raso.

Ou seja: as ferramentas de diagnóstico atuais resolvem bem o **"o que está quebrado agora"**, mas deixam para o humano todo o trabalho de **"isso já aconteceu antes? por quê? o que mudou? o que fazemos?"** — que é exatamente o trabalho que consome mais tempo (e sono) de quem está de plantão.

## O que é o k8s-mentor

**k8s-mentor** é um agente local — rodando 100% via [Ollama](https://ollama.com/), sem depender de nenhuma API externa — que atua como uma **camada de memória e contexto** complementar a ferramentas de diagnóstico como o K8sGPT.

Ele não substitui o K8sGPT (ou qualquer outra fonte de diagnóstico). Ele **consome** o output dessas ferramentas e o enriquece com:

1. 🧠 **Memória de incidentes** — vetoriza cada análise/erro num vector DB local e reconhece padrões recorrentes.
2. 📚 **RAG sobre runbooks internos** — indexa a documentação e os postmortems do seu time e sugere o runbook certo na hora certa.
3. 🕒 **Correlação temporal com mudanças (GitOps)** — cruza o erro com deploys, syncs do ArgoCD/Flux e commits recentes em manifests.
4. 📈 **Correlação com métricas (Prometheus)** — identifica tendências antes da falha, não só o estado após ela.
5. ✅ **Ação supervisionada (human-in-the-loop)** — pode *sugerir* remediação (rollback, scale, restart), mas nunca age sem aprovação explícita do SRE.

Um ponto de design central: **o k8s-mentor é desacoplado do K8sGPT.** O nome do projeto não amarra a uma ferramenta específica de diagnóstico — hoje a integração de referência é com o K8sGPT, mas a arquitetura prevê consumir logs diretos do cluster, eventos do `kube-apiserver`, alertas do Prometheus, ou qualquer outra fonte de sinal de erro no futuro.

E o ponto que sustenta tudo isso: **roda inteiramente na sua rede local.** Nenhum manifesto, log, secret, métrica ou runbook interno sai do laptop/cluster para uma API de terceiros. Isso não é só uma escolha de custo — é uma escolha deliberada de segurança, dado o tipo de dado sensível que passa por ferramentas de diagnóstico de cluster (ver [Decisões de design](docs/ARCHITECTURE.md#decisões-de-design) para o racional completo).

### Fluxo em alto nível

```mermaid
flowchart LR
    subgraph Cluster["Cluster Kubernetes"]
        R[Recursos: Pods, Services,\nPVCs, HPAs, CRDs...]
    end

    subgraph Diag["Camada de Diagnóstico"]
        K[K8sGPT\n(ou outra fonte: logs,\neventos, alerts)]
    end

    subgraph Mentor["k8s-mentor (100% local)"]
        direction TB
        O[Ollama\nLLM local]
        V[(Vector DB\nmemória de incidentes)]
        RAG[RAG\nrunbooks internos]
        GX[Correlação GitOps\nArgoCD/Flux/git log]
        PX[Correlação métricas\nPrometheus]
        D{Camada de decisão}
        O --- D
        V --- D
        RAG --- D
        GX --- D
        PX --- D
    end

    subgraph Out["Output enriquecido"]
        S[Diagnóstico com causa raiz provável\n+ histórico + runbook + sugestão de ação]
    end

    subgraph Human["SRE"]
        H{Aprova ação?}
    end

    R --> K --> Mentor
    Mentor --> S --> H
    H -- "sim" --> Act[Executa remediação\n(rollback/scale/restart)]
    H -- "não" --> Log[Registra decisão\ne encerra]
    Act --> V
    Log --> V
```

> Diagrama detalhado de componentes, fluxo de dados e decisões de design: veja [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Como funciona

### 1. 🧠 Memória de incidentes

Toda vez que uma análise entra (via K8sGPT ou outra fonte), o k8s-mentor gera um embedding do erro — tipo do recurso, mensagem, namespace, labels relevantes — e grava no vector DB local junto com metadados (timestamp, contexto do cluster, resolução aplicada, se houve). Antes de responder, ele consulta esse histórico por similaridade semântica, não por *string match* exato. Isso permite respostas como:

> "Este `OOMKilled` já ocorreu 5 vezes neste namespace nos últimos 30 dias. Nas últimas 3 ocorrências, o `limits.memory` foi ajustado manualmente como correção paliativa, mas o padrão de crescimento sugere um memory leak na aplicação, não subdimensionamento do limite."

### 2. 📚 RAG sobre runbooks internos

O time indexa seus runbooks, playbooks de incidente e postmortems (Markdown, Confluence exportado, etc.) numa base local. Quando um erro conhecido aparece, o k8s-mentor faz *retrieval* dos trechos relevantes desses documentos e os injeta no contexto do LLM antes de gerar a resposta — trazendo o "como o time já resolveu isso antes" para dentro do diagnóstico, em vez de depender só do conhecimento genérico do modelo.

### 3. 🕒 Correlação temporal com mudanças (GitOps)

O k8s-mentor cruza o timestamp do erro com:
- Histórico de `sync` do ArgoCD/Flux (Application/Kustomization events);
- `git log` dos manifests relevantes no repositório GitOps;
- Metadados de deploy (labels de versão/commit no Pod, quando disponíveis).

O objetivo é responder "o que mudou pouco antes disso quebrar?" automaticamente, em vez do SRE precisar caçar manualmente em três painéis diferentes.

### 4. 📈 Correlação com métricas (Prometheus)

Para erros com componente temporal (memória, CPU, latência, taxa de erro), o k8s-mentor consulta o Prometheus pela janela de tempo antes do evento e identifica tendências — não só o valor no momento da falha. É a diferença entre "o pod tomou OOMKilled com 512Mi de limite" e "a memória subiu de forma linear e previsível nas últimas 6h, batendo o limite às 03:14".

### 5. ✅ Ação supervisionada (human-in-the-loop)

O k8s-mentor pode ter conhecimento suficiente para sugerir uma ação concreta (rollback do último deploy, restart do Deployment), mas **nunca a executa sozinho**. Toda ação sugerida é apresentada com o racional por trás, o comando exato que seria aplicado, e exige aprovação explícita (`apply-action --yes`) antes de qualquer chamada à API do cluster. O objetivo não é automação cega — é reduzir o tempo de diagnóstico mantendo o humano na decisão final. Implementado e testado: `analyze --with-action` sugere, `apply-action` (só com `--yes`) executa — validado com um rollback real que recuperou um Deployment quebrado.

## Quickstart

> Os 5 pilares (memória, RAG, GitOps, Prometheus, ação supervisionada) estão implementados e testados contra um cluster Kind real. Veja `k8s-mentor analyze --help` e o [Roadmap](#roadmap) pra detalhes de cada um.

Arquitetura de execução: **Ollama e o NFS server rodam fora do Kind, via docker-compose**, anexados a uma rede Docker fixa (subnet previsível — padrão adaptado de [kind-infra-lab](https://github.com/silvemerson/kind-infra-lab)); **Qdrant e o agente k8s-mentor rodam dentro de um cluster Kind de 3 nodes como microserviços**, com LoadBalancer via MetalLB, storage persistente via NFS e exposição HTTP opcional via Gateway API.

```bash
# 1. Baixar os modelos locais: um para geração e outro, leve, para embeddings.
ollama pull llama3.1:8b
ollama pull nomic-embed-text

# 2. Subir TUDO: rede Docker fixa, Ollama + NFS server, cluster Kind (3 nodes),
#    MetalLB, NFS provisioner, Gateway API e o chart do k8s-mentor
./scripts/setup-cluster.sh

# 3. Instalar o K8sGPT (fonte de diagnóstico de referência)
brew install k8sgpt   # ou: veja https://docs.k8sgpt.ai/getting-started/installation/
k8sgpt auth add --backend ollama --baseurl http://localhost:11434 --model <modelo-já-baixado>
k8sgpt auth default --provider ollama   # garante que nenhum backend externo seja usado por engano

# 4. (Opcional, para a demo) Injetar os cenários de falha e o histórico sintético
./demo/scenarios/apply-all.sh
./scripts/seed-synthetic-memory.sh

# 5. Enviar a saída JSON ao k8s-mentor. O primeiro incidente é registrado;
#    execuções seguintes informam quantos eventos semanticamente similares já
#    ocorreram no mesmo namespace.
k8sgpt analyze -o json | go run ./cmd/k8s-mentor analyze --namespace demo --with-history

# Pra derrubar tudo no final:
./scripts/teardown-cluster.sh
```

Veja o roteiro completo da demo em [`docs/DEMO.md`](docs/DEMO.md) e o detalhe de cada peça de infraestrutura em [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Stack técnica

| Componente | Escolha | Justificativa |
|---|---|---|
| **LLM local** | [Ollama](https://ollama.com/) + modelo leve (`llama3.1:8b` ou `qwen2.5:7b`, a confirmar conforme testes na máquina de demo), **rodando via docker-compose no host** | Elimina dependência de API externa, roda em laptop comum, modelo trocável sem mudar o resto da stack. Fora do Kind de propósito: mais simples lidar com RAM/GPU do modelo fora de um node que já é Docker-em-Docker. |
| **Vector DB** | [Qdrant](https://qdrant.tech/), **rodando como microserviço dentro do Kind** (Deployment + Service via Helm) | Escrito em Rust, API HTTP/gRPC simples de consumir a partir de Go, binário/imagem leve o suficiente para rodar junto do resto da demo sem competir por RAM com o LLM. Alternativa avaliada: [Chroma](https://www.trychroma.com/) — mais simples de prototipar em Python, mas com ecossistema cliente mais fraco em Go, o que pesa contra dado a escolha de linguagem abaixo. |
| **Linguagem do agente** | **Go** | O ecossistema Kubernetes é nativamente Go (`client-go`, controllers, operators) e o próprio K8sGPT é escrito em Go — integrar como biblioteca ou processo irmão fica natural. Além disso, Go compila para um binário único estático, o que é ideal tanto para distribuição (Homebrew/Krew/imagem de container mínima) quanto para a robustez da demo ao vivo (sem runtime/venv para quebrar no palco). O próprio agente também roda como microserviço dentro do Kind (Deployment + Service), uma vez que a imagem existir (ver Roadmap). |
| **Camada de diagnóstico (fonte)** | [K8sGPT](https://k8sgpt.ai/) (referência inicial) | Ferramenta madura e amplamente adotada; consumida via sua saída estruturada (JSON), mantendo o k8s-mentor desacoplado — outras fontes (logs, eventos do `kube-apiserver`, alertas do Prometheus) entram pela mesma interface no futuro. |
| **Cluster de demo** | [Kind](https://kind.sigs.k8s.io/), 3 nodes (control-plane + 2 workers) | 100% reproduzível localmente, sem custo de cloud, sem depender de internet no dia da palestra — qualquer pessoa da audiência consegue rodar a mesma demo depois. É o "backbone" de microserviços do projeto: Qdrant e o agente rodam nele como Deployments normais. |
| **LoadBalancer no Kind** | [MetalLB](https://metallb.universe.tf/) | Kind não tem LoadBalancer nativo; MetalLB dá IP real via ARP (modo L2) dentro da rede Docker local — necessário pro Gateway API expor o agente sem depender de NodePort/port-forward. |
| **Exposição HTTP** | [Gateway API](https://gateway-api.sigs.k8s.io/) + [NGINX Gateway Fabric](https://docs.nginx.com/nginx-gateway-fabric/) | Substitui o `Ingress` clássico por objetos desacoplados (`Gateway`, `HTTPRoute`); usado (opcionalmente) para expor o agente k8s-mentor via HTTP sem NodePort. |
| **Storage persistente** | NFS (container Docker) + [nfs-subdir-external-provisioner](https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner) | Dá ao Qdrant um `storageClass` de verdade dentro do Kind — a memória de incidentes sobrevive a reagendamento do pod entre os 3 nodes, em vez de sumir com um `emptyDir`. |
| **GitOps (correlação)** | Histórico nativo do Kubernetes (`client-go`, ReplicaSets) hoje; ArgoCD/Flux + `git log` no roadmap | Implementado sem depender de ArgoCD/Flux estarem no cluster — o histórico de rollout do Kubernetes já existe em qualquer cluster e já resolve "o que mudou recentemente". Integração com eventos de sync do ArgoCD/Flux de verdade fica pra quando o cluster de demo tiver um desses instalado. |
| **Métricas (correlação)** | Prometheus (via PromQL), instalado minimalista por `scripts/setup-cluster.sh` | Já é o padrão de observabilidade no ecossistema K8s; qualquer cluster que tenha o K8sGPT plugado provavelmente já tem Prometheus rodando. No cluster de demo, subimos só o server (sem Grafana/Alertmanager/node-exporter) com scrape de 5s. |

> A base de infraestrutura Kind (rede Docker fixa, MetalLB, NFS, Gateway API) é adaptada do homelab [kind-infra-lab](https://github.com/silvemerson/kind-infra-lab) do próprio autor — ver [`deploy/kind/`](deploy/kind/) e [`scripts/setup-cluster.sh`](scripts/setup-cluster.sh).

> A escolha entre Go e Python, assim como entre Qdrant e Chroma, está documentada aqui como **decisão inicial sujeita a revisão** — ver [Roadmap](#roadmap). Se o autor tiver mais domínio/velocidade em Python dado o prazo até o TDC SP, isso deve ser revisitado antes de começar a implementação.

## Roadmap

O projeto está sendo construído em fases incrementais, cada uma agregando um dos 5 pilares descritos em [Como funciona](#como-funciona):

- [x] **Fase 0 — Documentação e arquitetura**
      README, `docs/ARCHITECTURE.md`, `docs/DEMO.md`, estrutura de pastas.
- [x] **Fase 1 — MVP: memória de incidentes**
      Ingestão JSON do K8sGPT → embeddings locais → Qdrant local → detecção de recorrência básica ("já vi isso antes, N vezes"). Veja [`cmd/k8s-mentor`](cmd/k8s-mentor/) e [`agent/memory`](agent/memory/).
- [x] **Fase 2 — RAG de runbooks** *(ingestão + retrieval prontos; síntese via LLM ainda pendente)*
      Pipeline de ingestão de runbooks Markdown → embeddings → Qdrant (collection própria) → retrieval por similaridade. Veja [`agent/rag`](agent/rag/) e `k8s-mentor ingest-runbooks` / `analyze --with-runbooks`. Testado de ponta a ponta: cada cenário de demo recupera só o runbook correto, sem contaminação cruzada. Falta ainda injetar os trechos recuperados num prompt de geração (`ollama.Client.Chat` já existe, mas `analyze` ainda devolve os trechos brutos, não uma explicação sintetizada).
- [x] **Fase 3 — Correlação GitOps** *(via histórico nativo do Kubernetes, não ArgoCD/Flux — ver nota abaixo)*
      Correlaciona o timestamp do erro com o histórico de rollout (ReplicaSets) do Deployment dono do recurso — 100% Kubernetes nativo, sem depender de ArgoCD/Flux estarem instalados. Lê um annotation de commit convencionado no pod template (`k8s-mentor.dev/commit`, ver [`internal/k8s`](internal/k8s/)), do jeito que uma ferramenta de GitOps real também anotaria. Veja [`agent/correlation/gitops`](agent/correlation/gitops/) e `analyze --with-gitops`. Testado de ponta a ponta contra uma falha real (`CrashLoopBackOff` por env var quebrada): identificou a revisão e o commit exatos que introduziram o bug, com `suspect_recent_deploy: true`. Integração com ArgoCD/Flux de verdade (ler o commit deles, não o nosso annotation próprio) fica pra quando o projeto tiver um desses instalados no cluster de demo.
- [x] **Fase 4 — Correlação com Prometheus**
      Consulta `container_memory_working_set_bytes` na janela anterior ao erro e ajusta uma reta por mínimos quadrados pra detectar tendência de crescimento — não só o valor no momento da falha. Veja [`agent/correlation/metrics`](agent/correlation/metrics/), [`internal/prometheus`](internal/prometheus/) e `analyze --with-metrics`. Prometheus instalado via `scripts/setup-cluster.sh` (chart oficial `prometheus-community/prometheus`, minimalista — sem Grafana/Alertmanager/node-exporter, scrape a cada 5s pra caber amostras num ciclo de crash curto). Testado de ponta a ponta contra o `checkout-service` real: `growing_trend: true`, crescimento de 15MB→27MB (79%) em ~6.5min, 80 amostras.
- [x] **Fase 5 — Ação supervisionada (human-in-the-loop)**
      `analyze --with-action` sugere rollback (quando há deploy suspeito) ou restart (quando há vazamento de memória) — nunca executa. A execução mora só em `k8s-mentor apply-action`, que exige `--yes` explícito; sem isso, mostra o que faria e para. Veja [`agent/decision`](agent/decision/) e as duas únicas operações de escrita do agente em [`internal/k8s`](internal/k8s/) (`RollbackDeployment`/`RestartDeployment`), com RBAC própria e separada (`agent.rbac.allowActions`, desligável). Testado de ponta a ponta contra o `payment-worker` real: sugeriu rollback pra revisão 1, `apply-action` sem `--yes` não fez nada, `apply-action --yes` aplicou o rollback e o pod voltou a `1/1 Running` com o commit correto.

- [x] **Fase 6 — Execução periódica e notificação (observação contínua)**
      `k8s-mentor watch` roda o k8sgpt sozinho (empacotado na própria imagem — ver [`Dockerfile`](cmd/k8s-mentor/Dockerfile)) e passa o resultado pelos mesmos pilares do `analyze`, sem intervenção manual. Disparado por um `CronJob` (`deploy/helm/k8s-mentor/templates/watch.yaml`, `agent.watch.enabled`), roda com a mesma RBAC de leitura do agente — nenhuma permissão nova de escrita. Quando encontra algo, notifica um canal do Slack (`agent.watch.slack.enabled`, URL do webhook via `Secret`, nunca em texto plano) com o diagnóstico e a ação sugerida — **nunca executa nada sozinho**: a mensagem sempre inclui o comando exato pra um humano aprovar com `apply-action --yes`. Testado de ponta a ponta: `Job` rodou, achou incidentes reais, notificou um servidor de teste com sucesso.

Cada fase é demonstrável isoladamente e todas já foram validadas contra um cluster Kind real, não só localmente com mocks.

## Motivação / Talk

Este projeto nasceu como preparação para uma palestra no **TDC São Paulo**, sobre os limites de ferramentas de diagnóstico automatizado no Kubernetes e como um agente local, com memória, pode preencher a lacuna entre "o robô disse o que quebrou" e "o time entende por que quebrou e o que fazer".

- 🎤 Slides da talk: `[link a definir]`
- 🎥 Gravação: `[link a definir, se disponível após o evento]`
- 🧪 Demo ao vivo: roteiro completo em [`docs/DEMO.md`](docs/DEMO.md), com cenários 100% reproduzíveis via Kind.

Feedback, issues e PRs são bem-vindos — especialmente de quem já sofreu com alertas de madrugada que uma ferramenta de diagnóstico "resolveu" mas não explicou de verdade.

## Contribuindo

O projeto ainda está em fase inicial. Issues com sugestões de escopo, arquitetura ou cenários de demo são bem-vindas, especialmente para as fases de RAG, GitOps e métricas. Um guia de contribuição mais formal (`CONTRIBUTING.md`) é o próximo passo de documentação.

## Licença

Distribuído sob a licença **Apache 2.0** — a mesma usada pela maior parte do ecossistema CNCF (Kubernetes, Prometheus, ArgoCD, entre outros), o que facilita adoção e contribuição por quem já transita nesse ecossistema. Veja [`LICENSE`](LICENSE).
