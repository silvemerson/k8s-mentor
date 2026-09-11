# Slides — k8s-mentor (TDC São Paulo)

Segue o padrão de `palestras/template-slides` (CSS, capa, bio, "Sobre a 4Linux",
"Curso gratuito" e "Obrigado" idênticos ao template — a referência viva mais
recente é `log-to-postmortem-slm-ollama/slides`): [Marp](https://marp.app/) +
um roteiro separado com as falas.

- [`slides.md`](slides.md) — os slides em si (Marp)
- [`roteiro-palestra.md`](roteiro-palestra.md) — o que falar em cada slide, com timing (alvo: 30min) — 1 entrada por slide real, numeração conferida via `--images png` (não `pdftotext`, ver nota no topo do roteiro)
- [`assets/`](assets/) — onde os prints capturados manualmente entram (ver abaixo)
- [`images/`](images/) — ativos "padrão" copiados do template (logos CNCF Campinas/DOUG, QR codes de curso/contato)

## Pré-requisito

```bash
npm install -g @marp-team/marp-cli
```

## Visualizar no browser com live reload

```bash
marp --watch slides.md
```

## Exportar para PDF

```bash
marp slides.md --pdf --allow-local-files -o slides.pdf
```

## Exportar para HTML

```bash
marp slides.md --html --allow-local-files -o slides.html
```

## Prints que faltam capturar (ver `../docs/DEMO.md`)

O deck já referencia os arquivos abaixo em `assets/` — falta só colocá-los lá (screenshot real do terminal/Slack, não print de tela inteira):

| Arquivo esperado | Conteúdo | Comando de referência |
|---|---|---|
| `momento-1-k8sgpt-sozinho.png` | K8sGPT sozinho, sem contexto | `k8sgpt analyze --explain -o json` |
| `momento-2-tres-pilares.png` | memória + RAG + métricas juntos | `analyze --with-history --with-runbooks --with-metrics` |
| `momento-3a-sugestao.png` | correlação GitOps + sugestão de rollback | `analyze --with-gitops --with-action` |
| `momento-3b-apply-action.png` | preview → `--yes` → pod recuperado | `apply-action` (duas chamadas) + `kubectl get pods` |
| `momento-4-slack.png` | notificação real do Slack (Fase 6) | já capturado — é o print que você já mandou nesta sessão |

O artifact ["Do K8sGPT ao Rollback"](https://claude.ai/code/artifact/e722a5bf-dbda-4331-a945-aa3b8e801a95) serve de referência visual/formatação pros 3 primeiros — não precisa printar ele, é só inspiração de como apresentar o JSON de forma legível.

## Diagrama de arquitetura (slide "O que é o k8s-mentor")

O slide deixa um comentário `<!-- diagrama ... -->` no lugar — reaproveite o mermaid de [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md#diagrama-de-componentes), renderizado como imagem (ex: exportando do mermaid.live) e salvo em `assets/arquitetura.png`.
