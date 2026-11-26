package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggerLogsAtInfoLevel(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := requestLogger(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("request log is not valid JSON: %v", err)
	}
	if entry["level"] != "INFO" || entry["msg"] != "HTTP request" ||
		entry["method"] != http.MethodGet || entry["path"] != "/healthz" {
		t.Errorf("unexpected request log entry: %#v", entry)
	}
}
