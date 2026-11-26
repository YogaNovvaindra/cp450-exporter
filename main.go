package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/YogaNovvaindra/cp450-exporter/config"
	"github.com/YogaNovvaindra/cp450-exporter/cp450"
	"github.com/YogaNovvaindra/cp450-exporter/exporter"
)

func main() {
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(versionSummary())
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}
	logger := newLogger(cfg.LogLevel)

	targets := make([]exporter.Target, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		client, clientErr := cp450.NewClient(target.URL, cfg.Username, cfg.Password, cfg.Timeout)
		if clientErr != nil {
			logger.Error("create CP450 client", "target", target.Name, "error", clientErr)
			os.Exit(1)
		}
		targets = append(targets, exporter.Target{
			Name:    target.Name,
			URL:     target.URL,
			Scraper: client,
		})
	}

	collector, err := exporter.NewCollector(targets, cfg.Timeout, logger)
	if err != nil {
		logger.Error("create Prometheus collector", "error", err)
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collector)
	registerBuildInfo(registry)

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/healthz", textHandler(http.StatusOK, "ok\n"))
	mux.HandleFunc("/ready", textHandler(http.StatusOK, "ready\n"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, writeErr := fmt.Fprintf(w, "<html><body><h1>CP450 Exporter</h1><p><a href=%q>Metrics</a></p></body></html>\n", cfg.MetricsPath); writeErr != nil {
			logger.Debug("write landing page", "error", writeErr)
		}
	})

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           requestLogger(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      cfg.Timeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		logger.Info("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info(
		"starting CP450 exporter",
		"version", version,
		"listen_address", cfg.ListenAddress,
		"metrics_path", cfg.MetricsPath,
		"targets", len(targets),
	)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
	logger.Info("CP450 exporter stopped")
}

func registerBuildInfo(registry *prometheus.Registry) {
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cp450",
		Subsystem: "exporter",
		Name:      "build_info",
		Help:      "Build information for the CP450 exporter.",
	}, []string{"version", "commit", "build_date"})
	buildInfo.WithLabelValues(version, commit, buildDate).Set(1)
	registry.MustRegister(buildInfo)
}

func textHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			return
		}
	}
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info(
			"HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"duration", time.Since(started),
		)
	})
}

func newLogger(level string) *slog.Logger {
	var parsed slog.Level
	switch level {
	case "debug":
		parsed = slog.LevelDebug
	case "warn":
		parsed = slog.LevelWarn
	case "error":
		parsed = slog.LevelError
	default:
		parsed = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: parsed}))
}
