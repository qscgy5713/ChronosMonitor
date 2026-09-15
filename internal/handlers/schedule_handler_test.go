package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"chronosmonitor/internal/models"
)

func int64ptr(v int64) *int64 { return &v }
func strptr(s string) *string { return &s }

func TestRegisterSchedule_IntervalMode_Success(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: int64ptr(86400),
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
	sched := resp.Schedules[0]
	if sched.ExpectedIntervalSeconds == nil || *sched.ExpectedIntervalSeconds != 86400 || sched.GracePeriodSeconds != 300 {
		t.Errorf("schedule = %+v, want interval=86400 grace=300", sched)
	}
	if sched.CronExpression != nil {
		t.Errorf("CronExpression = %v, want nil for an interval-mode schedule", sched.CronExpression)
	}
}

func TestRegisterSchedule_CronMode_Success(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:           "weekday-report",
		CronExpression:     strptr("0 9 * * 1-5"),
		GracePeriodSeconds: 600,
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}

	w = doJSON(t, r, http.MethodGet, "/schedules", nil)
	var resp struct {
		Schedules []models.Schedule `json:"schedules"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Schedules) != 1 {
		t.Fatalf("schedules = %+v, want 1 entry", resp.Schedules)
	}
	sched := resp.Schedules[0]
	if sched.CronExpression == nil || *sched.CronExpression != "0 9 * * 1-5" {
		t.Errorf("CronExpression = %v, want \"0 9 * * 1-5\"", sched.CronExpression)
	}
	if sched.ExpectedIntervalSeconds != nil {
		t.Errorf("ExpectedIntervalSeconds = %v, want nil for a cron-mode schedule", sched.ExpectedIntervalSeconds)
	}
}

func TestRegisterSchedule_InvalidCronExpressionReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:       "daily-report",
		CronExpression: strptr("not a cron expression"),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for an invalid cron expression", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_MissingTaskNameReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", map[string]int64{"expected_interval_seconds": 60})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_NeitherIntervalNorCronReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName: "daily-report",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d when neither interval nor cron is given", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_BothIntervalAndCronReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: int64ptr(60),
		CronExpression:          strptr("0 9 * * *"),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d when both interval and cron are given", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_NonPositiveIntervalReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: int64ptr(0),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a zero interval", w.Code, http.StatusBadRequest)
	}
}

func TestRegisterSchedule_NegativeGracePeriodReturns400(t *testing.T) {
	r, _, _ := newTestSetup(t)

	w := doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName:                "daily-report",
		ExpectedIntervalSeconds: int64ptr(60),
		GracePeriodSeconds:      -1,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d for a negative grace period", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteSchedule_Success(t *testing.T) {
	r, _, _ := newTestSetup(t)

	doJSON(t, r, http.MethodPost, "/schedules", models.RegisterScheduleRequest{
		TaskName: "daily-report", ExpectedIntervalSeconds: int64ptr(60),
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
