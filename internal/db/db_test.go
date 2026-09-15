package db

import (
	"testing"

	"chronosmonitor/internal/config"
)

func TestRebind(t *testing.T) {
	cases := []struct {
		name   string
		driver string
		query  string
		want   string
	}{
		{
			name:   "sqlite leaves placeholders untouched",
			driver: "sqlite",
			query:  "SELECT * FROM t WHERE a = ? AND b = ?",
			want:   "SELECT * FROM t WHERE a = ? AND b = ?",
		},
		{
			name:   "postgres rewrites placeholders in order",
			driver: "postgres",
			query:  "SELECT * FROM t WHERE a = ? AND b = ?",
			want:   "SELECT * FROM t WHERE a = $1 AND b = $2",
		},
		{
			name:   "postgres query with no placeholders is unchanged",
			driver: "postgres",
			query:  "SELECT 1",
			want:   "SELECT 1",
		},
		{
			name:   "postgres handles double-digit placeholder counts",
			driver: "postgres",
			query:  "?,?,?,?,?,?,?,?,?,?,?",
			want:   "$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conn := &Conn{driver: tc.driver}
			got := conn.rebind(tc.query)
			if got != tc.want {
				t.Errorf("rebind(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	_, err := Open(config.Config{DBDriver: "mysql"})
	if err == nil {
		t.Fatal("expected an error for an unsupported driver, got nil")
	}
}

func TestOpenRejectsPostgresWithoutDSN(t *testing.T) {
	_, err := Open(config.Config{DBDriver: "postgres"})
	if err == nil {
		t.Fatal("expected an error when CHRONOS_DB_DSN is empty, got nil")
	}
}
