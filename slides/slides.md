---
marp: true
theme: gaia
class:
  - lead
  - invert
paginate: true
backgroundColor: #0B1426
lang: pt-BR
footer: '![height:40px](4linux-logo/logo-4linux.png)'
style: |
  /* @theme 4linux — padrão de palestras/template-slides, com os .tag do k8s-mentor */
  @import 'gaia';

  :root {
    --color-background: #0B1426;
    --color-foreground: #FFFFFF;
    --color-highlight: #00E6CC;
    --color-accent: #00C4A7;
    --color-secondary: #4ECDC4;
    --color-dimmed: #8B9DC3;
    --4linux-green: #00E6CC;
    --4linux-dark-green: #00C4A7;
    --4linux-blue: #4ECDC4;
    --4linux-dark: #0B1426;
  }

  section {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    background-color: var(--4linux-dark);
    background: linear-gradient(135deg, #0B1426 0%, #1a2332 100%);
    color: var(--color-foreground);
    font-size: 28px;
    line-height: 1.4;
    padding: 60px;
    max-width: 1280px;
    margin: 0 auto;
    box-sizing: border-box;
    position: relative;
  }

  section::before {
    content: "";
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 4px;
    background: linear-gradient(90deg, var(--4linux-green) 0%, #f15a29 100%) !important;
    z-index: 10;
  }

  section p {
    margin-bottom: 0.8em;
    max-width: 100%;
    word-wrap: break-word;
    hyphens: auto;
  }

  section ul {
    margin-left: 1.2em;
    max-width: calc(100% - 1.2em);
  }

  section li {
    margin-bottom: 0.5em;
    line-height: 1.3;
    word-wrap: break-word;
    hyphens: auto;
  }

  section h1 {
    font-size: 2.4em;
    font-weight: 700;
    color: var(--4linux-green);
    text-shadow: 0 0 20px rgba(0, 230, 204, 0.3);
    margin-bottom: 0.8em;
    line-height: 1.1;
    border-bottom: none;
    white-space: normal;
  }

  section h2 {
    font-size: 2.2em;
    font-weight: 600;
    color: var(--color-foreground);
    margin-bottom: 0.8em;
    position: relative;
  }

  section h2::before {
    content: "";
    position: absolute;
    left: -25px;
    top: 50%;
    transform: translateY(-50%);
    width: 4px;
    height: 50px;
    background: linear-gradient(135deg, var(--4linux-green), var(--4linux-blue));
    border-radius: 2px;
  }

  section h3 {
    font-size: 1.6em;
    font-weight: 500;
    color: var(--4linux-blue);
    margin-bottom: 0.6em;
  }

  section strong {
    color: var(--4linux-green);
    font-weight: 700;
  }

  section code {
    background: rgba(0, 230, 204, 0.1);
    color: var(--4linux-green);
    padding: 0.2em 0.5em;
    border-radius: 4px;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    border: 1px solid rgba(0, 230, 204, 0.2);
  }

  section pre {
    background: #060e1a;
    border: 1px solid rgba(0, 230, 204, 0.25);
    border-radius: 8px;
    padding: 1em 1.2em;
    margin: 0.6em 0;
  }

  section pre code {
    background: transparent;
    border: none;
    padding: 0;
    color: #e2e8f0;
    font-size: 0.82em;
    line-height: 1.6;
  }

  section ul li::marker {
    color: var(--4linux-green);
  }

  section::after {
    font-weight: 500;
    content: attr(data-marpit-pagination) ' / ' attr(data-marpit-pagination-total);
    font-size: 14px;
    color: var(--4linux-green);
    background: none;
    border: none;
    box-shadow: none;
    backdrop-filter: none;
    padding: 0;
    border-radius: 0;
    position: fixed;
    top: 25px;
    right: 30px;
  }

  /* Tags de seção — específico do k8s-mentor, não faz parte do template base */
  .tag {
    display: inline-block;
    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
    font-size: 0.55em;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    padding: 3px 10px;
    border-radius: 5px;
    margin-bottom: 10px;
  }
  .tag.neutral { color: #8B9DC3; background: rgba(139,157,195,0.12); border: 1px solid rgba(139,157,195,0.3); }
  .tag.accent  { color: var(--4linux-green); background: rgba(0,230,204,0.12); border: 1px solid rgba(0,230,204,0.35); }
  .tag.warn    { color: #f0ac6e; background: rgba(240,172,110,0.13); border: 1px solid rgba(240,172,110,0.38); }
  .tag.good    { color: #6adba0; background: rgba(106,219,160,0.13); border: 1px solid rgba(106,219,160,0.38); }
---
<!-- _paginate: false -->
<style scoped>
section {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  height: 100%;
  text-align: center;
  padding: 80px;
  position: relative;
}
h1 {
  font-size: 2.2rem;
  font-weight: 800;
  background: linear-gradient(135deg, #00E6CC 0%, #00E6CC 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: 0.8rem;
  line-height: 1.1;
  letter-spacing: -0.02em;
  text-shadow: none;
}
h2 {
  font-size: 1.4rem;
  font-weight: 400;
  color: #F8FAFC;
  opacity: 0.85;
  margin: 0 0 1.2rem 0;
}
h2::before { display: none; }
h3 {
  font-size: 1.3rem;
  color: #4ECDC4;
  font-weight: 500;
  margin: 0.20rem 0;
  letter-spacing: 0.5px;
}
</style>

# K8sGPT resolve 70% dos seus problemas.

## Vou te mostrar como resolver os outros 30% com um agente local

### Emerson Silva

<div style="display:flex; justify-content:center; align-items:center; gap:40px; margin-top:1.5rem;">
  <img src="tdc-logo/tdc-sao-paulo.png" height="80">
</div>

---
<!-- _paginate: false -->
<style scoped>
section {
  display: flex;
  flex-direction: row;
  justify-content: flex-start;
  align-items: center;
  gap: 50px;
}
.wi-photo {
  flex-shrink: 0;
  width: 230px;
  height: 230px;
  object-fit: cover;
  border-radius: 20px;
  border: 3px solid rgba(0, 230, 204, 0.35);
  box-shadow: 0 0 30px rgba(0, 230, 204, 0.15);
}
</style>

<div>

# 🇧🇷 Emerson Silva

* Engenheiro DevOps na **4Linux**
* +9 anos em ambientes DevOps críticos
* Foco em **Kubernetes**, IaC e confiabilidade
* Palestrante, instrutor,escritor , ex-zagueiro 
* Community Lead de Kubernetes na DOUGBrazil
* Organizador do CNCF Campinas

</div>

<div style="display:flex; flex-direction:column; align-items:center; gap:20px;">
  <img class="wi-photo" src="eu-rsrs/perfil2.png">
  <div style="display:flex; align-items:center; gap:24px;">
    <img src="images/cncf-campinas.png" height="60">
    <img src="images/doug.png" height="60">
  </div>
</div>

---

## Agenda

1. O que é o K8sGPT — e onde ele já acerta
2. O que falta, mesmo assim
3. O que é o k8s-mentor, e por que 100% local
4. Os 5 pilares — um print real por vez
5. Fase 6: rodando sozinho, avisando no Slack
6. Testado de verdade, e quem decide no final

---

<span class="tag neutral">O K8sGPT</span>

# O que é o K8sGPT

Projeto **CNCF Sandbox** (aceito em dez/2023), Apache-2.0, escrito em Go.

- **8,2k stars** e **1,0k forks** no GitHub
- **14 analyzers** por padrão (Pod, Deployment, Service, Ingress, PVC, Node...) + 21 opcionais
- **15 backends de IA** — OpenAI, Azure, Bedrock... e local: **Ollama**, LocalAI
- Escaneia o cluster e traduz o problema em explicação legível


<!-- fonte: github.com/k8sgpt-ai/k8sgpt (stars/forks de set/2026) -->

---

<span class="tag neutral">O problema</span>

# O que falta no K8sGPT

- **Stateless** — cada análise começa do zero
- Não sabe se **já aconteceu antes**, nem quantas vezes
- Não conhece o **runbook** que seu time escreveu
- Não correlaciona com **deploys recentes**
- Não vê **tendência** de métrica antes do crash

*Ele responde "o que quebrou". O humano ainda cata o "por quê" em 3 painéis — e isso consome horas de plantão que não precisavam ser gastas assim.*

---

<span class="tag accent">A solução</span>

# k8s-mentor

Roda **do lado** do K8sGPT (não substitui) e adiciona:

1. Memória de incidentes
2. RAG sobre runbooks internos
3. Correlação com deploys (GitOps)
4. Correlação com métricas (Prometheus)
5. Ação supervisionada — nunca decide sozinho

**100% local**: Ollama + Qdrant. Sem depender de API externa, sem expor dado sensível nem secret do cluster pra fora da rede.

<!--
diagrama de arquitetura vai aqui — reaproveitar o mermaid de
../docs/ARCHITECTURE.md#diagrama-de-componentes (renderizar como imagem)
-->

---

<span class="tag accent">100% local</span>

# Por que isso importa de verdade

- **Dado sensível não sai da rede** — nome de recurso, erro, runbook, secret mal higienizado num log
- **Sem custo por token** — rodando sozinho a cada 5min, uma API paga vira conta que ninguém previu
- **Sem depender de internet no incidente** — se o cluster caiu, sua saída pra nuvem pode ter caído junto
- **Sem vendor lock-in** — troca de modelo é `ollama pull`, não migração de contrato

---

<span class="tag neutral">Escolha de modelo</span>

# Por que Qwen2.5

Ollama rodando **Qwen2.5**, Qdrant pra memória vetorial — os dois no mesmo cluster ou no laptop.

- **Roda sem GPU dedicada** — variantes pequenas cabem no laptop de demo
- **Segue formato de saída de forma consistente** — o pipeline inteiro depende de JSON bem formado voltando do modelo
- **Apache 2.0** — mesma licença do K8sGPT, stack open source de ponta a ponta

---

<style scoped>
section p { margin: 0.5em 0; }
</style>

![bg right:38%](assets/Gemini_Generated_Image_ik2sljik2sljik2s.jpeg)

<span class="tag neutral">Por que "mentor"?</span>

# O nome não é acaso

Na *Odisseia*, **Méntor** é o amigo a quem Ulisses confia seu filho Telêmaco antes de ir pra guerra — não pra decidir por ele, mas pra aconselhar.

Um mentor não aperta o botão por você. Ele **conhece seu histórico**, **já viu o problema antes**, **aponta o caminho** — a decisão final continua sua.

É exatamente o papel deste agente ao lado do K8sGPT.

---

<span class="tag neutral">Momento 1</span>

# K8sGPT sozinho

![h:390](assets/momento-1-k8sgpt-sozinho.png)

<!-- print: k8sgpt analyze --explain -o json (checkout-service, OOMKilled) -->

---

<style scoped>
section h2 { margin-bottom: 0.5em; }
.split { display:grid; grid-template-columns:1fr 1fr; gap:2.5rem; margin-top:0.5em; }
.split ul { margin-top: 0.4em; padding-left: 1.2em; }
.split li { margin-bottom: 0.4em; }
</style>

## Quem faz o quê

<div class="split">
<div>

**K8sGPT**

- Escaneia o cluster em tempo real
- Traduz erro críptico em explicação legível
- Cobre Pods, Deployments, PVCs, RBAC...
- Cada análise começa do zero

</div>
<div>

**k8s-mentor**

- Lembra do que já aconteceu antes
- Cruza com o runbook que seu time escreveu
- Aponta o deploy que provavelmente causou
- Confirma a tendência real na métrica

</div>
</div>

*Consome a saída do K8sGPT — não substitui, adiciona o contexto que falta.*

---

<span class="tag accent">Momento 2</span>

# Memória + RAG + Métricas

![h:390](assets/momento-2-tres-pilares.png)

<!-- print: analyze --with-history --with-runbooks --with-metrics -->

---

## Mesmo erro, contexto real

- `recurrence: 9` — similaridade **semântica**, não palavra-chave
- Runbook certo achado: `checkout-service-memory-leak.md` (score 0.72)
- Prometheus confirma: memória em **crescimento sustentado**

---

<span class="tag warn">Momento 3</span>

# Ação supervisionada

![h:400](assets/momento-3a-sugestao.png)

<!-- print: analyze --with-gitops --with-action (payment-worker) -->

---

## Sugerir → aprovar → confirmar

![h:480](assets/momento-3b-apply-action.png)

<!-- print: apply-action preview + --yes + recuperação do pod -->

**Sugerir não é decidir. Quem aprova é humano, sempre.**

---

<span class="tag good">Fase 6</span>

# Rodando sozinho, avisando no Slack

Até aqui, tudo rodou porque eu digitei um comando. Um `CronJob` dentro do próprio cluster roda o k8sgpt sozinho, passa pelos mesmos pilares e — quando acha algo — notifica um canal do Slack.

Repara o que a mensagem **não** faz: não diz "eu resolvi".
Diz o que sugere, o porquê, e o comando pra aprovar.

E notifica **com critério**: só interrompe o canal quando o padrão se confirma — recorrência, tendência de métrica ou ação sugerida. Evento isolado fica só no log.

---
<!-- _paginate: false -->
<!-- _footer: "" -->
<style scoped>
section { display:flex; align-items:center; justify-content:center; padding: 30px; }
img { max-height: 100%; max-width: 100%; }
</style>

![](assets/momento-4-slack.png)

<!-- print real: notificação do Slack já capturada nesta sessão -->

---

<span class="tag good">Credibilidade</span>

# Isso é testado de verdade

Cada pilar foi validado contra um cluster **real** e a saída **real** do K8sGPT — não JSON forjado à mão:

- Parser ajustado ao formato real de saída do K8sGPT
- Thresholds de similaridade calibrados com dados reais, não texto de exemplo
- Detecção de tendência endurecida no caso real de vazamento de memória
- Correlação de métrica corrigida pra pegar a série certa no Prometheus
- Notificação do Slack com filtro de recorrência, pra avisar só o que importa

*Não é slideware. É a diferença entre uma demo que parece funcionar e uma que funciona.*

---

## Stack técnica

| Componente | Escolha |
|---|---|
| Agente | **Go** (ecossistema K8s nativo) |
| LLM local | **Ollama** + **Qwen2.5** |
| Memória vetorial | **Qdrant** |
| Cluster de demo | **Kind**, 3 nodes |
| Diagnóstico (fonte) | **K8sGPT**, desacoplado |

---
<!-- _paginate: false -->

# Quem decide?

Quanto mais forte o sinal — mais recorrência, mais correlação —
mais fácil é a tentação de deixar a máquina decidir sozinha.

**k8s-mentor foi desenhado pra resistir essa tentação.**

Ele fica mais inteligente. A decisão continua sendo sua.

---
<!-- _paginate: false -->

## Curso gratuito

![bg right:45% 70%](images/qrcode-curso.png)

### DevOps, AIOps e SRE Fundamentals

Curso gratuito da 4Linux

Escaneie o QR code pra se inscrever

* 🎓 4linux.com.br/cursos

---
<!-- _paginate: false -->

## Obrigado!

![bg right:45% 70%](images/qrcode.png)

### Contato

* 🔗 github.com/silvemerson/k8s-mentor
* 📝 emerson-silva.blog.br
* 💼 linkedin.com/in/silvemerson

Vamos manter contato!

<!--
ARQUIVADO — slide institucional "Sobre a 4Linux" (timeline 2001→2024+), removido
do fluxo pra não abrir a talk com cara de slide comercial. A menção à 4Linux
agora é falada junto do "Quem sou eu" (ver roteiro-palestra.md). Reaproveite
este bloco (cole de volta como um slide entre "Quem sou eu" e "Agenda", com
--- antes e depois) se algum dia fizer sentido reintroduzir a versão longa.

## (arquivado) Sobre a 4Linux

![height:52px](4linux-logo/logo-4linux.png)

Fundada em 2001, a 4Linux participou das principais transformações da área de tecnologia. Começamos com Linux e software livre, navegamos por Cloud, DevOps e Containers — e hoje atuamos com agentes de IA, sempre com base em **inovação aberta**.

<div class="timeline">
  <div class="tl-item"><span class="tl-year">2001</span><span class="tl-label">Linux &<br>Software Livre</span></div>
  <div class="tl-item"><span class="tl-year">2008</span><span class="tl-label">DevOps</span></div>
  <div class="tl-item"><span class="tl-year">2016</span><span class="tl-label">Containers &<br>Kubernetes</span></div>
  <div class="tl-item"><span class="tl-year">2019</span><span class="tl-label">Machine<br>Learning</span></div>
  <div class="tl-item"><span class="tl-year">2020</span><span class="tl-year" style="color:#4ECDC4">Cloud</span></div>
  <div class="tl-item"><span class="tl-year">2024+</span><span class="tl-label">IA com<br>Open Source</span></div>
</div>
-->
