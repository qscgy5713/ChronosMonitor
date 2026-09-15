package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DBDriver != "sqlite" {
		t.Errorf("DBDriver = %q, want sqlite", cfg.DBDriver)
	}
	if cfg.DBDSN != "" {
		t.Errorf("DBDSN = %q, want empty by default", cfg.DBDSN)
	}
	if cfg.TTLSweepInterval != 30*time.Second {
		t.Errorf("TTLSweepInterval = %v, want 30s", cfg.TTLSweepInterval)
	}
}

func TestLoadReadsEnvOverrides(t *testing.T) {
	t.Setenv("CHRONOS_PORT", "9090")
	t.Setenv("CHRONOS_DB_DRIVER", "postgres")
	t.Setenv("CHRONOS_DB_DSN", "postgres://x")
	t.Setenv("CHRONOS_TTL_SWEEP_INTERVAL_SECONDS", "5")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.DBDriver != "postgres" {
		t.Errorf("DBDriver = %q, want postgres", cfg.DBDriver)
	}
	if cfg.DBDSN != "postgres://x" {
		t.Errorf("DBDSN = %q, want postgres://x", cfg.DBDSN)
	}
	if cfg.TTLSweepInterval != 5*time.Second {
		t.Errorf("TTLSweepInterval = %v, want 5s", cfg.TTLSweepInterval)
	}
}
