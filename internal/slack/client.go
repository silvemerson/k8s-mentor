// Package slack envia notificações via Incoming Webhook do Slack — a
// integração mais simples que existe (uma URL, sem OAuth/bot token). Usado
// pelo subcomando `watch` (Fase 6) só pra AVISAR o que encontrou e o que
// sugere; nunca decide nem executa nada — isso continua exclusivo do
// `apply-action --yes`, rodado por um humano depois de ler a mensagem.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	webhookURL string
	http       *http.Client
}

func New(webhookURL string) *Client {
	return &Client{webhookURL: webhookURL, http: &http.Client{Timeout: 10 * time.Second}}
}

// Notify manda uma mensagem de texto (formatação mrkdwn do Slack: *negrito*,
// _itálico_, `código`, ```bloco```) pro canal configurado no Incoming Webhook.
func (c *Client) Notify(ctx context.Context, text string) error {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("montar payload do Slack: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("criar requisição pro Slack: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("chamar Slack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Slack respondeu status HTTP %d", resp.StatusCode)
	}
	return nil
}
