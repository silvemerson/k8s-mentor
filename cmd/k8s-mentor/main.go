// k8s-mentor adiciona memória local a diagnósticos do Kubernetes.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/silvemerson/k8s-mentor/agent/correlation/gitops"
	"github.com/silvemerson/k8s-mentor/agent/correlation/metrics"
	"github.com/silvemerson/k8s-mentor/agent/decision"
	"github.com/silvemerson/k8s-mentor/agent/memory"
	"github.com/silvemerson/k8s-mentor/agent/rag"
	"github.com/silvemerson/k8s-mentor/internal/k8s"
	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
	"github.com/silvemerson/k8s-mentor/internal/ollama"
	"github.com/silvemerson/k8s-mentor/internal/prometheus"
	"github.com/silvemerson/k8s-mentor/internal/slack"
	"github.com/silvemerson/k8s-mentor/internal/vectordb"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "analyze":
		err = analyze(os.Args[2:])
	case "ingest-runbooks":
		err = ingestRunbooks(os.Args[2:])
	case "apply-action":
		err = applyAction(os.Args[2:])
	case "watch":
		err = watch(os.Args[2:])
	case "serve":
		err = serve(os.Args[2:])
	case "help", "--help", "-h":
		usage()
		return
	default:
		err = fmt.Errorf("comando desconhecido: %s", os.Args[1])
	}
	if err != nil {
		log.Printf("erro: %v", err)
		os.Exit(1)
	}
}

func analyze(args []string) error {
	flags := flag.NewFlagSet("analyze", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	input := flags.String("input", "-", "arquivo JSON do k8sgpt analyze -o json; use - para stdin")
	namespace := flags.String("namespace", "", "restringe os eventos a um namespace")
	withHistory := flags.Bool("with-history", false, "busca incidentes semelhantes antes de gravar o evento")
	withRunbooks := flags.Bool("with-runbooks", false, "busca trechos de runbook relevantes (ver ingest-runbooks)")
	withGitOps := flags.Bool("with-gitops", false, "correlaciona o erro com o histórico de rollout do Deployment dono do recurso")
	withMetrics := flags.Bool("with-metrics", false, "correlaciona o erro com a tendência de memória do Pod no Prometheus")
	withAction := flags.Bool("with-action", false, "sugere uma ação de remediação (nunca executa — ver apply-action). Some com o Deployment se --with-gitops também não for passado")
	ollamaURL := flags.String("ollama-url", environment("OLLAMA_BASE_URL", "http://localhost:11434"), "URL base do Ollama")
	embeddingModel := flags.String("embedding-model", environment("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"), "modelo local de embeddings")
	qdrantURL := flags.String("qdrant-url", environment("QDRANT_BASE_URL", "http://localhost:6333"), "URL base do Qdrant")
	prometheusURL := flags.String("prometheus-url", environment("PROMETHEUS_BASE_URL", "http://prometheus-server.monitoring.svc.cluster.local"), "URL base do Prometheus")
	timestamp := flags.String("timestamp", "", "RFC3339; substitui \"agora\" como timestamp do evento (usado pra semear histórico sintético com datas passadas)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("analyze não aceita argumentos posicionais")
	}
	eventTime := time.Now()
	if *timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, *timestamp)
		if err != nil {
			return fmt.Errorf("--timestamp inválido (esperado RFC3339): %w", err)
		}
		eventTime = parsed
	}
	data, err := readInput(*input)
	if err != nil {
		return err
	}
	results, err := runPipeline(data, pipelineOptions{
		namespace:      *namespace,
		eventTime:      eventTime,
		withHistory:    *withHistory,
		withRunbooks:   *withRunbooks,
		withGitOps:     *withGitOps,
		withMetrics:    *withMetrics,
		withAction:     *withAction,
		ollamaURL:      *ollamaURL,
		embeddingModel: *embeddingModel,
		qdrantURL:      *qdrantURL,
		prometheusURL:  *prometheusURL,
	})
	if err != nil {
		return err
	}
	if *namespace != "" && len(results) == 0 {
		return fmt.Errorf("nenhum evento corresponde ao namespace %q", *namespace)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"incidents": results})
}

type result struct {
	Event    k8sgpt.Event         `json:"event"`
	History  memory.History       `json:"history"`
	Runbooks []rag.Match          `json:"relevant_runbooks,omitempty"`
	GitOps   *gitops.Result       `json:"gitops,omitempty"`
	Metrics  *metrics.Result      `json:"metrics,omitempty"`
	Action   *decision.Suggestion `json:"action,omitempty"`
}

