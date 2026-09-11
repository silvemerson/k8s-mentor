# Roteiro de Apresentação — K8sGPT resolve 70%. E os outros 30%? (k8s-mentor)

**Evento:** TDC São Paulo
**Palestrante:** Emerson Silva · 4Linux
**Tempo total estimado:** ~27,25 min (soma dos rótulos por slide) + 5–7 min perguntas (alvo: 30–35 min) — recalibrado após revisão; sem o slide 19 opcional, cai pra ~26,25 min
**Formato:** prints reais (não é demo ao vivo) — ver `../docs/DEMO.md` e o artifact ["Do K8sGPT ao Rollback"](https://claude.ai/code/artifact/e722a5bf-dbda-4331-a945-aa3b8e801a95)

> **Numeração:** 1 entrada = 1 slide real do `slides.md` (22 no total). Se o deck mudar de novo, reconfira com `marp slides.md --allow-local-files --images png -o /tmp/slide.png` e contando os arquivos gerados — não use `pdftotext`, ele pula silenciosamente slides sem texto (ex: o slide 17, só com a imagem do Slack), nem conte `---` no markdown, o número de blocos de frontmatter/estilo bagunça a conta.

---

## Slide 1 — Título (1 min)

> **Tom:** direto, sem "bom dia, meu nome é...". O título já é o gancho — deixa ele fazer o trabalho.

"K8sGPT resolve 70% dos seus problemas. Sério — é uma ferramenta excelente, escaneia seu cluster e traduz erro críptico em explicação legível."

_[pausa]_

"Hoje eu vou te mostrar como resolver os outros 30% — com um agente rodando 100% na sua máquina, sem mandar nada do seu cluster pra fora."

---

## Slide 2 — Quem sou eu (45 seg)

> **Tom:** a menção à 4Linux entra aqui, embutida na apresentação pessoal — de propósito, pra não virar um slide institucional isolado logo na abertura.

"Emerson Silva, engenheiro DevOps/SRE na 4Linux — empresa que existe desde 2001, passou por Linux e software livre, DevOps, containers, cloud, e hoje trabalha com agentes de IA, que é exatamente o capítulo de onde nasce o projeto de hoje. O que eu trago não é teoria — é um projeto que construí, testei contra um cluster real, e onde vou mostrar os bugs que achei no caminho. Não é slideware."

---

## Slide 3 — Agenda (30 seg)

"Hoje: o que é o K8sGPT de verdade e onde ele já acerta. O que falta, mesmo assim. O que é o k8s-mentor e por que ele roda 100% local. Os 5 pilares que ele adiciona, um por um, com print real de cada. E fecho com uma pergunta que toda automação de infra precisa responder: quem decide?"

---

## Slide 4 — O que é o K8sGPT (1min30s)

> **Tom:** dados reais, não hype — estabelece credibilidade da ferramenta antes de criticar. Confere o número de stars/forks antes de subir no palco (github.com/k8sgpt-ai/k8sgpt), o dado aqui é de set/2026.

"Pra quem não conhece: K8sGPT é um projeto CNCF Sandbox, aceito em dezembro de 2023, licença Apache-2.0, escrito em Go. Hoje tem 8,2 mil stars e mil forks no GitHub — não é um projeto de fim de semana, é mantido ativamente e usado de verdade."

podman-desktop e atlantis

"Como funciona: ele roda um conjunto de analyzers — hoje são 14 habilitados por padrão, mais 21 opcionais — que escaneiam Pods, Deployments, Services, Ingress, PVC, Node... cada analyzer sabe verificar um tipo de recurso. Quando acha um problema, manda pro backend de IA traduzir em explicação legível. E aqui já importa pra nossa talk: são 15 backends suportados, incluindo dois locais — Ollama e LocalAI, ferramentas que rodam modelos de linguagem direto na sua máquina, sem chamar nenhuma API externa — em vez de só OpenAI, Azure, Bedrock e afins."

---

## Slide 5 — O problema (2 min)

> **Tom:** reconhece o valor do K8sGPT antes de apontar a lacuna — não é "essa ferramenta é ruim".

"Isso que acabei de descrever já resolve boa parte do trabalho de plantão. Mas ele é stateless — ou seja, não guarda memória entre uma execução e outra. Toda análise começa do zero, como se fosse a primeira vez que aquele cluster existe. Ele não sabe que esse mesmo `OOMKilled` — sigla de 'Out Of Memory Killed', quando o Kubernetes mata um container porque ele estourou o limite de memória — já aconteceu 5 vezes esse mês. Não conhece o runbook que seu time escreveu depois do último postmortem. Não sabe se um deploy recente é a causa raiz. Não vê a tendência de memória subindo no Prometheus — a ferramenta de métricas mais usada no ecossistema Kubernetes — antes do crash."

"Isso não é falha do K8sGPT — é o modelo. Ele responde 'o que está quebrado agora'. Quem responde 'isso já aconteceu antes, por quê, e o que fazer' continua sendo o humano, catando contexto em três painéis diferentes."

---

## Slide 6 — O que é o k8s-mentor (1min30s)

"k8s-mentor é uma camada que roda do lado do K8sGPT (não substitui ele) e adiciona 5 pilares de contexto: memória de incidentes, RAG sobre runbooks, correlação com deploys recentes, correlação com métricas, e sugestão de ação supervisionada."

"Ponto central: **100% local**. Ollama pra LLM, Qdrant pra memória vetorial, tudo rodando dentro do próprio cluster Kind — Kubernetes-in-Docker, um cluster completo rodando em containers na sua máquina, sem precisar de nuvem pra testar — ou no seu laptop."

"E o que é 'memória vetorial'? É um banco de dados especializado em guardar texto na forma de vetores numéricos — chamados embeddings — que capturam o significado da frase, não as palavras exatas. Assim dá pra perguntar 'isso é parecido com o que já vi antes?' comparando significado, não fazendo grep de texto. É a peça que sustenta tanto a memória de incidentes quanto o RAG."

"Nada do seu cluster — nomes de recursos, mensagens de erro, runbooks internos, métricas — sai pra uma API de terceiros."

_[mostra o diagrama de arquitetura — ver docs/ARCHITECTURE.md]_

---

## Slide 7 — Por que isso importa de verdade (1min30s)

> **Tom:** aqui é onde a talk paga a promessa do abstract — "sem depender de API externa, sem expor dado sensível". Não é detalhe de implementação, é o motivo de o projeto existir assim.

"Local não é capricho. São quatro razões práticas: primeiro, dado sensível não sai da rede — nome de recurso, mensagem de erro, runbook interno, um secret mal higienizado que vazou pra dentro de um log, nada disso vira payload pra API de terceiro."

"Segundo: sem custo por token. Rodando sozinho a cada 5 minutos, uma API paga vira uma conta que ninguém previu no orçamento."

"Terceiro: sem depender de internet no meio do incidente — se o cluster caiu, sua saída pra nuvem pode ter caído junto, exatamente na hora que você mais precisa da ferramenta."

"E quarto: sem vendor lock-in. Trocar de modelo é um `ollama pull`, não uma migração de contrato."

"Na prática: Ollama rodando Qwen2.5, Qdrant pra memória vetorial — os dois dentro do mesmo cluster ou no seu laptop, sem GPU dedicada."

---

## Slide 8 — Por que Qwen2.5 (1 min)

> **Tom:** rápido, técnico, sem se alongar — mostra que a escolha do modelo não foi aleatória.

"Por que Qwen2.5 especificamente e não outro modelo? Três motivos: as variantes pequenas rodam sem GPU dedicada, cabendo no laptop de demo. A família segue instrução e formato de saída de forma consistente mesmo nos tamanhos menores — e isso importa muito aqui, porque todo o pipeline depende de JSON bem formado voltando do modelo. E é Apache 2.0, mesma licença do K8sGPT — mantém a stack coerente com o discurso de open source de ponta a ponta."

---

## Slide 9 — Por que "mentor"? (1 min)

> **Tom:** leve, quase uma piada interna — mas amarra direto na tese central da talk.

"Um parênteses rápido sobre o nome. 'Mentor' vem da Odisseia — é o amigo a quem Ulisses deixa seu filho Telêmaco antes de ir pra guerra, não pra decidir por ele, mas pra aconselhar. É daí que a palavra existe até hoje."

"Por que 'mentor' e não 'auto-healer' ou 'auto-fixer'? Porque um mentor não faz o trabalho por você — ele já viu aquele problema antes, conhece o histórico, aponta o caminho. Mas quem decide continua sendo você."

"Guarda essa ideia, porque ela volta com tudo lá no final."

---

## Slide 10 — Momento 1: K8sGPT sozinho (1min30s)

> **Print real:** saída do `k8sgpt analyze --explain -o json` pro `checkout-service` (OOMKilled).

"Aqui o K8sGPT fazendo o que faz de melhor: diagnosticou certo, deu 5 passos de solução genéricos. Sem memória de que isso já aconteceu. Sem saber que existe um runbook sobre exatamente esse padrão. Guarda esse print — ele volta na próxima tela, enriquecido."

---

## Slide 11 — Quem faz o quê (1 min)

> **Tom:** slide de conteúdo, não de transição — deixa claro que não é concorrência, é complemento.

"Pra ficar bem claro antes de seguir: o K8sGPT escaneia o cluster em tempo real, cobre Pods, Deployments, PVCs, RBAC, e traduz erro críptico em explicação legível. Isso ele já faz muito bem, e cada análise dele começa do zero."

"O k8s-mentor entra do lado: lembra do que já aconteceu antes, cruza com o runbook que o time escreveu, aponta o deploy que provavelmente causou, confirma a tendência real na métrica. Um não substitui o outro — o k8s-mentor consome a saída do K8sGPT."

---

## Slide 12 — Momento 2: memória + RAG + métricas (2 min)

> **Print real:** mesmo evento, agora com `analyze --with-history --with-runbooks --with-metrics`.

"Antes de mostrar o print, um parênteses rápido, porque essa sigla vai aparecer bastante: RAG. Pensa assim — um médico plantonista recém-chegado examina o paciente só com o que aprendeu na faculdade. Um médico experiente faz a mesma coisa, mas também abre o prontuário do paciente e consulta um manual de protocolos do hospital antes de decidir. RAG é isso: antes de o modelo responder, a gente busca um documento de verdade — o runbook que o seu time escreveu — e entrega esse documento pra ele como consulta. A resposta deixa de ser genérica e vira uma resposta ancorada no que o seu time realmente sabe."

"E como o sistema sabe qual documento buscar? Ele não procura por palavra exata — procura por *significado*. Então mesmo que o runbook não use as mesmas palavras do erro, se o assunto é o mesmo, ele acha. É essa mesma ideia que dá a recorrência aqui: `recurrence: 9` não veio de comparar texto igual — veio de reconhecer que esses 9 eventos passados 'significam' a mesma coisa que o de agora."

---

## Slide 13 — Mesmo erro, contexto real (1min30s)

"Achou o runbook certo — `checkout-service-memory-leak.md` — que fala exatamente do vazamento de cache. O sistema vem com um número junto, tipo uma nota de 0 a 1 pra dizer o quanto ele tem certeza daquele achado — quanto mais perto de 1, mais forte a relação. Aqui deu 0.72, uma nota alta o suficiente pra eu confiar que é o documento certo, não coincidência. E, por fora, o Prometheus confirma com dado real: a memória vinha subindo de forma sustentada, não foi um pico aleatório."

"Isso é literalmente 'já aconteceu N vezes, aqui está o porquê documentado pelo seu time, e aqui a métrica confirmando que é o mesmo padrão' — automático."

---

## Slide 14 — Momento 3: ação supervisionada (1min30s)

> **Print real:** resultado de `analyze --with-gitops --with-action` (cenário payment-worker).

"Cenário diferente: um deploy quebrou um serviço 25 segundos antes do erro aparecer. Isso é correlação GitOps — GitOps é a prática de tratar o histórico de mudanças da infra como uma linha do tempo auditável, normalmente via ferramentas como ArgoCD ou Flux. Aqui eu não preciso de nenhuma delas: o k8s-mentor lê diretamente o histórico de rollout que o próprio Kubernetes já mantém — quais revisões existiram, quando cada uma foi aplicada — e cruza isso com o horário do erro. E sugere: 'volta pra revisão anterior'."

---

## Slide 15 — Sugerir → aprovar → confirmar (1min30s)

> **Print real:** sequência preview → `apply-action --yes` → pod recuperado.

"Repara a sequência: primeiro ele só *mostra* o que faria. Só executa com `--yes` explícito. E depois de aprovado — o pod realmente volta a rodar, log confirmando a conexão certa de novo."

"Sugerir não é decidir. Quem aperta o gatilho é humano, sempre."

---

## Slide 16 — Fase 6: rodando sozinho (1min30s)

"Até aqui, tudo rodou porque eu digitei um comando. Mas dá pra ir além: um `CronJob` — o mecanismo nativo do Kubernetes pra rodar uma tarefa em intervalos programados, tipo um cron do Linux, só que dentro do cluster — executa o k8sgpt sozinho, a cada poucos minutos, passando pelos mesmos 5 pilares, e — quando acha algo — notifica um canal do Slack."

"Repara o que a mensagem NÃO faz: ela não diz 'eu já resolvi'. Ela diz o que sugere, o porquê, e o comando exato pra alguém aprovar. Automatizei a observação, não a decisão."

"E ele notifica com critério: só interrompe o canal quando o padrão se confirma — recorrência, tendência de métrica ou ação sugerida. Evento isolado fica só no log, não vira alerta."

---

## Slide 17 — [print em tela cheia] Notificação real do Slack (1 min)

> **Print real.** Slide só de imagem — deixa o print respirar, sem texto competindo. Aponta no telão: recorrência, runbook, ação sugerida, o aviso final de "nada foi executado automaticamente".

---

## Slide 18 — Isso é testado de verdade (1min30s)

> **Tom:** credibilidade técnica, tom de confiança — não é confissão de bug, é rigor de engenharia.

"Rápido, porque acho importante: cada pilar que mostrei hoje foi validado contra um cluster real e a saída real do K8sGPT — não JSON forjado à mão. Isso endureceu o projeto de verdade: o parser foi ajustado ao formato real de saída, os thresholds de similaridade — o score mínimo, tipo aquele 0.72 que mostrei, abaixo do qual eu descarto o match por ser coincidência demais — foram calibrados com dados reais, a detecção de tendência endurecida no caso real de vazamento, a correlação de métrica corrigida pra pegar a série certa, e a notificação do Slack ganhou o filtro de recorrência que acabei de mostrar."

---

## Slide 19 — Arquitetura e stack (1 min, se sobrar tempo)

"Go, porque o ecossistema Kubernetes já é Go. Qdrant pela API simples via HTTP/gRPC. Ollama pelo modelo rodando local. Kind pra reproduzir isso na sua máquina depois — o repositório é aberto."

---

## Slide 20 — Quem decide? (1min30s)

> **Tom:** fecha com a tese, não com "obrigado".

"A pergunta que abriu essa talk era sobre o que falta nas ferramentas de diagnóstico. A resposta não é 'mais automação' — é memória e contexto. E a parte que eu quero que fique: quanto mais forte fica o sinal — mais recorrência, mais correlação, mais confiança — mais fácil fica a tentação de deixar a máquina decidir sozinha."

"k8s-mentor foi desenhado pra resistir exatamente essa tentação. Ele fica mais inteligente. A decisão continua sendo sua."

---

## Slide 21 — Curso gratuito (30 seg)

> **Tom:** rápido, institucional — CTA padrão da 4Linux, mesma usada em outras palestras.

"Antes de fechar: quem quiser aprofundar em DevOps, AIOps e SRE, a 4Linux tem um curso gratuito — QR code na tela pra se inscrever."

---

## Slide 22 — Obrigado! (30 seg)

_[link do repositório + contato na tela]_

"Obrigado! Fico por aqui pras perguntas."

---

## Backup / perguntas frequentes esperadas

- **"Por que não usar GPT-4/Claude na nuvem?"** → dado sensível de cluster (nomes internos, runbooks, secrets mal higienizados em mensagens de erro) não deveria sair da rede — ver `docs/ARCHITECTURE.md#decisões-de-design`.
- **"Isso substitui o K8sGPT?"** → não, é desacoplado de propósito — consome a saída dele hoje, mas o núcleo aceita qualquer fonte de diagnóstico.
- **"Por que não automatizar o rollback também?"** → foi cogitado durante o desenvolvimento (Prometheus alertando → rollback automático → Slack avisando depois) e descartado conscientemente — contradiz o pilar central do projeto. Bom gancho pra reforçar a tese de novo.
- **"Roda em produção hoje?"** → é pré-alpha, feito pra essa talk — mas os 5 pilares + a Fase 6 já rodaram de verdade contra um cluster real, não é só conceito.
