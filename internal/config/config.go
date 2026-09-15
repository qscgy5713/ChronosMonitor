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
}

func Load() Config {
	return Config{
		Port:             getEnv("CHRONOS_PORT", "8080"),
		DBPath:           getEnv("CHRONOS_DB_PATH", "data/chronos.db"),
		DBDriver:         getEnv("CHRONOS_DB_DRIVER", "sqlite"),
		DBDSN:            getEnv("CHRONOS_DB_DSN", ""),
		TTLSweepInterval: getEnvSeconds("CHRONOS_TTL_SWEEP_INTERVAL_SECONDS", 30) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSeconds(key string, fallback int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n)
		}
	}
	return time.Duration(fallback)
}
