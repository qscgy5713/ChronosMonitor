package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port   string
	DBPath string

	// DBDriver selects the storage backend: "sqlite" (default, zero-dependency
	// file-based DB) or "postgres" (requires DBDSN).
	DBDriver string
	DBDSN    string

	TTLSweepInterval time.Duration

	// AlertWebhookURL enables alerting when set: a POST is sent for every
	// task.failed / task.timeout event. Empty disables alerting entirely.
	AlertWebhookURL string
	// AlertWebhookFormat shapes the POST body: "slack" (default, {"text":...},
	// also understood by Mattermost and other Slack-compatible receivers),
	// "discord" ({"content":...}), or "generic" (the raw task JSON).
	AlertWebhookFormat string
	// AlertRateLimitPerMinute caps how many alerts are sent per rolling
	// minute, so a burst of simultaneous failures (e.g. a shared dependency
	// going down) doesn't flood the notification channel. 0 disables the cap.
	AlertRateLimitPerMinute int

	// APIKey, when set, is required (as "Authorization: Bearer <key>" or a
	// "?token=<key>" query param) on every /api/v1/* request. Empty disables
	// auth entirely — the default, for backwards compatibility.
	APIKey string

	// RetentionDays, when > 0, enables a background job that permanently
	// deletes finished (success/failed/timeout) task runs older than this
	// many days. 0 (the default) keeps everything forever. Running tasks are
	// never deleted regardless of age.
	RetentionDays int
	// RetentionSweepInterval controls how often the retention job checks for
	// data to delete.
	RetentionSweepInterval time.Duration

	// MissedRunCheckInterval controls how often registered schedules are
	// checked for overdue tasks. There's no on/off flag for this one — it's
	// always running, since a schedule only ever affects anything once a
	// task_name is explicitly registered via the schedules API.
	MissedRunCheckInterval time.Duration
}

func Load() Config {
	return Config{
		Port:                    getEnv("CHRONOS_PORT", "8080"),
		DBPath:                  getEnv("CHRONOS_DB_PATH", "data/chronos.db"),
		DBDriver:                getEnv("CHRONOS_DB_DRIVER", "sqlite"),
		DBDSN:                   getEnv("CHRONOS_DB_DSN", ""),
		TTLSweepInterval:        getEnvSeconds("CHRONOS_TTL_SWEEP_INTERVAL_SECONDS", 30) * time.Second,
		AlertWebhookURL:         getEnv("CHRONOS_ALERT_WEBHOOK_URL", ""),
		AlertWebhookFormat:      getEnv("CHRONOS_ALERT_WEBHOOK_FORMAT", "slack"),
		AlertRateLimitPerMinute: getEnvInt("CHRONOS_ALERT_RATE_LIMIT_PER_MINUTE", 10),
		APIKey:                  getEnv("CHRONOS_API_KEY", ""),
		RetentionDays:           getEnvInt("CHRONOS_RETENTION_DAYS", 0),
		RetentionSweepInterval:  getEnvSeconds("CHRONOS_RETENTION_SWEEP_INTERVAL_SECONDS", 3600) * time.Second,
		MissedRunCheckInterval:  getEnvSeconds("CHRONOS_MISSED_RUN_CHECK_INTERVAL_SECONDS", 60) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSeconds(key string, fallback int) time.Duration {
	return time.Duration(getEnvInt(key, fallback))
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
