package main

import (
	"testing"

	"github.com/silvemerson/k8s-mentor/agent/correlation/metrics"
	"github.com/silvemerson/k8s-mentor/agent/decision"
	"github.com/silvemerson/k8s-mentor/agent/memory"
	"github.com/silvemerson/k8s-mentor/internal/k8sgpt"
)

// TestExtractJSONObject cobre o bug real achado testando `watch` contra o
// cluster: o k8sgpt escreve avisos de RBAC negado no stdout, antes do JSON,
// quando a RBAC é escopada ao namespace (não vê recursos cluster-scoped
// como nodes/webhooks). Sem esse corte, json.Unmarshal falhava com
// "invalid character 'o' in literal null" — o parser via "nodes..." e
// tentava casar contra o literal "null".
func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "sem aviso nenhum",
			input: `{"status":"ok"}`,
			want:  `{"status":"ok"}`,
		},
		{
			name: "avisos de RBAC negado antes do JSON (caso real)",
			input: "nodes is forbidden: User \"x\" cannot list resource \"nodes\" at the cluster scope\n" +
				"mutatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: ...\n" +
				`{"provider":"ollama","status":"ProblemDetected"}`,
			want: `{"provider":"ollama","status":"ProblemDetected"}`,
		},
		{
			name:    "sem JSON nenhum",
			input:   "só texto, nada de JSON aqui",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractJSONObject([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("esperava erro, não teve")
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFilterNotifiable cobre o filtro de ruído do Slack na Fase 6: um
// evento isolado (sem recorrência, sem tendência, sem ação sugerida) não
// deve virar notificação — mas continua no log/JSON, isso aqui só decide o
// que vale interromper alguém no Slack.
func TestFilterNotifiable(t *testing.T) {
	event := func(name string) k8sgpt.Event { return k8sgpt.Event{Kind: "Pod", Name: name} }

	isolado := result{Event: event("isolado")}
	recorrente := result{Event: event("recorrente"), History: memory.History{Recurrence: 3}}
	crescendo := result{Event: event("crescendo"), Metrics: &metrics.Result{Growing: true}}
	comAcao := result{Event: event("com-acao"), Action: &decision.Suggestion{Type: "rollback"}}
	acaoNone := result{Event: event("acao-none"), Action: &decision.Suggestion{Type: "none"}}

	all := []result{isolado, recorrente, crescendo, comAcao, acaoNone}
	got := filterNotifiable(all, 2)

	wantNames := map[string]bool{"recorrente": true, "crescendo": true, "com-acao": true}
	if len(got) != len(wantNames) {
		t.Fatalf("got %d incidentes notificáveis, want %d (%+v)", len(got), len(wantNames), got)
	}
	for _, r := range got {
		if !wantNames[r.Event.Name] {
			t.Errorf("%q não deveria ter passado no filtro", r.Event.Name)
		}
	}
}
