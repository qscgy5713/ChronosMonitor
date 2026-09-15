package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"chronosmonitor/internal/config"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS task_runs (
	run_id             TEXT PRIMARY KEY,
	task_name          TEXT NOT NULL,
	source             TEXT,
	status             TEXT NOT NULL,
	started_at         DATETIME NOT NULL,
	finished_at        DATETIME,
	last_heartbeat_at  DATETIME,
	ttl_seconds        INTEGER,
	duration_ms        INTEGER,
	error_message      TEXT
);
CREATE INDEX IF NOT EXISTS idx_task_runs_task_name ON task_runs(task_name);
CREATE INDEX IF NOT EXISTS idx_task_runs_status ON task_runs(status);
`

const postgresSchema = `
CREATE TABLE IF NOT EXISTS task_runs (
	run_id             TEXT PRIMARY KEY,
	task_name          TEXT NOT NULL,
	source             TEXT,
	status             TEXT NOT NULL,
	started_at         TIMESTAMPTZ NOT NULL,
	finished_at        TIMESTAMPTZ,
	last_heartbeat_at  TIMESTAMPTZ,
	ttl_seconds        BIGINT,
	duration_ms        BIGINT,
	error_message      TEXT
);
CREATE INDEX IF NOT EXISTS idx_task_runs_task_name ON task_runs(task_name);
CREATE INDEX IF NOT EXISTS idx_task_runs_status ON task_runs(status);
`

// Conn wraps *sql.DB and rewrites the "?" placeholders used throughout
// internal/store into each backend's native style ("?" for SQLite, "$1, $2,
// ..." for Postgres). This lets the store layer be written once against a
// single SQL dialect regardless of which backend is configured.
type Conn struct {
	*sql.DB
	driver string
}

// Open connects to the backend selected by cfg.DBDriver and ensures the
// schema exists.
func Open(cfg config.Config) (*Conn, error) {
	switch cfg.DBDriver {
	case "", "sqlite":
		return openSQLite(cfg.DBPath)
	case "postgres":
		return openPostgres(cfg.DBDSN)
	default:
		return nil, fmt.Errorf(`unsupported CHRONOS_DB_DRIVER %q (expected "sqlite" or "postgres")`, cfg.DBDriver)
	}
}

func openSQLite(path string) (*Conn, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	sqlDB.SetMaxOpenConns(1) // modernc.org/sqlite: avoid concurrent write locks in MVP

	if _, err := sqlDB.Exec(sqliteSchema); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Conn{DB: sqlDB, driver: "sqlite"}, nil
}

func openPostgres(dsn string) (*Conn, error) {
	if dsn == "" {
		return nil, fmt.Errorf("CHRONOS_DB_DSN is required when CHRONOS_DB_DRIVER=postgres")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
	}
	sqlDB.SetMaxOpenConns(10)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := sqlDB.Exec(postgresSchema); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Conn{DB: sqlDB, driver: "postgres"}, nil
}

func (c *Conn) Exec(query string, args ...any) (sql.Result, error) {
	return c.DB.Exec(c.rebind(query), args...)
}

func (c *Conn) Query(query string, args ...any) (*sql.Rows, error) {
	return c.DB.Query(c.rebind(query), args...)
}

func (c *Conn) QueryRow(query string, args ...any) *sql.Row {
	return c.DB.QueryRow(c.rebind(query), args...)
}

// rebind rewrites "?" placeholders to "$1, $2, ..." for Postgres. SQLite
// accepts "?" natively, so it's a no-op there.
func (c *Conn) rebind(query string) string {
	if c.driver != "postgres" || !strings.ContainsRune(query, '?') {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
