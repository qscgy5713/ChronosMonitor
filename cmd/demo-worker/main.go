// demo-worker is a fake Go job runner that reports its (made-up) task
// lifecycle to a running ChronosMonitor server over plain HTTP, so you can
// see the dashboard update live without wiring up a real cron job.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var baseURL = getEnv("CHRONOS_BASE_URL", "http://localhost:8080")

var taskNames = []string{"nightly-backup", "invoice-sync", "email-digest", "report-export"}

func main() {
	log.Printf("demo-worker reporting to %s (Ctrl+C to stop)", baseURL)
	for {
		runDemoTask()
		time.Sleep(3 * time.Second)
	}
}

func runDemoTask() {
	name := taskNames[rand.Intn(len(taskNames))]

	runID, err := start(name)
	if err != nil {
		log.Printf("[%s] start failed: %v", name, err)
		return
	}
	log.Printf("[%s] started run=%s", name, runID)

	switch rand.Intn(10) {
	case 0: // hangs forever with a short TTL -> demonstrates the timeout sweep
		log.Printf("[%s] simulating a stuck task with a 3s TTL (watch it time out)", name)
	case 1, 2: // fails
		time.Sleep(400 * time.Millisecond)
		msg := "Traceback (most recent call last):\n  connection refused to db:5432"
		if err := fail(runID, msg); err != nil {
			log.Printf("[%s] report failed() error: %v", name, err)
			return
		}
		log.Printf("[%s] reported failed run=%s", name, runID)
	default: // succeeds, with a couple heartbeats along the way
		for i := 0; i < 2; i++ {
			time.Sleep(300 * time.Millisecond)
			if err := heartbeat(runID); err != nil {
				log.Printf("[%s] heartbeat error: %v", name, err)
			}
		}
		time.Sleep(300 * time.Millisecond)
		if err := succeed(runID); err != nil {
			log.Printf("[%s] report success() error: %v", name, err)
			return
		}
		log.Printf("[%s] reported success run=%s", name, runID)
	}
}

func start(taskName string) (string, error) {
	body := map[string]any{"task_name": taskName, "source": "demo-go-worker"}
	if rand.Intn(10) == 0 {
		body["ttl_seconds"] = 3
	}

	var resp struct {
		RunID string `json:"run_id"`
	}
	if err := postJSON("/api/v1/events/start", body, &resp); err != nil {
		return "", err
	}
	return resp.RunID, nil
}

func heartbeat(runID string) error {
	return postJSON("/api/v1/events/heartbeat", map[string]any{"run_id": runID}, nil)
}

func succeed(runID string) error {
	return postJSON("/api/v1/events/success", map[string]any{"run_id": runID}, nil)
}

func fail(runID, errMsg string) error {
	return postJSON("/api/v1/events/failed", map[string]any{"run_id": runID, "error_message": errMsg}, nil)
}

func postJSON(path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s: unexpected status %d", path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
