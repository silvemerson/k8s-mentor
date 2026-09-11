package k8sgpt

import (
	"testing"
	"time"
)

func TestParseK8sGPTDiagnostics(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	events, err := Parse([]byte(`[{"kind":"Pod","name":"api-123","namespace":"demo","error":[{"text":"OOMKilled","explanation":"limite excedido"}]}]`), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "OOMKilled" || events[0].Namespace != "demo" || events[0].Explanation != "limite excedido" {
		t.Fatalf("eventos inesperados: %#v", events)
	}
}

// TestParseRealK8sGPTShape usa o formato real de `k8sgpt analyze -o json
// --explain`: "details" (a explicação gerada) fica no nível do recurso,
// irmã de "error" — não dentro de cada mensagem de erro.
func TestParseRealK8sGPTShape(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	events, err := Parse([]byte(`[{
		"kind": "Pod",
		"name": "checkout-service-7d8f9c-abcde",
		"namespace": "demo",
		"error": [{"Text": "back-off restarting failed container"}],
		"details": "O container está reiniciando repetidamente por OOMKilled."
	}]`), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("eventos inesperados: %#v", events)
	}
	if events[0].Message != "back-off restarting failed container" {
		t.Fatalf("mensagem inesperada: %q", events[0].Message)
	}
	if events[0].Explanation != "O container está reiniciando repetidamente por OOMKilled." {
		t.Fatalf("explicação não propagada do nível do documento: %q", events[0].Explanation)
	}
}

// TestParseRealK8sGPTOutput usa uma saída real e completa de `k8sgpt analyze
// -o json --explain` (capturada rodando contra o cenário oomkilled-facil no
// cluster de demo): os diagnósticos ficam dentro de "results", não soltos
// num array no nível raiz, e "name" vem como "namespace/nome" — sem
// "namespace" separado. Pipar a saída de verdade do k8sgpt pro k8s-mentor
// (o fluxo documentado no README) falhava silenciosamente antes desse teste
// existir: "nenhum diagnóstico encontrado".
func TestParseRealK8sGPTOutput(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	events, err := Parse([]byte(`{
		"provider": "ollama",
		"errors": null,
		"status": "ProblemDetected",
		"problems": 1,
		"results": [
			{
				"kind": "Pod",
				"name": "demo/image-resizer-75f859f6c7-qwzqs",
				"error": [
					{"Text": "the termination reason is OOMKilled exitCode=137 container=image-resizer pod=image-resizer-75f859f6c7-qwzqs"}
				],
				"details": "The container was killed due to Out of Memory (OOM) issues, with exit code 137.",
				"parentObject": "Deployment/image-resizer"
			}
		]
	}`), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("eventos inesperados: %#v", events)
	}
	event := events[0]
	if event.Namespace != "demo" {
		t.Fatalf("namespace não extraído de \"namespace/nome\": %q", event.Namespace)
	}
	if event.Name != "image-resizer-75f859f6c7-qwzqs" {
		t.Fatalf("name deveria vir sem o prefixo de namespace: %q", event.Name)
	}
	if event.Kind != "Pod" {
		t.Fatalf("kind inesperado: %q", event.Kind)
	}
}

func TestParseStringError(t *testing.T) {
	events, err := Parse([]byte(`[{"kind":"Service","name":"web","error":"sem endpoints"}]`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Message != "sem endpoints" {
		t.Fatalf("eventos inesperados: %#v", events)
	}
}
