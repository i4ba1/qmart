package config

import (
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration, sourced from environment variables
// with sensible defaults so the service runs with minimal setup.
type Config struct {
	Addr        string
	DSN         string
	TokenSecret string
	TokenTTL    time.Duration
	AdminEmails []string
}

func Load() Config {
	return Config{
		Addr: getenv("QMART_ADDR", ":8080"),
		DSN: getenv("QMART_DSN",
			"postgres://qmart:qmart@localhost:5432/qmart?sslmode=disable"),
		TokenSecret: getenv("QMART_TOKEN_SECRET", "dev-secret-change-me"),
		TokenTTL:    24 * time.Hour,
		AdminEmails: splitCSV(getenv("QMART_ADMIN_EMAILS", "admin@example.com")),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}
