package store

import (
	"database/sql"
	"errors"
	"time"

	"chronosmonitor/internal/db"
	"chronosmonitor/internal/models"
)

var ErrNotFound = errors.New("task run not found")

type TaskStore struct {
	db *db.Conn
}

func NewTaskStore(conn *db.Conn) *TaskStore {
	return &TaskStore{db: conn}
}

func (s *TaskStore) Create(t models.TaskRun) error {
	_, err := s.db.Exec(
		`INSERT INTO task_runs (run_id, task_name, source, status, started_at, last_heartbeat_at, ttl_seconds)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.RunID, t.TaskName, t.Source, t.Status, t.StartedAt, t.StartedAt, t.TTLSeconds,
	)
	return err
}

func (s *TaskStore) Heartbeat(runID string, at time.Time) error {
	res, err := s.db.Exec(
		`UPDATE task_runs SET last_heartbeat_at = ? WHERE run_id = ? AND status = ?`,
		at, runID, models.StatusRunning,
	)
	return checkRowsAffected(res, err)
}

func (s *TaskStore) Finish(runID string, status models.TaskStatus, errMsg string, finishedAt time.Time) error {
	var run models.TaskRun
	if err := s.db.QueryRow(`SELECT started_at FROM task_runs WHERE run_id = ?`, runID).Scan(&run.StartedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	durationMs := finishedAt.Sub(run.StartedAt).Milliseconds()

	res, err := s.db.Exec(
		`UPDATE task_runs SET status = ?, finished_at = ?, duration_ms = ?, error_message = ? WHERE run_id = ?`,
		status, finishedAt, durationMs, errMsg, runID,
	)
	return checkRowsAffected(res, err)
}

// SweepTimeouts marks any "running" task whose last heartbeat (or start, if no
// heartbeat was ever received) is older than its declared TTL as "timeout".
// It returns the tasks that were transitioned.
//
// The comparison is done in Go rather than via SQL date functions because the
// sqlite driver round-trips time.Time through its own internal string format,
// which SQLite's julianday()/datetime() cannot parse.
func (s *TaskStore) SweepTimeouts(now time.Time) ([]models.TaskRun, error) {
	rows, err := s.db.Query(
		`SELECT run_id, started_at, last_heartbeat_at, ttl_seconds FROM task_runs
		 WHERE status = ? AND ttl_seconds IS NOT NULL`,
		models.StatusRunning,
	)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		runID     string
		reference time.Time
		ttl       int64
	}
	var candidates []candidate
	for rows.Next() {
		var runID string
		var startedAt time.Time
		var lastHeartbeatAt sql.NullTime
		var ttlSeconds int64
		if err := rows.Scan(&runID, &startedAt, &lastHeartbeatAt, &ttlSeconds); err != nil {
			rows.Close()
			return nil, err
		}
		reference := startedAt
		if lastHeartbeatAt.Valid {
			reference = lastHeartbeatAt.Time
		}
		candidates = append(candidates, candidate{runID: runID, reference: reference, ttl: ttlSeconds})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var timedOut []models.TaskRun
	for _, c := range candidates {
		if now.Sub(c.reference) <= time.Duration(c.ttl)*time.Second {
			continue
		}
		res, err := s.db.Exec(
			`UPDATE task_runs
			 SET status = ?, finished_at = ?, duration_ms = ?, error_message = ?
			 WHERE run_id = ? AND status = ?`,
			models.StatusTimeout, now, now.Sub(c.reference).Milliseconds(),
			"task exceeded TTL without heartbeat or completion", c.runID, models.StatusRunning,
		)
		if err != nil {
			return timedOut, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		task, err := s.Get(c.runID)
		if err != nil {
			return timedOut, err
		}
		timedOut = append(timedOut, *task)
	}
	return timedOut, nil
}

func (s *TaskStore) Get(runID string) (*models.TaskRun, error) {
	row := s.db.QueryRow(
		`SELECT run_id, task_name, source, status, started_at, finished_at, last_heartbeat_at, ttl_seconds, duration_ms, error_message
		 FROM task_runs WHERE run_id = ?`, runID,
	)
	return scanTaskRun(row)
}

func (s *TaskStore) List(status string) ([]models.TaskRun, error) {
	query := `SELECT run_id, task_name, source, status, started_at, finished_at, last_heartbeat_at, ttl_seconds, duration_ms, error_message
	          FROM task_runs`
	args := []any{}
	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY started_at DESC LIMIT 200`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.TaskRun
	for rows.Next() {
		t, err := scanTaskRun(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *t)
	}
	return results, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTaskRun(row scanner) (*models.TaskRun, error) {
	var t models.TaskRun
	var source, errMsg sql.NullString
	var finishedAt, lastHeartbeatAt sql.NullTime
	var ttlSeconds, durationMs sql.NullInt64

	err := row.Scan(&t.RunID, &t.TaskName, &source, &t.Status, &t.StartedAt,
		&finishedAt, &lastHeartbeatAt, &ttlSeconds, &durationMs, &errMsg)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	t.Source = source.String
	t.ErrorMessage = errMsg.String
	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}
	if lastHeartbeatAt.Valid {
		t.LastHeartbeatAt = &lastHeartbeatAt.Time
	}
	if ttlSeconds.Valid {
		t.TTLSeconds = &ttlSeconds.Int64
	}
	if durationMs.Valid {
		t.DurationMs = &durationMs.Int64
	}

	return &t, nil
}

func checkRowsAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
