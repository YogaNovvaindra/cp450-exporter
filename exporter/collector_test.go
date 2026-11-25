package exporter

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/YogaNovvaindra/cp450-exporter/cp450"
)

type fakeScraper struct {
	snapshot cp450.Snapshot
	err      error
}

func (f fakeScraper) Snapshot(context.Context) (cp450.Snapshot, error) {
	return f.snapshot, f.err
}

func TestCollectorExportsDeviceMetrics(t *testing.T) {
	t.Parallel()

	collector, err := NewCollector([]Target{{
		Name: "bridge",
		URL:  "http://10.1.1.240",
		Scraper: fakeScraper{snapshot: cp450.Snapshot{
			Status: cp450.Status{
				ProductName:   "CP450",
				FMVersion:     "V4.1.0cu.747",
				OperationMode: 1,
				UpTime:        "1;2;3;4",
				SSID1:         "TENGAH",
				BSSID1:        "aa:bb:cc:dd:ee:ff",
				StationNum1:   1,
				AuthMode:      "WPAPSKWPA2PSK",
				Encryption:    "TKIPAES",
			},
			Devices:  []cp450.Device{{Index: 1, Type: 1, IP: "10.1.1.10", MAC: "aa:bb:cc:dd:ee:ff"}},
			Stations: []cp450.Station{{MAC: "aa:bb:cc:dd:ee:ff", DBM: 60, RSSI: 90, Time: 300}},
		}},
	}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)
	expected := `
# HELP cp450_connected_devices_count Number of connected devices by access type.
# TYPE cp450_connected_devices_count gauge
cp450_connected_devices_count{access="unknown",target="bridge"} 0
cp450_connected_devices_count{access="wired",target="bridge"} 0
cp450_connected_devices_count{access="wireless",target="bridge"} 1
# HELP cp450_up Whether the last CP450 target scrape succeeded.
# TYPE cp450_up gauge
cp450_up{target="bridge"} 1
# HELP cp450_uptime_seconds CP450 uptime in seconds.
# TYPE cp450_uptime_seconds gauge
cp450_uptime_seconds{target="bridge"} 93784
# HELP cp450_wifi_station_signal_dbm Associated Wi-Fi station signal strength in dBm.
# TYPE cp450_wifi_station_signal_dbm gauge
cp450_wifi_station_signal_dbm{mac="AA:BB:CC:DD:EE:FF",target="bridge"} -60
`
	if err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"cp450_connected_devices_count",
		"cp450_up",
		"cp450_uptime_seconds",
		"cp450_wifi_station_signal_dbm",
	); err != nil {
		t.Fatalf("GatherAndCompare() error = %v", err)
	}
}

func TestCollectorExportsFailure(t *testing.T) {
	t.Parallel()

	collector, err := NewCollector([]Target{{
		Name:    "offline",
		URL:     "http://10.1.1.250",
		Scraper: fakeScraper{err: errors.New("unreachable")},
	}}, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewCollector() error = %v", err)
	}

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(collector)
	expected := `
# HELP cp450_exporter_scrape_errors_total Total number of failed CP450 target scrapes.
# TYPE cp450_exporter_scrape_errors_total counter
cp450_exporter_scrape_errors_total{target="offline"} 1
# HELP cp450_up Whether the last CP450 target scrape succeeded.
# TYPE cp450_up gauge
cp450_up{target="offline"} 0
`
	if err := testutil.GatherAndCompare(
		registry,
		strings.NewReader(expected),
		"cp450_exporter_scrape_errors_total",
		"cp450_up",
	); err != nil {
		t.Fatalf("GatherAndCompare() error = %v", err)
	}
}

func TestDurationSeconds(t *testing.T) {
	t.Parallel()
	if got := durationSeconds("1;2;3;4"); got != 93784 {
		t.Fatalf("durationSeconds() = %v, want 93784", got)
	}
}
