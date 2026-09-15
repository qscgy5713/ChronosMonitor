package models

import "time"

type ScheduleStatus string

const (
	ScheduleStatusOK     ScheduleStatus = "ok"
	ScheduleStatusMissed ScheduleStatus = "missed"
)

// Schedule is a registered expectation that a task_name should report a
// "start" at least once every ExpectedIntervalSeconds (plus a grace buffer).
// It's keyed by task_name, not run_id — it doesn't track individual runs,
// just whether *any* run has shown up recently enough.
type Schedule struct {
	TaskName                string         `json:"task_name"`
	ExpectedIntervalSeconds int64          `json:"expected_interval_seconds"`
	GracePeriodSeconds      int64          `json:"grace_period_seconds"`
	LastSeenAt              *time.Time     `json:"last_seen_at,omitempty"`
	Status                  ScheduleStatus `json:"status"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

type RegisterScheduleRequest struct {
	TaskName                string `json:"task_name" binding:"required"`
	ExpectedIntervalSeconds int64  `json:"expected_interval_seconds" binding:"required"`
	GracePeriodSeconds      int64  `json:"grace_period_seconds"`
}
