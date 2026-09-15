package models

import "time"

type TaskStatus string

const (
	StatusRunning TaskStatus = "running"
	StatusSuccess TaskStatus = "success"
	StatusFailed  TaskStatus = "failed"
	StatusTimeout TaskStatus = "timeout"
)

type TaskRun struct {
	RunID           string     `json:"run_id"`
	TaskName        string     `json:"task_name"`
	Source          string     `json:"source,omitempty"`
	Status          TaskStatus `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	TTLSeconds      *int64     `json:"ttl_seconds,omitempty"`
	DurationMs      *int64     `json:"duration_ms,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

type StartRequest struct {
	TaskName   string `json:"task_name" binding:"required"`
	RunID      string `json:"run_id"`
	Source     string `json:"source"`
	TTLSeconds *int64 `json:"ttl_seconds"`
}

type HeartbeatRequest struct {
	RunID string `json:"run_id" binding:"required"`
}

type SuccessRequest struct {
	RunID string `json:"run_id" binding:"required"`
}

type FailedRequest struct {
	RunID        string `json:"run_id" binding:"required"`
	ErrorMessage string `json:"error_message"`
}
