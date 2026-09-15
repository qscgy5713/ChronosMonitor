package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"chronosmonitor/internal/models"
)

func TestRegisterSchedule_Success(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: 86400,
		GracePeriodSeconds:      300,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}

	w = doJSON(t, r, http.MethodGet, "/schedules", nil)
	var resp struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Schedules) != 1 || resp.Schedules[0].TaskName != "daily-report" {
		t.Fatalf("schedules = %+v, want [daily-report]", resp.Schedules)
	}
	if resp.Schedules[0].ExpectedIntervalSeconds != 86400 || resp.Schedules[0].GracePeriodSeconds != 300 {
		t.Errorf("schedule = %+v, want interval=86400 grace=300", resp.Schedules[0])
	}
}

func TestRegisterSchedule_MissingTaskNameReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", map[string]int64{"expected_interval_seconds": 60})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_NonPositiveIntervalReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: 0,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a zero interval", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_NegativeGracePeriodReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: 60,
		GracePeriodSeconds:      -1,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a negative grace period", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteSchedule_Success(t *testing.T) {
	r, _, _ := newTestSetup(t)

	doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName: "daily-report", ExpectedIntervalSeconds: 60,
	})

	w := doJSON(t, r, http.MethodDelete, "/schedules/daily-report", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	w = doJSON(t, r, http.MethodGet, "/schedules", nil)
	var resp struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Schedules) != 0 {
		t.Errorf("schedules after delete = %+v, want empty", resp.Schedules)
	}
}

func TestDeleteSchedule_UnknownReturns404(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodDelete, "/schedules/does-not-exist", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
