package config

import (
	"os"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port            string
	LogLevel        string
	DBURL           string
	RedisURL        string
	JWTSecret       string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	RateLimitMax    int
	RateLimitWindow time.Duration
	OTLPEndpoint    string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:            envOrDefault("PORT", "8080"),
		LogLevel:        envOrDefault("LOG_LEVEL", "info"),
		DBURL:           envOrDefault("DB_URL", "postgres://user:pass@localhost:5432/auth?sslmode=disable"),
		RedisURL:        envOrDefault("REDIS_URL", "redis://localhost:6379/0"),
		JWTSecret:       envOrDefault("JWT_SECRET", ""),
		JWTAccessTTL:    parseDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:   parseDuration("JWT_REFRESH_TTL", 168*time.Hour),
		RateLimitMax:    parseInt("RATE_LIMIT_MAX", 100),
		RateLimitWindow: parseDuration("RATE_LIMIT_WINDOW", 60*time.Second),
		OTLPEndpoint:    envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func parseInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}
