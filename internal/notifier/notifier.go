// Package notifier sends alerts to an external webhook (Slack, Discord, or a
// generic JSON receiver) when a task fails or times out.
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chronosmonitor/internal/models"
)

// Notifier delivers a single alert for a task run.
type Notifier interface {
	Notify(task models.TaskRun) error
}

// Format shapes the webhook's POST body.
type Format string

const (
	FormatSlack   Format = "slack"
	FormatDiscord Format = "discord"
	FormatGeneric Format = "generic"
)

const maxErrorMessageLen = 500

// WebhookNotifier POSTs a formatted alert to a single webhook URL.
type WebhookNotifier struct {
	url    string
	format Format
	client *http.Client
}

func NewWebhookNotifier(url string, format Format) *WebhookNotifier {
	return &WebhookNotifier{
		url:    url,
		format: format,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *WebhookNotifier) Notify(task models.TaskRun) error {
	payload, err := n.buildPayload(task)
	if err != nil {
		return fmt.Errorf("build webhook payload: %w", err)
	}

	resp, err := n.client.Post(n.url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (n *WebhookNotifier) buildPayload(task models.TaskRun) ([]byte, error) {
	switch n.format {
	case FormatDiscord:
		return json.Marshal(map[string]string{"content": formatMessage(task)})
	case FormatGeneric:
		return json.Marshal(task)
	default: // Slack, and every Slack-compatible receiver (Mattermost, etc.)
		return json.Marshal(map[string]string{"text": formatMessage(task)})
	}
}

func formatMessage(task models.TaskRun) string {
	icon, verb := "⚠️", string(task.Status)
	switch task.Status {
	case models.StatusFailed:
		icon, verb = "🔴", "failed"
	case models.StatusTimeout:
		icon, verb = "⏰", "timed out"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s *%s* %s (run `%s`)", icon, task.TaskName, verb, task.RunID)
	if task.Source != "" {
		fmt.Fprintf(&b, " from `%s`", task.Source)
	}
	if task.DurationMs != nil {
		fmt.Fprintf(&b, "\nDuration: %dms", *task.DurationMs)
	}
	if task.ErrorMessage != "" {
		fmt.Fprintf(&b, "\n```%s```", truncate(task.ErrorMessage, maxErrorMessageLen))
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... (truncated)"
}