type pipelineOptions struct {
	namespace      string
	eventTime      time.Time
	withHistory    bool
	withRunbooks   bool
	withGitOps     bool
	withMetrics    bool
	withAction     bool
	ollamaURL      string
	embeddingModel string
	qdrantURL      string
	prometheusURL  string
}

// runPipeline é o núcleo compartilhado entre `analyze` (invocação manual, um
// diagnóstico por vez) e `watch` (Fase 6 — execução periódica dentro do
// cluster, chamando o k8sgpt sozinho). Recebe o JSON bruto do k8sgpt e
// devolve os incidentes já passados pelos pilares habilitados.
func runPipeline(data []byte, opts pipelineOptions) ([]result, error) {
	events, err := k8sgpt.Parse(data, opts.eventTime)
	if err != nil {
		return nil, err
	}
	// Ollama e Qdrant são compartilhados entre memória e RAG — um único
	// cliente HTTP de cada, não um por pilar.
	embeddings := ollama.New(opts.ollamaURL, opts.embeddingModel)
	vectors := vectordb.New(opts.qdrantURL)
	memStore := memory.New(embeddings, vectors)
	ragStore := rag.New(embeddings, vectors)
	promClient := prometheus.New(opts.prometheusURL)

	var k8sClient *k8s.Client
	if opts.withGitOps {
		k8sClient, err = k8s.New()
		if err != nil {
			return nil, fmt.Errorf("--with-gitops requer acesso ao cluster: %w", err)
		}
	}

	results := make([]result, 0, len(events))
	for _, event := range events {
		if opts.namespace != "" && event.Namespace != opts.namespace {
			continue
		}
		history, err := memStore.Remember(context.Background(), event, opts.withHistory)
		if err != nil {
			return nil, fmt.Errorf("%s/%s: %w", event.Kind, event.Name, err)
		}
		var runbooks []rag.Match
		if opts.withRunbooks {
			query := strings.TrimSpace(event.Message + "\n" + event.Explanation)
			// 0.70, não 0.75: calibrado contra saída REAL do k8sgpt (não texto
			// forjado à mão). Um match genuíno (OOMKilled real vs. o runbook de
			// memory leak) pontuou 0.66–0.74; um evento genuinamente sem
			// runbook relevante (ConfigMap não usado) pontuou até 0.663 contra
			// o runbook errado. 0.70 é o corte que separa os dois casos reais
			// testados — ver internal/k8sgpt parser_test.go e a sessão que
			// validou isso contra o cluster de demo.
			runbooks, err = ragStore.Retrieve(context.Background(), query, 5, 0.70)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", event.Kind, event.Name, err)
			}
		}
		var gitopsResult *gitops.Result
		if opts.withGitOps {
			correlated, err := gitops.Correlate(context.Background(), k8sClient, event)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: correlacionar com GitOps: %w", event.Kind, event.Name, err)
			}
			gitopsResult = &correlated
		}
		var metricsResult *metrics.Result
		if opts.withMetrics {
			correlated, err := metrics.Correlate(context.Background(), promClient, event)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: correlacionar com Prometheus: %w", event.Kind, event.Name, err)
			}
			metricsResult = &correlated
		}
		var action *decision.Suggestion
		if opts.withAction {
			suggestion := decision.Suggest(event, gitopsResult, metricsResult)
			action = &suggestion
		}
		results = append(results, result{Event: event, History: history, Runbooks: runbooks, GitOps: gitopsResult, Metrics: metricsResult, Action: action})
	}
	return results, nil
}

func ingestRunbooks(args []string) error {
	flags := flag.NewFlagSet("ingest-runbooks", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("dir", "demo/runbooks", "diretório com runbooks em Markdown (.md), varredura recursiva")
	ollamaURL := flags.String("ollama-url", environment("OLLAMA_BASE_URL", "http://localhost:11434"), "URL base do Ollama")
	embeddingModel := flags.String("embedding-model", environment("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"), "modelo local de embeddings")
	qdrantURL := flags.String("qdrant-url", environment("QDRANT_BASE_URL", "http://localhost:6333"), "URL base do Qdrant")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("ingest-runbooks não aceita argumentos posicionais")
	}
	store := rag.New(ollama.New(*ollamaURL, *embeddingModel), vectordb.New(*qdrantURL))
	count, err := store.IndexDir(context.Background(), *dir)
	if err != nil {
		return err
	}
	log.Printf("indexados %d trecho(s) de runbook de %q", count, *dir)
	return nil
}

