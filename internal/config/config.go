package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                   string
	PostgresDSN            string
	QueueMaxSize           int
	BatchMaxSize           int
	BatchMaxWait           time.Duration
	MaxBodyBytes           int64
	RateLimitMetricsPerMin int
	APIKeys                map[string]struct{}
	ClockSkew              time.Duration
}

func Parse() Config {
	return Config{
		Port:                   getString("PORT", "8080"),
		PostgresDSN:            getString("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/events?sslmode=disable"),
		QueueMaxSize:           getInt("QUEUE_MAX_SIZE", 10_000),
		BatchMaxSize:           getInt("BATCH_MAX_SIZE", 500),
		BatchMaxWait:           time.Duration(getInt("BATCH_MAX_WAIT_MS", 50)) * time.Millisecond,
		MaxBodyBytes:           int64(getInt("MAX_BODY_BYTES", 1_048_576)),
		RateLimitMetricsPerMin: getInt("RATE_LIMIT_METRICS_PER_MIN", 20),
		APIKeys:                parseKeys(getString("API_KEYS", "")),
		ClockSkew:              time.Duration(getInt("CLOCK_SKEW_SECONDS", 300)) * time.Second,
	}
}

func parseKeys(csv string) map[string]struct{} {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return map[string]struct{}{}
	}
	m := make(map[string]struct{})
	for _, k := range strings.Split(csv, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			m[k] = struct{}{}
		}
	}
	return m
}

func getString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
