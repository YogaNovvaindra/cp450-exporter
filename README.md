# CP450 Exporter

Prometheus exporter for TOTOLINK CP450 devices. It implements the device's
private CGI login/session flow and converts status, connected-device, and Wi-Fi
station data into Prometheus metrics without exporting passwords or Wi-Fi keys.

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://docker.com/)

Validated with CP450 firmware `V4.1.0cu.747` in bridge and client modes.

## Quick start

```bash
export CP450_TARGETS='bridge=http://10.1.1.240,client=http://10.1.1.241'
export CP450_USERNAME='admin'
export CP450_PASSWORD='<device password>'

go run .
curl http://127.0.0.1:9817/metrics
```

The exporter logs in lazily on the first scrape, reuses each device session,
and automatically logs in again when a session expires. Targets are scraped in
parallel so one slow CP450 does not delay another.

## Docker

```bash
docker build -t cp450-exporter .

docker run --rm -p 9817:9817 \
  -e CP450_TARGETS='bridge=http://10.1.1.240,client=http://10.1.1.241' \
  -e CP450_USERNAME='admin' \
  -e CP450_PASSWORD='<device password>' \
  cp450-exporter
```

Published images from this repository are available at:

```text
ghcr.io/yoganovvaindra/cp450-exporter:latest
```

## Configuration

| Variable | Description | Default |
| --- | --- | --- |
| `CP450_TARGETS` | Comma-separated `name=URL` devices | Required |
| `CP450_USERNAME` | Device username | `admin` |
| `CP450_PASSWORD` | Device password | Required |
| `CP450_TIMEOUT` | Maximum duration of one target scrape | `15s` |
| `WEB_LISTEN_ADDRESS` | Exporter listen address | `:9817` |
| `WEB_TELEMETRY_PATH` | Prometheus endpoint | `/metrics` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error` | `info` |

A target name is optional; when omitted, its hostname is used:

```bash
CP450_TARGETS='http://10.1.1.240,http://10.1.1.241'
```

## Endpoints

| Endpoint | Description |
| --- | --- |
| `/metrics` | Prometheus metrics; triggers live device collection |
| `/healthz` | Exporter process health |
| `/ready` | Exporter readiness |

Device reachability is represented by `cp450_up{target="..."}`. The exporter
stays healthy when one CP450 is offline, allowing Prometheus to retain metrics
for the other targets.

## Metrics

Important metrics include:

| Metric | Description |
| --- | --- |
| `cp450_up` | Whether the device scrape succeeded |
| `cp450_uptime_seconds` | Device uptime |
| `cp450_info` | Product, firmware, mode, and LAN identity |
| `cp450_lan_received_packets_total` | LAN receive packets |
| `cp450_lan_transmitted_packets_total` | LAN transmit packets |
| `cp450_wan_connected` | WAN connection state |
| `cp450_wifi_channel` | Current radio channel |
| `cp450_wifi_clients` | Associated clients per SSID |
| `cp450_connected_devices_count` | Connected devices by wired/wireless type |
| `cp450_connected_device_info` | Connected client IP/MAC information |
| `cp450_wifi_station_signal_dbm` | Station signal strength |
| `cp450_wifi_station_signal_percent` | Station signal quality |
| `cp450_port_link_up` | Link state by firmware port index |
| `cp450_exporter_scrape_errors_total` | Failed device scrapes |

The device's positive `dbm` magnitude is normalized to conventional negative
dBm; for example, native value `60` is exported as `-60`.

## Prometheus scrape configuration

```yaml
scrape_configs:
  - job_name: cp450
    static_configs:
      - targets:
          - cp450-exporter:9817
```

Example alert:

```yaml
groups:
  - name: cp450
    rules:
      - alert: CP450Down
        expr: cp450_up == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: 'CP450 {{ $labels.target }} is unreachable'
```

## Development

```bash
go test -race ./...
go build -o cp450-exporter .
docker build -t cp450-exporter .
```

## Security

- CP450 firmware `V4.1.0cu.747` uses plain HTTP and includes the password in an
  internal login redirect. Run this exporter on a trusted management network.
- Native status responses contain Wi-Fi keys. The Go response structs omit all
  secret fields, so they are never registered as metrics.
- Do not place `CP450_PASSWORD` in the image or repository. Inject it as a
  runtime secret.
