package store

import (
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"chronosmonitor/internal/db"
	"chronosmonitor/internal/models"
)

type ScheduleStore struct {
	db *db.Conn
}

func NewScheduleStore(conn *db.Conn) *ScheduleStore {
	return &ScheduleStore{db: conn}
}

// Upsert registers a task_name's schedule expectation, or updates an
// existing one's expectation/grace period. Exactly one of
// expectedIntervalSeconds or cronExpression should be non-nil; callers
// (the HTTP handler) are responsible for validating that and for validating
// a cron expression parses, so this layer doesn't have to reject bad input.
// It never resets last_seen_at or status — re-registering a schedule
// doesn't erase what's already known about whether the task has been
// showing up.
func (s *ScheduleStore) Upsert(taskName string, expectedIntervalSeconds *int64, cronExpression *string, gracePeriodSeconds int64, now time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO task_schedules (task_name, expected_interval_seconds, cron_expression, grace_period_seconds, status, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(task_name) DO UPDATE SET
		   expected_interval_seconds = excluded.expected_interval_seconds,
		   cron_expression = excluded.cron_expression,
		   grace_period_seconds = excluded.grace_period_seconds,
		   updated_at = excluded.updated_at`,
		taskName, expectedIntervalSeconds, cronExpression, gracePeriodSeconds, models.ScheduleStatusOK, now,
	)
	return err
}

// Touch records that a run started for taskName: it resets the schedule to
// "ok" and bumps last_seen_at. It's a no-op (not an error) if no schedule is
// registered for this task_name — most tasks won't have one.
func (s *ScheduleStore) Touch(taskName string, at time.Time) error {
	_, err := s.db.Exec(
		`UPDATE task_schedules SET last_seen_at = ?, status = ?, updated_at = ? WHERE task_name = ?`,
		at, models.ScheduleStatusOK, at, taskName,
	)
	return err
}

func (s *ScheduleStore) Get(taskName string) (*models.Schedule, error) {
	row := s.db.QueryRow(
		`SELECT task_name, expected_interval_seconds, cron_expression, grace_period_seconds, last_seen_at, status, updated_at
		 FROM task_schedules WHERE task_name = ?`,
		taskName,
	)
	return scanSchedule(row)
}

func (s *ScheduleStore) List() ([]models.Schedule, error) {
	rows, err := s.db.Query(
		`SELECT task_name, expected_interval_seconds, cron_expression, grace_period_seconds, last_seen_at, status, updated_at
		 FROM task_schedules ORDER BY task_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Schedule
	for rows.Next() {
		sched, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *sched)
	}
	return results, rows.Err()
}

func (s *ScheduleStore) Delete(taskName string) error {
	res, err := s.db.Exec(`DELETE FROM task_schedules WHERE task_name = ?`, taskName)
	return checkRowsAffected(res, err)
}

// DetectMissed finds schedules currently "ok" that are overdue, and flips
// them to "missed". Only newly-flagged schedules are returned — one already
// at "missed" isn't returned again on a later call, so callers don't have to
// de-duplicate repeat alerts for the same missed episode themselves.
//
// "Overdue" is computed from each schedule's last-seen time (or registration
// time, if never seen at all):
//   - interval mode: reference + expected_interval_seconds + grace_period_seconds
//   - cron mode: the next cron occurrence after reference, + grace_period_seconds
//
// The comparison is done in Go rather than via SQL date functions for the
// same reason as TaskStore.SweepTimeouts: the sqlite driver round-trips
// time.Time through its own internal string format, which SQLite's
// julianday()/datetime() can't parse.
func (s *ScheduleStore) DetectMissed(now time.Time) ([]models.Schedule, error) {
	rows, err := s.db.Query(
		`SELECT task_name, expected_interval_seconds, cron_expression, grace_period_seconds, last_seen_at, updated_at
		 FROM task_schedules WHERE status = ?`,
		models.ScheduleStatusOK,
	)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		taskName string
		deadline time.Time
	}
	var candidates []candidate
	for rows.Next() {
		var taskName string
		var intervalSeconds sql.NullInt64
		var cronExpr sql.NullString
		var graceSeconds int64
		var lastSeenAt sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&taskName, &intervalSeconds, &cronExpr, &graceSeconds, &lastSeenAt, &updatedAt); err != nil {
			rows.Close()
			return nil, err
		}

		reference := updatedAt // registered but never seen: count from registration time
		if lastSeenAt.Valid {
			reference = lastSeenAt.Time
		}
		grace := time.Duration(graceSeconds) * time.Second

		var deadline time.Time
		if cronExpr.Valid {
			sched, err := cron.ParseStandard(cronExpr.String)
			if err != nil {
				// Was validated at registration time, so this shouldn't
				// happen — but don't let one bad row break the whole sweep.
				log.Printf("missed-run: skipping %q, invalid stored cron_expression %q: %v", taskName, cronExpr.String, err)
				continue
			}
			deadline = sched.Next(reference).Add(grace)
		} else {
			deadline = reference.Add(time.Duration(intervalSeconds.Int64)*time.Second + grace)
		}

		candidates = append(candidates, candidate{taskName: taskName, deadline: deadline})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var missed []models.Schedule
	for _, c := range candidates {
		if now.Before(c.deadline) {
			continue
		}
		res, err := s.db.Exec(
			`UPDATE task_schedules SET status = ?, updated_at = ? WHERE task_name = ? AND status = ?`,
			models.ScheduleStatusMissed, now, c.taskName, models.ScheduleStatusOK,
		)
		if err != nil {
			return missed, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		sched, err := s.Get(c.taskName)
		if err != nil {
			return missed, err
		}
		missed = append(missed, *sched)
	}
	return missed, nil
}

func scanSchedule(row scanner) (*models.Schedule, error) {
	var sched models.Schedule
	var intervalSeconds sql.NullInt64
	var cronExpr sql.NullString
	var lastSeenAt sql.NullTime

	err := row.Scan(&sched.TaskName, &intervalSeconds, &cronExpr, &sched.GracePeriodSeconds,
		&lastSeenAt, &sched.Status, &sched.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if intervalSeconds.Valid {
		sched.ExpectedIntervalSeconds = &intervalSeconds.Int64
	}
	if cronExpr.Valid {
		sched.CronExpression = &cronExpr.String
	}
	if lastSeenAt.Valid {
		sched.LastSeenAt = &lastSeenAt.Time
	}

	return &sched, nil
}