// applyAction é a ÚNICA porta de entrada pra escrita no cluster — analyze é
// estritamente leitura, mesmo com --with-action (que só sugere). Sem --yes,
// mostra exatamente o que seria feito e para aí; nada é executado.
func applyAction(args []string) error {
	flags := flag.NewFlagSet("apply-action", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	input := flags.String("input", "-", "arquivo JSON de uma decision.Suggestion (ex: .incidents[0].action da saída do analyze --with-action); use - para stdin")
	yes := flags.Bool("yes", false, "confirma e executa a ação. Sem isso, só mostra o que seria feito")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("apply-action não aceita argumentos posicionais")
	}
	data, err := readInput(*input)
	if err != nil {
		return err
	}
	var suggestion decision.Suggestion
	if err := json.Unmarshal(data, &suggestion); err != nil {
		return fmt.Errorf("decodificar sugestão: %w", err)
	}

	if suggestion.Type == "" || suggestion.Type == "none" {
		fmt.Fprintln(os.Stderr, "Nenhuma ação sugerida — nada a fazer.")
		return nil
	}

	fmt.Fprintf(os.Stderr, "Ação sugerida: %s\nMotivo: %s\nComando equivalente: %s\n", suggestion.Type, suggestion.Reason, suggestion.Command)
	if !*yes {
		fmt.Fprintln(os.Stderr, "\nNada foi executado. Rode de novo com --yes pra aplicar.")
		return nil
	}

	client, err := k8s.New()
	if err != nil {
		return fmt.Errorf("apply-action requer acesso ao cluster: %w", err)
	}
	ctx := context.Background()
	switch suggestion.Type {
	case "rollback":
		err = client.RollbackDeployment(ctx, suggestion.Namespace, suggestion.Deployment, suggestion.ReplicaSet)
	case "restart":
		err = client.RestartDeployment(ctx, suggestion.Namespace, suggestion.Deployment)
	default:
		return fmt.Errorf("tipo de ação desconhecido: %q", suggestion.Type)
	}
	if err != nil {
		return fmt.Errorf("aplicar ação: %w", err)
	}
	fmt.Fprintln(os.Stderr, "✅ Ação aplicada.")
	return nil
}

