package models

import "time"

type ScheduleStatus string

const (
	ScheduleStatusOK     ScheduleStatus = "ok"
	ScheduleStatusMissed ScheduleStatus = "missed"
)

// Schedule is a registered expectation that a task_name should report a
// "start" on some cadence. It's keyed by task_name, not run_id — it doesn't
// track individual runs, just whether *any* run has shown up recently
// enough.
//
// Exactly one of ExpectedIntervalSeconds or CronExpression is set:
//   - ExpectedIntervalSeconds: "at least once every N seconds" — simple,
//     good for roughly-periodic tasks.
//   - CronExpression: a standard 5-field cron spec (e.g. "0 12 * * *") —
//     correctly handles non-uniform schedules (weekdays only, monthly,
//     etc.) that a fixed interval would misjudge.
type Schedule struct {
	TaskName                string         `json:"task_name"`
	ExpectedIntervalSeconds *int64         `json:"expected_interval_seconds,omitempty"`
	CronExpression          *string        `json:"cron_expression,omitempty"`
	GracePeriodSeconds      int64          `json:"grace_period_seconds"`
	LastSeenAt              *time.Time     `json:"last_seen_at,omitempty"`
	Status                  ScheduleStatus `json:"status"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

type RegisterScheduleRequest struct {
	TaskName                string  `json:"task_name" binding:"required"`
	ExpectedIntervalSeconds *int64  `json:"expected_interval_seconds"`
	CronExpression          *string `json:"cron_expression"`
	GracePeriodSeconds      int64   `json:"grace_period_seconds"`
}
