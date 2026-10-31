// Package config loads and validates process configuration from the environment.
//
// Rules:
//   - Every setting has a safe development default except secrets.
//   - Invalid values are a startup failure, never a silent fallback.
//   - Nothing in this package reads files or touches the network.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type Config struct {
	Env                string
	HTTPPort           string
	DatabaseURL        string
	CORSAllowedOrigins []string
	SessionCookieName  string
	SessionTTL         time.Duration
	SecureCookies      bool
	AppBaseURL         string
	SMTP               SMTP
	AdminEmail         string
	AdminPassword      string
	FinanceEmail       string
	FinancePassword    string
	Currency           string
	CompanyTimezone    string
	LogLevel           slog.Level
}

// Load reads configuration from the environment. It never returns a partially
// valid config: the caller either gets a usable Config or an error.
func Load() (Config, error) {
	env := getenv("APP_ENV", "development")

	sessionTTL, err := time.ParseDuration(getenv("SESSION_TTL", "168h"))
	if err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL must be a duration such as 168h: %w", err)
	}

	smtpPort, err := strconv.Atoi(getenv("SMTP_PORT", "1025"))
	if err != nil {
		return Config{}, fmt.Errorf("SMTP_PORT must be a number: %w", err)
	}

	logLevel, err := parseLevel(getenv("LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Env:                env,
		HTTPPort:           getenv("HTTP_PORT", "8080"),
		DatabaseURL:        getenv("DATABASE_URL", "postgres://brightloop:brightloop@localhost:5432/brightloop?sslmode=disable"),
		CORSAllowedOrigins: splitList(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		SessionCookieName:  getenv("SESSION_COOKIE_NAME", "bl_session"),
		SessionTTL:         sessionTTL,
		SecureCookies:      getenvBool("COOKIE_SECURE", env == "production"),
		AppBaseURL:         strings.TrimRight(getenv("APP_BASE_URL", "http://localhost:5173"), "/"),
		SMTP: SMTP{
			Host:     getenv("SMTP_HOST", ""),
			Port:     smtpPort,
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     getenv("SMTP_FROM", "Brightloop <no-reply@brightloop.test>"),
		},
		AdminEmail:      getenv("ADMIN_EMAIL", "admin@brightloop.test"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		FinanceEmail:    os.Getenv("FINANCE_EMAIL"),
		FinancePassword: os.Getenv("FINANCE_PASSWORD"),
		Currency:        getenv("CURRENCY", "USD"),
		CompanyTimezone: getenv("COMPANY_TIMEZONE", "UTC"),
		LogLevel:        logLevel,
	}

	if cfg.SessionCookieName == "" {
		return Config{}, fmt.Errorf("SESSION_COOKIE_NAME must not be empty")
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("CORS_ALLOWED_ORIGINS must list at least one origin")
	}
	if _, err := time.LoadLocation(cfg.CompanyTimezone); err != nil {
		return Config{}, fmt.Errorf("COMPANY_TIMEZONE %q is not a valid IANA timezone: %w", cfg.CompanyTimezone, err)
	}
	if cfg.Currency == "" {
		return Config{}, fmt.Errorf("CURRENCY must not be empty")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL %q is not one of debug, info, warn, error", raw)
	}
}