// watch é a Fase 6: execução periódica dentro do cluster (via CronJob, ver
// deploy/helm/k8s-mentor). Chama o k8sgpt sozinho (empacotado na mesma
// imagem — ver Dockerfile) e passa o resultado pelo mesmo runPipeline do
// `analyze`. Continua estritamente leitura: uma ação sugerida fica só
// registrada no relatório (stdout — os logs do Job), esperando alguém
// rodar `apply-action --yes` manualmente. watch nunca chama apply-action
// sozinho, de propósito — isso destruiria o pilar 5 (ação supervisionada).
func watch(args []string) error {
	flags := flag.NewFlagSet("watch", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	namespace := flags.String("namespace", "demo", "namespace observado pelo k8sgpt")
	k8sgptPath := flags.String("k8sgpt-path", "/usr/local/bin/k8sgpt", "caminho do binário do k8sgpt empacotado na imagem")
	k8sgptConfig := flags.String("k8sgpt-config", "/etc/k8sgpt/k8sgpt.yaml", "config do k8sgpt (provider Ollama) — ver ConfigMap no chart")
	k8sgptBackend := flags.String("k8sgpt-backend", "ollama", "backend de IA do k8sgpt")
	withHistory := flags.Bool("with-history", true, "busca incidentes semelhantes antes de gravar o evento")
	withRunbooks := flags.Bool("with-runbooks", true, "busca trechos de runbook relevantes")
	withGitOps := flags.Bool("with-gitops", true, "correlaciona com o histórico de rollout")
	withMetrics := flags.Bool("with-metrics", true, "correlaciona com a tendência de memória no Prometheus")
	withAction := flags.Bool("with-action", true, "sugere uma ação de remediação — nunca executa, fica só no relatório")
	ollamaURL := flags.String("ollama-url", environment("OLLAMA_BASE_URL", "http://ollama:11434"), "URL base do Ollama")
	embeddingModel := flags.String("embedding-model", environment("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text"), "modelo local de embeddings")
	qdrantURL := flags.String("qdrant-url", environment("QDRANT_BASE_URL", "http://qdrant:6333"), "URL base do Qdrant")
	prometheusURL := flags.String("prometheus-url", environment("PROMETHEUS_BASE_URL", "http://prometheus-server.monitoring.svc.cluster.local"), "URL base do Prometheus")
	slackWebhookURL := flags.String("slack-webhook-url", environment("SLACK_WEBHOOK_URL", ""), "Incoming Webhook do Slack; vazio desativa a notificação (só avisa, nunca decide nem executa)")
	slackMinRecurrence := flags.Int("slack-min-recurrence", environmentInt("SLACK_MIN_RECURRENCE", 2), "só notifica o Slack se recurrence >= N (ou se houver tendência de crescimento/ação sugerida) — evita notificar todo evento isolado; o log/JSON continuam completos, esse filtro só reduz ruído no Slack")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("watch não aceita argumentos posicionais")
	}

	log.Printf("watch: rodando k8sgpt no namespace %q...", *namespace)
	cmd := exec.Command(*k8sgptPath, "analyze",
		"--namespace", *namespace,
		"--backend", *k8sgptBackend,
		"--config", *k8sgptConfig,
		"--explain",
		"-o", "json",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		// k8sgpt sai com status != 0 em alguns casos mesmo tendo gerado JSON
		// válido (ex: quando acha problemas) — só trata como erro de
		// execução de verdade quando não saiu JSON nenhum no stdout.
		return fmt.Errorf("rodar k8sgpt: %w (stderr: %s)", err, stderr.String())
	}

	k8sgptJSON, err := extractJSONObject(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("saída do k8sgpt: %w (stderr: %s)", err, stderr.String())
	}

	results, err := runPipeline(k8sgptJSON, pipelineOptions{
		namespace:      *namespace,
		eventTime:      time.Now(),
		withHistory:    *withHistory,
		withRunbooks:   *withRunbooks,
		withGitOps:     *withGitOps,
		withMetrics:    *withMetrics,
		withAction:     *withAction,
		ollamaURL:      *ollamaURL,
		embeddingModel: *embeddingModel,
		qdrantURL:      *qdrantURL,
		prometheusURL:  *prometheusURL,
	})
	if err != nil {
		return fmt.Errorf("processar diagnóstico: %w", err)
	}

	if len(results) == 0 {
		log.Printf("watch: nenhum problema encontrado em %q — tudo certo.", *namespace)
		return nil
	}

	log.Printf("watch: %d incidente(s) em %q:", len(results), *namespace)
	for _, line := range summarizeResults(results) {
		log.Print("  " + line)
	}

	if *slackWebhookURL != "" {
		notifiable := filterNotifiable(results, *slackMinRecurrence)
		if len(notifiable) == 0 {
			log.Printf("watch: %d incidente(s) encontrados, mas nenhum atinge o limiar de notificação (recurrence >= %d, tendência de crescimento ou ação sugerida) — Slack não avisado, só o log/JSON.", len(results), *slackMinRecurrence)
		} else {
			if len(notifiable) < len(results) {
				log.Printf("watch: notificando só %d de %d incidente(s) no Slack (os demais são eventos isolados, abaixo do limiar de confiança).", len(notifiable), len(results))
			}
			message := slackReport(*namespace, notifiable)
			if err := slack.New(*slackWebhookURL).Notify(context.Background(), message); err != nil {
				// Falha em notificar o Slack não derruba o Job — o relatório
				// completo já está nos logs (kubectl logs), que é o que importa
				// de verdade. Slack é conveniência, não a fonte de verdade.
				log.Printf("watch: aviso — falha ao notificar Slack: %v", err)
			}
		}
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{"incidents": results})
}

// summarizeResults gera uma linha legível por incidente — reaproveitada
// tanto no log do Job (`kubectl logs`) quanto na mensagem do Slack, pra não
// ter duas versões do mesmo resumo divergindo com o tempo.
func summarizeResults(results []result) []string {
	lines := make([]string, 0, len(results))
	for _, r := range results {
		line := fmt.Sprintf("%s/%s: %s", r.Event.Kind, r.Event.Name, r.Event.Message)
		if r.History.Recurrence > 0 {
			line += fmt.Sprintf(" [recorrência: %dx]", r.History.Recurrence)
		}
		if len(r.Runbooks) > 0 {
			line += fmt.Sprintf(" [runbook: %s]", r.Runbooks[0].Path)
		}
		if r.Metrics != nil && r.Metrics.Growing {
			line += " [tendência: memória em crescimento]"
		}
		if r.Action != nil && r.Action.Type != "" && r.Action.Type != "none" {
			line += fmt.Sprintf(" [AÇÃO SUGERIDA: %s]", r.Action.Type)
		}
		lines = append(lines, line)
	}
	return lines
}

