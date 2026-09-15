package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"chronosmonitor/internal/broker"
	"chronosmonitor/internal/config"
	"chronosmonitor/internal/db"
	"chronosmonitor/internal/models"
	"chronosmonitor/internal/store"
)

func newTestSetup(t *testing.T) (*gin.Engine, *broker.Hub, *store.ScheduleStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	conn, err := db.Open(config.Config{
		DBDriver: "sqlite",
		DBPath:   filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	hub := broker.New()
	scheduleStore := store.NewScheduleStore(conn)
	h := NewTaskHandler(store.NewTaskStore(conn), scheduleStore, hub)
	sh := NewScheduleHandler(scheduleStore)

	r := gin.New()
	r.POST("/start", h.Start)
	r.POST("/heartbeat", h.Heartbeat)
	r.POST("/success", h.Success)
	r.POST("/failed", h.Failed)
	r.GET("/tasks", h.List)
	r.GET("/tasks/:runID", h.Get)
	r.POST("/schedules", sh.Register)
	r.GET("/schedules", sh.List)
	r.DELETE("/schedules/:taskName", sh.Delete)

	return r, hub, scheduleStore
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func expectEvent(t *testing.T, ch <-chan broker.Event, wantType string) broker.Event {
	t.Helper()
	select {
	case evt := <-ch:
		if evt.Type != wantType {
			t.Fatalf("received event type %q, want %q", evt.Type, wantType)
		}
		return evt
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q event", wantType)
		return broker.Event{}
	}
}

func TestStart_ReturnsRunIDAndPublishesEvent(t *testing.T) {
	r, hub, _ := newTestSetup(t)
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{
		TaskName: "daily-report",
		Source:   "laravel-cron",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.RunID == "" {
		t.Fatal("expected a generated run_id, got empty string")
	}

	evt := expectEvent(t, ch, broker.EventStarted)
	if evt.Task.RunID != resp.RunID || evt.Task.Status != models.StatusRunning {
		t.Errorf("published event = %+v, want RunID=%s Status=running", evt.Task, resp.RunID)
	}
}

func TestStart_UsesClientSuppliedRunID(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{
		TaskName: "custom-id-task",
		RunID:    "my-custom-id",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.RunID != "my-custom-id" {
		t.Errorf("run_id = %q, want the client-supplied id", resp.RunID)
	}
}

func TestStart_MissingTaskNameReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/start", map[string]string{"source": "x"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHeartbeat_UnknownRunIDReturns404(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/heartbeat", models.HeartbeatRequest{RunID: "nope"})
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFullLifecycle_StartHeartbeatSuccess(t *testing.T) {
	r, hub, _ := newTestSetup(t)
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "sync-job"})
	var started struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &started)
	expectEvent(t, ch, broker.EventStarted)

	w = doJSON(t, r, http.MethodPost, "/heartbeat", models.HeartbeatRequest{RunID: started.RunID})
	if w.Code != http.StatusNoContent {
		t.Fatalf("heartbeat status = %d, want %d", w.Code, http.StatusNoContent)
	}
	expectEvent(t, ch, broker.EventHeartbeat)

	w = doJSON(t, r, http.MethodPost, "/success", models.SuccessRequest{RunID: started.RunID})
	if w.Code != http.StatusNoContent {
		t.Fatalf("success status = %d, want %d", w.Code, http.StatusNoContent)
	}
	evt := expectEvent(t, ch, broker.EventSucceeded)
	if evt.Task.Status != models.StatusSuccess {
		t.Errorf("published task status = %q, want success", evt.Task.Status)
	}

	w = doJSON(t, r, http.MethodGet, "/tasks/"+started.RunID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", w.Code, http.StatusOK)
	}
	var task models.TaskRun
	json.Unmarshal(w.Body.Bytes(), &task)
	if task.Status != models.StatusSuccess {
		t.Errorf("final task status = %q, want success", task.Status)
	}
	if task.DurationMs == nil {
		t.Error("expected duration_ms to be set after success")
	}
}

func TestFailed_StoresAndPublishesErrorMessage(t *testing.T) {
	r, hub, _ := newTestSetup(t)
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "invoice-sync"})
	var started struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &started)
	expectEvent(t, ch, broker.EventStarted)

	w = doJSON(t, r, http.MethodPost, "/failed", models.FailedRequest{
		RunID:        started.RunID,
		ErrorMessage: "connection refused",
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("failed status = %d, want %d", w.Code, http.StatusNoContent)
	}

	evt := expectEvent(t, ch, broker.EventFailed)
	if evt.Task.ErrorMessage != "connection refused" {
		t.Errorf("published error_message = %q, want %q", evt.Task.ErrorMessage, "connection refused")
	}

	w = doJSON(t, r, http.MethodGet, "/tasks/"+started.RunID, nil)
	var task models.TaskRun
	json.Unmarshal(w.Body.Bytes(), &task)
	if task.Status != models.StatusFailed || task.ErrorMessage != "connection refused" {
		t.Errorf("task = %+v, want status=failed error_message=%q", task, "connection refused")
	}
}

func TestList_FiltersByStatus(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "a"})
	var a struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &a)
	doJSON(t, r, http.MethodPost, "/failed", models.FailedRequest{RunID: a.RunID, ErrorMessage: "x"})

	doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "b"})

	w = doJSON(t, r, http.MethodGet, "/tasks?status=failed", nil)
	var resp struct {
		Tasks []models.TaskRun `json:"tasks"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Tasks) != 1 || resp.Tasks[0].TaskName != "a" {
		t.Fatalf("filtered list = %+v, want only task 'a'", resp.Tasks)
	}
}

func TestStart_TouchesRegisteredScheduleAndClearsMissedStatus(t *testing.T) {
	r, _, scheduleStore := newTestSetup(t)

	if err := scheduleStore.Upsert("daily-report", 60, 0, time.Now().UTC()); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	// Simulate it having already been flagged missed before this run showed up.
	if _, err := scheduleStore.DetectMissed(time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("DetectMissed() error = %v", err)
	}

	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "daily-report"})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
	}

	sched, err := scheduleStore.Get("daily-report")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if sched.Status != models.ScheduleStatusOK {
		t.Errorf("schedule status = %q, want ok after a fresh start", sched.Status)
	}
	if sched.LastSeenAt == nil {
		t.Error("schedule LastSeenAt is nil, want it set by the start report")
	}
}

func TestStart_UnregisteredTaskNameDoesNotFail(t *testing.T) {
	r, _, _ := newTestSetup(t)

	// "unregistered-task" has no schedule; Start() must still succeed.
	w := doJSON(t, r, http.MethodPost, "/start", models.StartRequest{TaskName: "unregistered-task"})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}
