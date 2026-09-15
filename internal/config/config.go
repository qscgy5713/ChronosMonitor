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