// filterNotifiable decide o que vale a pena notificar no Slack — não é
// filtro de "falso positivo" (o k8sgpt só reporta problemas reais; não há
// detecção especulativa a filtrar), é filtro de RUÍDO: um evento isolado,
// sem recorrência, sem tendência confirmada e sem ação sugerida, não
// justifica interromper alguém no Slack — mas continua no log/JSON (a fonte
// de verdade nunca é podada, só a notificação). Um incidente entra se
// qualquer sinal de contexto reforça que não é ruído passageiro:
//   - já aconteceu >= minRecurrence vezes (memória de incidentes);
//   - a métrica confirma tendência de crescimento sustentado, não um pico;
//   - há uma ação de remediação sugerida (GitOps encontrou causa provável).
func filterNotifiable(results []result, minRecurrence int) []result {
	notifiable := make([]result, 0, len(results))
	for _, r := range results {
		switch {
		case r.History.Recurrence >= minRecurrence:
		case r.Metrics != nil && r.Metrics.Growing:
		case r.Action != nil && r.Action.Type != "" && r.Action.Type != "none":
		default:
			continue
		}
		notifiable = append(notifiable, r)
	}
	return notifiable
}

// slackReport monta a mensagem enviada ao Slack. Só relata — nunca sugere
// que algo foi ou será executado sozinho; toda ação listada vem com o
// comando exato que um humano precisa rodar pra aprovar.
func slackReport(namespace string, results []result) string {
	var b strings.Builder
	fmt.Fprintf(&b, ":mag: *k8s-mentor* encontrou *%d* incidente(s) em `%s`\n\n", len(results), namespace)
	for _, r := range results {
		fmt.Fprintf(&b, "• *%s/%s*\n", r.Event.Kind, r.Event.Name)
		fmt.Fprintf(&b, "  %s\n", r.Event.Message)
		if r.History.Recurrence > 0 {
			fmt.Fprintf(&b, "  _recorrência: %dx nos últimos registros_\n", r.History.Recurrence)
		}
		if len(r.Runbooks) > 0 {
			fmt.Fprintf(&b, "  :bulb: runbook: `%s`\n", r.Runbooks[0].Path)
		}
		if r.Action != nil && r.Action.Type != "" && r.Action.Type != "none" {
			fmt.Fprintf(&b, "  :warning: *ação sugerida: %s* — %s\n", r.Action.Type, r.Action.Reason)
			fmt.Fprintf(&b, "  aprove com: ```k8s-mentor apply-action --input <arquivo-com-a-sugestao> --yes```\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("_Nada foi executado automaticamente — nenhuma ação acontece sem aprovação manual explícita._\n\n")
	b.WriteString("```\n")
	for _, line := range summarizeResults(results) {
		b.WriteString(line + "\n")
	}
	b.WriteString("```")
	return b.String()
}

// extractJSONObject devolve só a partir do primeiro "{" em data. O k8sgpt
// escreve avisos de "resource is forbidden" (RBAC negado) direto no STDOUT
// antes do JSON, não no stderr — descoberto rodando `watch` de verdade
// contra uma RBAC propositalmente escopada ao namespace (agent.rbac em
// values.yaml): analyzers que tentam listar recursos cluster-scoped (nodes,
// webhooks) sempre geram esse aviso, e sem esse corte o json.Unmarshal falha
// com "invalid character 'o' in literal null" — o parser vendo "nodes..."
// e tentando casar contra o literal "null". k8sgpt com -o json sempre fecha
// num objeto, nunca um array, então cortar no primeiro "{" é seguro.
func extractJSONObject(data []byte) ([]byte, error) {
	i := bytes.IndexByte(data, '{')
	if i < 0 {
		return nil, fmt.Errorf("nenhum JSON encontrado na saída (%d bytes)", len(data))
	}
	return data[i:], nil
}

func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	address := flags.String("listen", ":8080", "endereço HTTP do agente")
	if err := flags.Parse(args); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})
	log.Printf("servidor do k8s-mentor escutando em %s", *address)
	return http.ListenAndServe(*address, mux)
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
func environment(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func environmentInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
func usage() {
	fmt.Fprintln(os.Stderr, "Uso:\n  k8s-mentor analyze --input diagnosticos.json --with-history --with-runbooks --with-gitops --with-metrics --with-action\n  k8sgpt analyze -o json | k8s-mentor analyze --with-history\n  k8s-mentor ingest-runbooks --dir demo/runbooks\n  k8s-mentor apply-action --input sugestao.json --yes\n  k8s-mentor watch --namespace demo   # roda o k8sgpt sozinho + os pilares; usado pelo CronJob\n  k8s-mentor serve")
}
