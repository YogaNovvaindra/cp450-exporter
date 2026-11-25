package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddress = ":9817"
	defaultMetricsPath   = "/metrics"
	defaultUsername      = "admin"
	defaultTimeout       = 15 * time.Second
)

// Target identifies one CP450 device.
type Target struct {
	Name string
	URL  string
}

// Config contains exporter configuration loaded from environment variables.
type Config struct {
	ListenAddress string
	MetricsPath   string
	Username      string
	Password      string
	Timeout       time.Duration
	LogLevel      string
	Targets       []Target
}

// Load reads and validates exporter configuration.
func Load() (Config, error) {
	targets, err := ParseTargets(os.Getenv("CP450_TARGETS"))
	if err != nil {
		return Config{}, err
	}

	password := os.Getenv("CP450_PASSWORD")
	if password == "" {
		return Config{}, fmt.Errorf("CP450_PASSWORD is required")
	}

	timeout := defaultTimeout
	if raw := os.Getenv("CP450_TIMEOUT"); raw != "" {
		timeout, err = time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse CP450_TIMEOUT: %w", err)
		}
		if timeout <= 0 {
			return Config{}, fmt.Errorf("CP450_TIMEOUT must be greater than zero")
		}
	}

	metricsPath := envOrDefault("WEB_TELEMETRY_PATH", defaultMetricsPath)
	if !strings.HasPrefix(metricsPath, "/") {
		return Config{}, fmt.Errorf("WEB_TELEMETRY_PATH must begin with /")
	}

	return Config{
		ListenAddress: envOrDefault("WEB_LISTEN_ADDRESS", defaultListenAddress),
		MetricsPath:   metricsPath,
		Username:      envOrDefault("CP450_USERNAME", defaultUsername),
		Password:      password,
		Timeout:       timeout,
		LogLevel:      envOrDefault("LOG_LEVEL", "info"),
		Targets:       targets,
	}, nil
}

// ParseTargets parses comma-separated name=URL entries. If the name is
// omitted, the URL hostname is used.
func ParseTargets(raw string) ([]Target, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("CP450_TARGETS is required")
	}

	seen := make(map[string]struct{})
	entries := strings.Split(raw, ",")
	targets := make([]Target, 0, len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		name := ""
		rawURL := entry
		if before, after, found := strings.Cut(entry, "="); found {
			name = strings.TrimSpace(before)
			rawURL = strings.TrimSpace(after)
		}

		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("invalid CP450 target URL %q", rawURL)
		}
		if parsed.User != nil {
			return nil, fmt.Errorf("credentials must not be embedded in target URL %q", rawURL)
		}

		if name == "" {
			name = parsed.Hostname()
		}
		if name == "" {
			return nil, fmt.Errorf("target name is empty for %q", entry)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate CP450 target name %q", name)
		}
		seen[name] = struct{}{}

		parsed.Path = strings.TrimRight(parsed.Path, "/")
		targets = append(targets, Target{Name: name, URL: parsed.String()})
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("CP450_TARGETS did not contain any targets")
	}
	return targets, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
