# Arquitetura — k8s-mentor

> Este documento detalha os componentes, o fluxo de dados e as decisões de design do k8s-mentor. Para a visão de produto e motivação, veja o [README](../README.md).

## Sumário

- [Diagrama de componentes](#diagrama-de-componentes)
- [Fluxo de dados: de um erro do K8sGPT a uma resposta enriquecida](#fluxo-de-dados-de-um-erro-do-k8sgpt-a-uma-resposta-enriquecida)
- [Decisões de design](#decisões-de-design)
- [Estrutura de pastas do repositório](#estrutura-de-pastas-do-repositório)

## Diagrama de componentes

> **Nota de topologia:** o Ollama roda **fora** do cluster, no host, via docker-compose — mais simples pra lidar com RAM/GPU do modelo do que dentro de um node do Kind (que já é Docker-em-Docker). Todo o resto do k8s-mentor (Qdrant + agente) roda **dentro** do Kind como microserviços normais (Deployment + Service via Helm), lado a lado com o cluster que está sendo diagnosticado. O container do Ollama fica anexado à mesma rede Docker fixa que o cluster usa, então o agente o alcança através de um `Service`/`EndpointSlice` interno apontando direto pro IP desse container — sem depender do gateway do host. Ver [`deploy/helm/k8s-mentor`](../deploy/helm/k8s-mentor/) e [`scripts/setup-cluster.sh`](../scripts/setup-cluster.sh).

```mermaid
flowchart TB
    subgraph Host["Host (fora do cluster)"]
        direction LR
        LLM[Ollama — docker-compose\nmodelo local (ex: llama3.1:8b)\nembeddings + geração]
    end

    subgraph K["Kind Cluster (demo) / Cluster K8s (prod)"]
        direction TB

        subgraph Workload["Workloads observados"]
            direction LR
            Pods[Pods / Services / PVCs\nHPAs / CRDs custom]
            APIServer[kube-apiserver\n(eventos)]
            Prom[(Prometheus)]
            Argo[ArgoCD / Flux\n(GitOps controller)]
        end

        subgraph Diag["Fontes de diagnóstico"]
            K8sGPT[K8sGPT\nanalyzers]
            FutSrc[futuro: logs diretos,\neventos kube-apiserver,\nalertas Prometheus]
        end

        Pods --> K8sGPT
        APIServer --> K8sGPT

        subgraph Mentor["k8s-mentor — microserviços dentro do Kind"]
            direction TB

            subgraph Ingest["Camada de Ingestão"]
                Norm[Normalizador\n(fonte → formato interno de evento)]
            end

            OllamaSvc["Service: ollama\n(EndpointSlice → host)"]

            subgraph MemLayer["Memória de Incidentes"]
                VDB[(Vector DB — Qdrant\nembeddings de eventos passados\n+ metadados: timestamp, namespace,\nresolução aplicada)]
            end

            subgraph RagLayer["RAG de Runbooks"]
                RunbookIndex[(Índice de runbooks\n/ postmortems locais)]
                Retriever[Retriever\n(top-k por similaridade)]
            end

            subgraph GitOpsLayer["Correlação GitOps"]
                GitLog[git log dos manifests]
                ArgoEvents[Eventos de sync\nArgoCD/Flux API]
            end

            subgraph MetricsLayer["Correlação de Métricas"]
                PromQL[Consultas PromQL\njanela pré-evento]
                Trend[Detector de tendência\n(ex: crescimento linear)]
            end

            subgraph Decision["Camada de Decisão"]
                Correlator[Correlator\n(agrega todos os sinais)]
                Prompt[Prompt Builder\n(monta contexto p/ o LLM)]
                ActionSuggest[Gerador de sugestão\nde ação]
            end

            Norm --> OllamaSvc
            Norm --> VDB
            VDB -- "eventos similares" --> Correlator
            RunbookIndex --> Retriever
            Retriever -- "trechos relevantes" --> Correlator
            GitLog --> Correlator
            ArgoEvents --> Correlator
            PromQL --> Trend --> Correlator
            Correlator --> Prompt --> OllamaSvc
            OllamaSvc --> ActionSuggest
        end

        subgraph OutLayer["Output"]
            Report[Diagnóstico enriquecido:\ncausa raiz provável + histórico\n+ runbook sugerido + ação proposta]
        end
    end

    subgraph SRE["SRE (human-in-the-loop)"]
        Gate{Aprova a ação\nsugerida?}
    end

    K8sGPT -- "output JSON\n(erro + explicação)" --> Ingest
    FutSrc -.->|planejado| Ingest
    ActionSuggest --> Report --> Gate
    Gate -- "sim" --> Exec[Executor\n(chama API do cluster)]
    Gate -- "não" --> Discard[Registra decisão\n(sem executar)]
    Exec --> Pods
    Exec --> VDB
    Discard --> VDB

    Argo -.-> ArgoEvents
    Prom -.-> PromQL
    OllamaSvc -.->|"chamada de rede\n(gateway IP do Docker)"| LLM
```

## Fluxo de dados: de um erro do K8sGPT a uma consulta enriquecida

1. **Captura.** O K8sGPT roda sua análise normal contra o cluster e produz uma saída estruturada (JSON) por recurso problemático: tipo, namespace, mensagem de erro, explicação gerada.
2. **Normalização.** O k8s-mentor consome essa saída (hoje via K8sGPT; no futuro, potencialmente via outras fontes) e a converte para um **formato interno de evento** — um schema estável que desacopla o resto do pipeline da fonte específica de diagnóstico.
3. **Embedding + busca de memória.** O evento normalizado é convertido em embedding (via Ollama) e usado para consultar o Vector DB por eventos passados semanticamente similares — mesmo que a mensagem de erro não seja idêntica palavra por palavra. O resultado inclui frequência, janela de tempo e, se houver, a resolução aplicada em ocorrências anteriores.
4. **Retrieval de runbooks.** Em paralelo, o mesmo embedding (ou uma consulta derivada dele) é usado para buscar no índice de runbooks/postmortems locais os trechos de documentação mais relevantes para esse tipo de erro.
5. **Correlação GitOps.** O k8s-mentor consulta o histórico de sync do ArgoCD/Flux e o `git log` dos manifests relevantes na janela de tempo anterior ao evento, procurando mudanças que possam ser a causa raiz.
6. **Correlação de métricas.** Para erros com componente temporal relevante (memória, CPU, latência, taxa de erro), consulta o Prometheus na janela anterior ao evento e roda uma detecção de tendência simples (ex: regressão linear) sobre a série.
7. **Agregação (Correlator).** Todos esses sinais — memória de incidentes, runbooks, GitOps, métricas — são agregados num único contexto estruturado.
8. **Geração (Prompt Builder + LLM).** Esse contexto é injetado num prompt para o modelo local via Ollama, que gera o diagnóstico final em linguagem natural: causa raiz provável, histórico relevante, runbook sugerido.
9. **Sugestão de ação (opcional).** Se aplicável, o LLM (ou uma camada de regras determinística sobre seu output) propõe uma ação concreta de remediação, com o comando/manifest exato que seria aplicado.
10. **Human-in-the-loop.** O output completo é apresentado ao SRE. Nenhuma ação é executada sem aprovação explícita. A decisão do SRE (aprovou, rejeitou, aplicou algo diferente) é gravada de volta no Vector DB, alimentando a memória para a próxima ocorrência.

## Decisões de design

### Por que 100% local

Esta é a decisão central do projeto, e o ponto de partida da própria palestra: **nada do que passa pelo k8s-mentor sai da rede local.**

Isso importa porque os dados que alimentam esse pipeline são, por natureza, sensíveis:

- Nomes de recursos, namespaces e labels podem revelar estrutura interna de sistemas e nomes de clientes/projetos.
- Mensagens de erro de Pods frequentemente vazam fragmentos de configuração, variáveis de ambiente, e ocasionalmente até paths ou trechos de secrets mal higienizados.
- Runbooks e postmortems internos são, por definição, documentação sensível sobre como o time opera, o que já falhou e como a infraestrutura está desenhada — exatamente o tipo de informação que uma empresa não quer enviar para uma API de terceiros só para obter uma explicação de erro.
- Métricas do Prometheus podem revelar padrões de tráfego, carga e comportamento de negócio.

Usar **Ollama** rodando localmente (ou dentro da própria rede do cluster) elimina essa superfície de exposição por completo: não há chamada de API externa, não há dependência de uptime de terceiros, não há custo por token, e não há necessidade de aprovação de compliance/segurança para "mandar dados do cluster de produção para uma LLM na nuvem" — uma barreira real e comum em ambientes corporativos.

O trade-off consciente: modelos locais (7-8B parâmetros, rodando em laptop/cluster comum) são menos capazes que os maiores modelos hospedados. O k8s-mentor mitiga isso não tentando competir em raciocínio geral, mas **compensando com contexto** — memória de incidentes, RAG de runbooks e correlação de dados estruturados fazem o trabalho pesado, deixando para o LLM a tarefa mais tratável de sintetizar contexto já relevante em uma explicação legível, em vez de "adivinhar" a causa raiz do zero.

### Por que desacoplado do K8sGPT

O K8sGPT é a fonte de diagnóstico de referência hoje, mas o schema interno de evento (passo 2 do fluxo acima) existe justamente para que outras fontes — logs diretos via `kubectl logs`/API, eventos do `kube-apiserver`, alertas do Alertmanager, saídas de outros scanners — possam alimentar o mesmo pipeline sem mudar o núcleo do agente. O nome do projeto reflete isso: não é "K8sGPT-mentor", é "k8s-mentor".

### Por que Kind para a demo

Reprodutibilidade sem depender de cloud, cota, custo ou conectividade é um requisito não negociável para uma demo de palestra: qualquer pessoa da audiência deve conseguir reproduzir localmente depois, e a demo ao vivo não deve depender de internet do evento (risco real e recorrente). Isso reforça, inclusive, a narrativa de "tudo roda local" do próprio projeto.

### Por que Ollama fica fora do cluster, mas o resto fica dentro

Todos os componentes do k8s-mentor são microserviços — mas nem todos precisam estar *dentro* do Kind pra isso valer:

- **Ollama roda no host, via docker-compose.** O node do Kind já é Docker-em-Docker; lidar com RAM/GPU de um modelo de 7-8B dentro dessa camada extra é mais complicado e menos previsível do que rodar o Ollama direto no host, com bind mount simples do cache de modelos e, se disponível, passagem de GPU nativa do Docker.
- **Qdrant e o agente rodam dentro do Kind**, porque são exatamente o tipo de carga que um cluster Kubernetes gerencia bem — Deployment, Service, storage via `PersistentVolumeClaim`, reagendamento automático entre nodes.
- Os dois lados se enxergam porque **compartilham a mesma rede Docker fixa** (`kind`, subnet `172.20.0.0/16`) — o container do Ollama recebe um IP estável nessa rede, alcançável diretamente pelos pods do cluster (mesmo padrão já usado pelo NFS server, ver abaixo). Isso evita tanto a fragilidade de descobrir um IP de gateway dinamicamente quanto a complexidade de rodar o LLM dentro do cluster.

### Por que MetalLB + NFS + Gateway API, adaptados do kind-infra-lab

A base de infraestrutura do cluster Kind (rede Docker fixa, MetalLB, NFS, Gateway API) não foi desenhada do zero — é adaptada do homelab pessoal do autor, [kind-infra-lab](https://github.com/silvemerson/kind-infra-lab), que já resolve esses problemas de forma testada:

- **Rede Docker fixa (subnet pinada)** garante que os IPs do Ollama e do NFS server sejam **previsíveis** entre execuções — sem isso, o Docker sorteia a subnet toda vez que a rede não existe ainda, e qualquer IP fixo documentado deixa de bater. Crítico para uma demo ao vivo, onde nada pode depender de "descobrir o IP na hora".
- **MetalLB** dá ao Kind um LoadBalancer real (via ARP/L2), algo que ele não tem nativamente — necessário pro Gateway API expor o agente com um IP de verdade, em vez de NodePort.
- **NFS** dá ao Qdrant um `storageClass` real dentro do cluster — a memória de incidentes sobrevive ao pod sendo reagendado em outro dos 3 nodes, em vez de sumir com um `emptyDir` local ao node.
- **Gateway API** (via NGINX Gateway Fabric) é a forma mais atual de expor HTTP no Kubernetes — CRDs desacopladas (`GatewayClass`, `Gateway`, `HTTPRoute`) em vez do `Ingress` clássico amarrado a anotações específicas de cada controller.

Como cada cluster Kind é sua própria API do Kubernetes, não há conflito em instalar Gateway API/MetalLB de novo neste cluster mesmo que outro cluster seu (como o `sparta` do kind-infra-lab) já tenha os dois — o único cuidado é usar uma faixa de IP do MetalLB que **não colida** com a de outros clusters compartilhando a mesma rede Docker (ver comentário em [`deploy/kind/loadbalancer/metallb-config.yaml`](../deploy/kind/loadbalancer/metallb-config.yaml)).

## Estrutura de pastas do repositório

Monorepo simples, organizado por convenção Go (`cmd`/`internal`) mais pastas de domínio específicas do projeto:

```
k8s-mentor/
├── README.md
├── LICENSE
├── docs/
│   ├── ARCHITECTURE.md        # este arquivo
│   └── DEMO.md                # roteiro e checklist da demo
│
├── cmd/
│   └── k8s-mentor/            # entrypoint do binário CLI (main.go)
│
├── internal/                   # código não exportado, específico da implementação
│   ├── k8s/                   # client-go, acesso ao cluster
│   ├── k8sgpt/                # adapter/parser da saída do K8sGPT
│   ├── ollama/                # cliente HTTP para o Ollama (embeddings + geração)
│   ├── vectordb/              # cliente Qdrant + operações de indexação/busca
│   ├── gitops/                # integração ArgoCD/Flux + git log
│   └── prometheus/            # cliente PromQL + detecção de tendência
│
├── agent/                      # lógica de domínio do agente (os 5 pilares)
│   ├── memory/                # memória de incidentes (grava/consulta eventos)
│   ├── rag/                   # ingestão e retrieval de runbooks
│   ├── correlation/
│   │   ├── gitops/            # correlação temporal com deploys
│   │   └── metrics/           # correlação com séries do Prometheus
│   └── decision/               # correlator + prompt builder + geração de sugestão de ação
│
├── deploy/
│   ├── kind/
│   │   ├── cluster.yaml            # cluster Kind de 3 nodes (control-plane + 2 workers)
│   │   ├── loadbalancer/           # IPAddressPool + L2Advertisement do MetalLB
│   │   ├── gateway-api/            # Gateway do k8s-mentor (+ GatewayClass, só referência)
│   │   └── nfs/                    # smoke test opcional do storageClass nfs-client
│   ├── helm/
│   │   └── k8s-mentor/             # chart Helm: Qdrant + Service/EndpointSlice do Ollama + agente
│   └── docker-compose/             # Ollama + NFS server, fora do cluster, na rede Docker fixa
│
├── demo/
│   ├── scenarios/             # manifests dos cenários de falha injetados (ver docs/DEMO.md)
│   ├── runbooks/              # runbooks de exemplo usados no RAG da demo
│   └── synthetic-data/        # histórico sintético de incidentes pré-carregado
│
└── scripts/
    ├── setup-cluster.sh       # sobe TUDO na ordem certa (rede, docker-compose, Kind, MetalLB, NFS, Gateway API, Helm)
    ├── teardown-cluster.sh    # derruba o que o setup cria
    └── seed-synthetic-memory.sh  # popula o Qdrant com histórico sintético pra demo
```

Racional rápido por pasta:

- **`cmd/` vs `internal/` vs `agent/`** — separação convencional em projetos Go: `cmd` é só o entrypoint fino; `internal` é infraestrutura/clientes técnicos (como falar com Ollama, Qdrant, K8sGPT, Prometheus, Git); `agent` é a lógica de domínio que implementa os 5 pilares descritos no README, consumindo os clientes de `internal`. Essa separação existe para que a lógica de negócio (`agent`) não fique acoplada aos detalhes de transporte/protocolo de cada integração.
- **`deploy/`** concentra as três formas previstas de rodar a stack: Kind (+ MetalLB + NFS + Gateway API) para o cluster de demo/dev, Helm para instalar o k8s-mentor dentro dele, e docker-compose para os dois componentes que ficam fora do cluster de propósito (Ollama, NFS server) — ver [Decisões de design](#decisões-de-design).
- **`demo/`** é intencionalmente separado de `deploy/kind`: aqui vivem os cenários de falha e os dados sintéticos específicos da apresentação, não infraestrutura reutilizável.
- **`scripts/`** existe pra não exigir que quem for reproduzir a demo decore a ordem de 6 passos manuais — `setup-cluster.sh`/`teardown-cluster.sh` encapsulam exatamente essa sequência, de forma idempotente.
