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
	if cfg.AlertWebhookURL != "" {
		t.Errorf("AlertWebhookURL = %q, want empty (alerting disabled by default)", cfg.AlertWebhookURL)
	}
	if cfg.AlertWebhookFormat != "slack" {
		t.Errorf("AlertWebhookFormat = %q, want slack", cfg.AlertWebhookFormat)
	}
	if cfg.AlertRateLimitPerMinute != 10 {
		t.Errorf("AlertRateLimitPerMinute = %d, want 10", cfg.AlertRateLimitPerMinute)
	}
}

func TestLoadReadsEnvOverrides(t *testing.T) {
	t.Setenv("CHRONOS_PORT", "9090")
	t.Setenv("CHRONOS_DB_DRIVER", "postgres")
	t.Setenv("CHRONOS_DB_DSN", "postgres://x")
	t.Setenv("CHRONOS_TTL_SWEEP_INTERVAL_SECONDS", "5")
	t.Setenv("CHRONOS_ALERT_WEBHOOK_URL", "https://hooks.example.com/x")
	t.Setenv("CHRONOS_ALERT_WEBHOOK_FORMAT", "discord")
	t.Setenv("CHRONOS_ALERT_RATE_LIMIT_PER_MINUTE", "3")

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
	if cfg.AlertWebhookURL != "https://hooks.example.com/x" {
		t.Errorf("AlertWebhookURL = %q, want https://hooks.example.com/x", cfg.AlertWebhookURL)
	}
	if cfg.AlertWebhookFormat != "discord" {
		t.Errorf("AlertWebhookFormat = %q, want discord", cfg.AlertWebhookFormat)
	}
	if cfg.AlertRateLimitPerMinute != 3 {
		t.Errorf("AlertRateLimitPerMinute = %d, want 3", cfg.AlertRateLimitPerMinute)
	}
}
