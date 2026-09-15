package notifier

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chronosmonitor/internal/models"
)

func sampleFailedTask() models.TaskRun {
	durationMs := int64(132)
	return models.TaskRun{
		RunID:        "abc123",
		TaskName:     "invoice-sync",
		Source:       "laravel-cron",
		Status:       models.StatusFailed,
		DurationMs:   &durationMs,
		ErrorMessage: "connection refused to db:5432",
	}
}

func TestBuildPayload_Slack(t *testing.T) {
	n := NewWebhookNotifier("http://example.invalid", FormatSlack)

	payload, err := n.buildPayload(sampleFailedTask())
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	text, ok := body["text"]
	if !ok {
		t.Fatalf("payload = %s, want a \"text\" field", payload)
	}
	if !strings.Contains(text, "invoice-sync") || !strings.Contains(text, "connection refused") {
		t.Errorf("text = %q, want it to mention the task name and error", text)
	}
}

func TestBuildPayload_Discord(t *testing.T) {
	n := NewWebhookNotifier("http://example.invalid", FormatDiscord)

	payload, err := n.buildPayload(sampleFailedTask())
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}

	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := body["content"]; !ok {
		t.Fatalf("payload = %s, want a \"content\" field", payload)
	}
}

func TestBuildPayload_Generic(t *testing.T) {
	n := NewWebhookNotifier("http://example.invalid", FormatGeneric)

	payload, err := n.buildPayload(sampleFailedTask())
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}

	var task models.TaskRun
	if err := json.Unmarshal(payload, &task); err != nil {
		t.Fatalf("unmarshal payload as TaskRun: %v", err)
	}
	if task.RunID != "abc123" || task.TaskName != "invoice-sync" {
		t.Errorf("decoded task = %+v, want the original task fields preserved", task)
	}
}

func TestFormatMessage_TruncatesLongErrorMessages(t *testing.T) {
	task := sampleFailedTask()
	task.ErrorMessage = strings.Repeat("x", maxErrorMessageLen+100)

	msg := formatMessage(task)
	if strings.Contains(msg, strings.Repeat("x", maxErrorMessageLen+1)) {
		t.Error("formatMessage() did not truncate an oversized error message")
	}
	if !strings.Contains(msg, "truncated") {
		t.Error("formatMessage() should note that the message was truncated")
	}
}

func TestFormatMessage_TimeoutUsesDifferentWording(t *testing.T) {
	task := sampleFailedTask()
	task.Status = models.StatusTimeout
	task.ErrorMessage = ""

	msg := formatMessage(task)
	if !strings.Contains(msg, "timed out") {
		t.Errorf("message = %q, want it to mention \"timed out\" for a timeout task", msg)
	}
}

func TestFormatMessage_MissedScheduleHasNoRunIDAndDifferentWording(t *testing.T) {
	task := models.TaskRun{
		TaskName:     "invoice-sync",
		Status:       models.StatusMissed,
		ErrorMessage: "expected at least once every 3600s (+0s grace); last seen: never",
	}

	msg := formatMessage(task)
	if !strings.Contains(msg, "missed its expected run") {
		t.Errorf("message = %q, want it to mention a missed run", msg)
	}
	if strings.Contains(msg, "run ``") || strings.Contains(msg, "(run `)") {
		t.Errorf("message = %q, should not print an empty run id for a synthetic missed-run alert", msg)
	}
	if !strings.Contains(msg, "expected at least once every 3600s") {
		t.Errorf("message = %q, want the schedule detail in the error_message body", msg)
	}
}

func TestNotify_SendsToWebhookAndSucceedsOn2xx(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, FormatSlack)
	if err := n.Notify(sampleFailedTask()); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if !strings.Contains(string(received), "invoice-sync") {
		t.Errorf("webhook received %s, want it to contain the task name", received)
	}
}

func TestNotify_ReturnsErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL, FormatSlack)
	if err := n.Notify(sampleFailedTask()); err == nil {
		t.Fatal("Notify() error = nil, want an error for a 500 response")
	}
}
