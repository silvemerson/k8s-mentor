// Package k8sgpt adapta a saída JSON do K8sGPT para o evento interno do agente.
package k8sgpt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Event é o contrato interno. Novas fontes devem produzir este formato, sem
// acoplar a memória aos detalhes de JSON de cada ferramenta.
type Event struct {
	Source      string    `json:"source"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace,omitempty"`
	Message     string    `json:"message"`
	Explanation string    `json:"explanation,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Parse aceita o formato de analyze -o json com error como string ou lista.
// A tolerância intencional evita quebrar o agente por pequenas variações entre
// versões do K8sGPT.
func Parse(data []byte, now time.Time) ([]Event, error) {
	documents, err := extractDocuments(data)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, document := range documents {
		kind := stringField(document, "kind")
		namespace := stringField(document, "namespace")
		name := stringField(document, "name")
		// O K8sGPT de verdade não manda "namespace" separado — vem embutido
		// em "name" como "namespace/nome" (ex: "demo/checkout-service").
		// Sem separar isso, Namespace fica sempre vazio (quebra --namespace
		// e a recorrência por namespace em agent/memory) e Name carrega o
		// prefixo errado (quebra qualquer chamada à API do Kubernetes por
		// esse nome, ex: agent/correlation/gitops).
		if namespace == "" {
			if idx := strings.Index(name, "/"); idx >= 0 {
				namespace, name = name[:idx], name[idx+1:]
			}
		}
		// No K8sGPT de verdade, "details" (a explicação gerada com --explain)
		// fica no nível do recurso, irmã de "error" — não dentro de cada
		// mensagem de erro. Lida com os dois formatos: prioriza o nível do
		// diagnóstico (outras fontes podem colocar ali) e cai pro nível do
		// documento quando ausente.
		documentExplanation := stringField(document, "explanation", "details")
		for _, diagnostic := range diagnostics(document) {
			message := stringField(diagnostic, "text", "message", "error")
			explanation := stringField(diagnostic, "explanation", "details")
			if explanation == "" {
				explanation = documentExplanation
			}
			if message == "" {
				continue
			}
			events = append(events, Event{Source: "k8sgpt", Kind: kind, Name: name, Namespace: namespace, Message: message, Explanation: explanation, Timestamp: now.UTC()})
		}
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("nenhum diagnóstico encontrado na saída do K8sGPT")
	}
	return events, nil
}

// extractDocuments normaliza os três formatos aceitos: array solto de
// recursos, um único recurso, ou o formato real do `k8sgpt analyze -o json`
// — um objeto com "provider"/"status"/"problems" no nível raiz e os
// diagnósticos por recurso dentro de "results".
func extractDocuments(data []byte) ([]map[string]any, error) {
	var array []map[string]any
	if err := json.Unmarshal(data, &array); err == nil {
		return array, nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("JSON do K8sGPT inválido: %w", err)
	}
	if results, ok := field(object, "results"); ok {
		list, ok := results.([]any)
		if !ok {
			return nil, nil
		}
		documents := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if document, ok := item.(map[string]any); ok {
				documents = append(documents, document)
			}
		}
		return documents, nil
	}
	return []map[string]any{object}, nil
}

func diagnostics(document map[string]any) []map[string]any {
	for _, key := range []string{"error", "errors", "diagnostics"} {
		value, ok := field(document, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			return []map[string]any{{"text": typed}}
		case map[string]any:
			return []map[string]any{typed}
		case []any:
			items := make([]map[string]any, 0, len(typed))
			for _, item := range typed {
				if diagnostic, ok := item.(map[string]any); ok {
					items = append(items, diagnostic)
				}
				if text, ok := item.(string); ok {
					items = append(items, map[string]any{"text": text})
				}
			}
			return items
		}
	}
	return nil
}

func stringField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := field(values, key); ok {
			value, ok := raw.(string)
			if !ok {
				continue
			}
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func field(values map[string]any, wanted string) (any, bool) {
	for key, value := range values {
		if strings.EqualFold(key, wanted) {
			return value, true
		}
	}
	return nil, false
}
