package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/YogaNovvaindra/cp450-exporter/cp450"
)

const namespace = "cp450"

var operationModes = map[int]string{
	0: "gateway",
	1: "bridge",
	2: "repeater",
	3: "wisp",
	4: "mesh",
	5: "client",
}

var wanModes = map[int]string{
	0: "static",
	1: "dhcp",
	3: "pppoe",
	4: "pptp",
	6: "l2tp",
}

// Target associates a configured name and URL with a device scraper.
type Target struct {
	Name    string
	URL     string
	Scraper cp450.Scraper
}

// Collector gathers a fresh CP450 snapshot whenever Prometheus scrapes it.
type Collector struct {
	targets        []Target
	timeout        time.Duration
	logger         *slog.Logger
	collectionMu   sync.Mutex
	scrapes        *prometheus.CounterVec
	scrapeErrors   *prometheus.CounterVec
	targetInfo     *prometheus.Desc
	up             *prometheus.Desc
	scrapeDuration *prometheus.Desc
	deviceInfo     *prometheus.Desc
	uptime         *prometheus.Desc
	lanRx          *prometheus.Desc
	lanTx          *prometheus.Desc
	lanDHCP        *prometheus.Desc
	wanInfo        *prometheus.Desc
	wanConnected   *prometheus.Desc
	wanUptime      *prometheus.Desc
	wanRx          *prometheus.Desc
	wanTx          *prometheus.Desc
	wifiEnabled    *prometheus.Desc
	wifiBand       *prometheus.Desc
	wifiChannel    *prometheus.Desc
	wifiRx         *prometheus.Desc
	wifiTx         *prometheus.Desc
	ssidInfo       *prometheus.Desc
	ssidEnabled    *prometheus.Desc
	wifiClients    *prometheus.Desc
	repeaterEnable *prometheus.Desc
	repeaterUp     *prometheus.Desc
	repeaterSignal *prometheus.Desc
	portLink       *prometheus.Desc
	deviceCount    *prometheus.Desc
	clientInfo     *prometheus.Desc
	stationCount   *prometheus.Desc
	stationInfo    *prometheus.Desc
	stationSignal  *prometheus.Desc
	stationPercent *prometheus.Desc
	stationTime    *prometheus.Desc
}

func NewCollector(targets []Target, timeout time.Duration, logger *slog.Logger) (*Collector, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one CP450 target is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("collector timeout must be greater than zero")
	}
	if logger == nil {
		logger = slog.Default()
	}

	collector := &Collector{
		targets: targets,
		timeout: timeout,
		logger:  logger,
		scrapes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrapes_total",
			Help:      "Total number of CP450 target scrapes.",
		}, []string{"target"}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of failed CP450 target scrapes.",
		}, []string{"target"}),
		targetInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "target_info"),
			"Static information about a configured CP450 target.",
			[]string{"target", "url"}, nil,
		),
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Whether the last CP450 target scrape succeeded.",
			[]string{"target"}, nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
			"Duration of the last CP450 target scrape in seconds.",
			[]string{"target"}, nil,
		),
		deviceInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "info"),
			"CP450 device and firmware information.",
			[]string{"target", "product", "firmware", "hardware_model", "operation_mode", "lan_ip", "lan_mac"}, nil,
		),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uptime_seconds"),
			"CP450 uptime in seconds.", []string{"target"}, nil,
		),
		lanRx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "lan", "received_packets_total"),
			"Total packets received on the LAN interface.", []string{"target"}, nil,
		),
		lanTx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "lan", "transmitted_packets_total"),
			"Total packets transmitted on the LAN interface.", []string{"target"}, nil,
		),
		lanDHCP: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "lan", "dhcp_server_enabled"),
			"Whether the CP450 DHCP server is enabled.", []string{"target"}, nil,
		),
		wanInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wan", "info"),
			"CP450 WAN interface information.",
			[]string{"target", "mode", "ip", "gateway", "mac"}, nil,
		),
		wanConnected: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wan", "connected"),
			"Whether the WAN interface is connected.", []string{"target"}, nil,
		),
		wanUptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wan", "connection_uptime_seconds"),
			"Current WAN connection uptime in seconds.", []string{"target"}, nil,
		),
		wanRx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wan", "received_packets_total"),
			"Total packets received on the WAN interface.", []string{"target"}, nil,
		),
		wanTx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wan", "transmitted_packets_total"),
			"Total packets transmitted on the WAN interface.", []string{"target"}, nil,
		),
		wifiEnabled: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "enabled"),
			"Whether the primary Wi-Fi radio is enabled.", []string{"target"}, nil,
		),
		wifiBand: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "band_mode"),
			"Numeric Wi-Fi band mode reported by the firmware.", []string{"target"}, nil,
		),
		wifiChannel: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "channel"),
			"Current Wi-Fi channel.", []string{"target"}, nil,
		),
		wifiRx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "received_packets_total"),
			"Total packets received by the Wi-Fi radio.", []string{"target"}, nil,
		),
		wifiTx: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "transmitted_packets_total"),
			"Total packets transmitted by the Wi-Fi radio.", []string{"target"}, nil,
		),
		ssidInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "ssid_info"),
			"Configured CP450 SSID information.",
			[]string{"target", "index", "ssid", "bssid", "authentication", "encryption"}, nil,
		),
		ssidEnabled: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "ssid_enabled"),
			"Whether a configured SSID is enabled.", []string{"target", "index"}, nil,
		),
		wifiClients: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi", "clients"),
			"Number of associated clients on an SSID.", []string{"target", "index", "ssid"}, nil,
		),
		repeaterEnable: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_repeater", "enabled"),
			"Whether repeater/client uplink mode is enabled.", []string{"target"}, nil,
		),
		repeaterUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_repeater", "connected"),
			"Whether the repeater/client uplink is connected.", []string{"target"}, nil,
		),
		repeaterSignal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_repeater", "signal_dbm"),
			"Repeater/client uplink signal strength in dBm.", []string{"target"}, nil,
		),
		portLink: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "port", "link_up"),
			"Whether a firmware port index reports link up.", []string{"target", "index"}, nil,
		),
		deviceCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "connected_devices", "count"),
			"Number of connected devices by access type.", []string{"target", "access"}, nil,
		),
		clientInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "connected_device", "info"),
			"Connected device information.", []string{"target", "id", "ip", "mac", "access"}, nil,
		),
		stationCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_station", "count"),
			"Number of associated Wi-Fi stations.", []string{"target"}, nil,
		),
		stationInfo: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_station", "info"),
			"Associated Wi-Fi station information.",
			[]string{"target", "mac", "ip", "mode", "bandwidth"}, nil,
		),
		stationSignal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_station", "signal_dbm"),
			"Associated Wi-Fi station signal strength in dBm.", []string{"target", "mac"}, nil,
		),
		stationPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_station", "signal_percent"),
			"Associated Wi-Fi station signal quality as a percentage.", []string{"target", "mac"}, nil,
		),
		stationTime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "wifi_station", "connected_seconds"),
			"Associated Wi-Fi station connection duration in seconds.", []string{"target", "mac"}, nil,
		),
	}
	for _, target := range targets {
		collector.scrapes.WithLabelValues(target.Name)
		collector.scrapeErrors.WithLabelValues(target.Name)
	}
	return collector, nil
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	c.scrapes.Describe(ch)
	c.scrapeErrors.Describe(ch)
	for _, desc := range c.descriptions() {
		ch <- desc
	}
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.collectionMu.Lock()
	defer c.collectionMu.Unlock()

	var wg sync.WaitGroup
	for _, target := range c.targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.collectTarget(ch, target)
		}()
	}
	wg.Wait()
	c.scrapes.Collect(ch)
	c.scrapeErrors.Collect(ch)
}

func (c *Collector) collectTarget(ch chan<- prometheus.Metric, target Target) {
	started := time.Now()
	c.scrapes.WithLabelValues(target.Name).Inc()
	ch <- prometheus.MustNewConstMetric(c.targetInfo, prometheus.GaugeValue, 1, target.Name, target.URL)

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	snapshot, err := target.Scraper.Snapshot(ctx)
	duration := time.Since(started).Seconds()
	ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration, target.Name)

	if err != nil {
		c.scrapeErrors.WithLabelValues(target.Name).Inc()
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, target.Name)
		c.logger.Error("CP450 scrape failed", "target", target.Name, "error", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, target.Name)
	c.collectStatus(ch, target.Name, snapshot.Status)
	c.collectDevices(ch, target.Name, snapshot.Devices)
	c.collectStations(ch, target.Name, snapshot.Stations)
}

func (c *Collector) collectStatus(ch chan<- prometheus.Metric, target string, status cp450.Status) {
	operationMode := nameFor(operationModes, status.OperationMode.Int())
	wanMode := nameFor(wanModes, status.WANMode.Int())

	ch <- prometheus.MustNewConstMetric(
		c.deviceInfo,
		prometheus.GaugeValue,
		1,
		target,
		status.ProductName,
		status.FMVersion,
		status.HardModel,
		operationMode,
		status.LANIP,
		strings.ToUpper(status.LANMAC),
	)
	ch <- prometheus.MustNewConstMetric(c.uptime, prometheus.GaugeValue, durationSeconds(status.UpTime), target)
	ch <- prometheus.MustNewConstMetric(c.lanRx, prometheus.CounterValue, status.LANRx.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.lanTx, prometheus.CounterValue, status.LANTx.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.lanDHCP, prometheus.GaugeValue, boolFloat(status.DHCPServer.Int() == 1), target)

	ch <- prometheus.MustNewConstMetric(c.wanInfo, prometheus.GaugeValue, 1, target, wanMode, status.WANIP, status.WANGateway, strings.ToUpper(status.WANMAC))
	ch <- prometheus.MustNewConstMetric(c.wanConnected, prometheus.GaugeValue, boolFloat(strings.EqualFold(status.WANConnStatus, "connected")), target)
	ch <- prometheus.MustNewConstMetric(c.wanUptime, prometheus.GaugeValue, durationSeconds(status.WANConnTime), target)
	ch <- prometheus.MustNewConstMetric(c.wanRx, prometheus.CounterValue, status.WANRx.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.wanTx, prometheus.CounterValue, status.WANTx.Float64(), target)

	ch <- prometheus.MustNewConstMetric(c.wifiEnabled, prometheus.GaugeValue, boolFloat(status.WiFiOff.Int() == 0), target)
	ch <- prometheus.MustNewConstMetric(c.wifiBand, prometheus.GaugeValue, status.Band.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.wifiChannel, prometheus.GaugeValue, status.Channel.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.wifiRx, prometheus.CounterValue, status.WLANRx.Float64(), target)
	ch <- prometheus.MustNewConstMetric(c.wifiTx, prometheus.CounterValue, status.WLANTx.Float64(), target)

	for _, ssid := range statusSSIDs(status) {
		index := strconv.Itoa(ssid.index)
		ch <- prometheus.MustNewConstMetric(c.ssidInfo, prometheus.GaugeValue, 1, target, index, ssid.ssid, strings.ToUpper(ssid.bssid), ssid.authentication, ssid.encryption)
		ch <- prometheus.MustNewConstMetric(c.ssidEnabled, prometheus.GaugeValue, boolFloat(ssid.enabled), target, index)
		ch <- prometheus.MustNewConstMetric(c.wifiClients, prometheus.GaugeValue, ssid.clients, target, index, ssid.ssid)
	}

	ch <- prometheus.MustNewConstMetric(c.repeaterEnable, prometheus.GaugeValue, boolFloat(status.APClientEnable.Int() == 1), target)
	ch <- prometheus.MustNewConstMetric(c.repeaterUp, prometheus.GaugeValue, boolFloat(strings.EqualFold(status.APClientStatus, "success")), target)
	ch <- prometheus.MustNewConstMetric(c.repeaterSignal, prometheus.GaugeValue, negativeDBM(status.APClientRSSI.Float64()), target)

	for index, value := range splitNonEmpty(status.PortLinkState, ",") {
		ch <- prometheus.MustNewConstMetric(c.portLink, prometheus.GaugeValue, boolFloat(value == "1"), target, strconv.Itoa(index))
	}
}

func (c *Collector) collectDevices(ch chan<- prometheus.Metric, target string, devices []cp450.Device) {
	counts := map[string]float64{"wired": 0, "wireless": 0, "unknown": 0}
	for _, device := range devices {
		access := accessName(device.Type.Int())
		counts[access]++
		ch <- prometheus.MustNewConstMetric(
			c.clientInfo,
			prometheus.GaugeValue,
			1,
			target,
			strconv.Itoa(device.Index.Int()),
			device.IP,
			strings.ToUpper(device.MAC),
			access,
		)
	}
	for _, access := range []string{"wired", "wireless", "unknown"} {
		ch <- prometheus.MustNewConstMetric(c.deviceCount, prometheus.GaugeValue, counts[access], target, access)
	}
}

func (c *Collector) collectStations(ch chan<- prometheus.Metric, target string, stations []cp450.Station) {
	ch <- prometheus.MustNewConstMetric(c.stationCount, prometheus.GaugeValue, float64(len(stations)), target)
	for _, station := range stations {
		mac := strings.ToUpper(station.MAC)
		ch <- prometheus.MustNewConstMetric(c.stationInfo, prometheus.GaugeValue, 1, target, mac, station.IP, station.Mode, station.Bandwidth)
		ch <- prometheus.MustNewConstMetric(c.stationSignal, prometheus.GaugeValue, negativeDBM(station.DBM.Float64()), target, mac)
		ch <- prometheus.MustNewConstMetric(c.stationPercent, prometheus.GaugeValue, station.RSSI.Float64(), target, mac)
		ch <- prometheus.MustNewConstMetric(c.stationTime, prometheus.GaugeValue, station.Time.Float64(), target, mac)
	}
}

func (c *Collector) descriptions() []*prometheus.Desc {
	return []*prometheus.Desc{
		c.targetInfo, c.up, c.scrapeDuration, c.deviceInfo, c.uptime,
		c.lanRx, c.lanTx, c.lanDHCP, c.wanInfo, c.wanConnected, c.wanUptime,
		c.wanRx, c.wanTx, c.wifiEnabled, c.wifiBand, c.wifiChannel, c.wifiRx,
		c.wifiTx, c.ssidInfo, c.ssidEnabled, c.wifiClients, c.repeaterEnable,
		c.repeaterUp, c.repeaterSignal, c.portLink, c.deviceCount, c.clientInfo,
		c.stationCount, c.stationInfo, c.stationSignal, c.stationPercent, c.stationTime,
	}
}

type ssidStatus struct {
	index          int
	ssid           string
	bssid          string
	authentication string
	encryption     string
	enabled        bool
	clients        float64
}

func statusSSIDs(status cp450.Status) []ssidStatus {
	authentication := strings.Split(status.AuthMode, ";")
	encryption := strings.Split(status.Encryption, ";")
	values := []ssidStatus{
		{index: 1, ssid: status.SSID1, bssid: status.BSSID1, enabled: status.WiFiOff.Int() == 0, clients: status.StationNum1.Float64()},
		{index: 2, ssid: status.SSID2, bssid: status.BSSID2, enabled: status.WiFiOff2.Int() == 0, clients: status.StationNum2.Float64()},
		{index: 3, ssid: status.SSID3, bssid: status.BSSID3, enabled: status.WiFiOff3.Int() == 0, clients: status.StationNum3.Float64()},
	}

	result := make([]ssidStatus, 0, len(values))
	for index := range values {
		if values[index].ssid == "" && values[index].bssid == "" {
			continue
		}
		if index < len(authentication) {
			values[index].authentication = authentication[index]
		}
		if index < len(encryption) {
			values[index].encryption = encryption[index]
		}
		result = append(result, values[index])
	}
	return result
}

func durationSeconds(value string) float64 {
	parts := strings.Split(value, ";")
	if len(parts) != 4 {
		return 0
	}
	parsed := make([]float64, 4)
	for index, part := range parts {
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		parsed[index] = value
	}
	return parsed[0]*86400 + parsed[1]*3600 + parsed[2]*60 + parsed[3]
}

func splitNonEmpty(value, separator string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, separator)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func negativeDBM(value float64) float64 {
	if value > 0 {
		return -value
	}
	return value
}

func accessName(value int) string {
	switch value {
	case 0:
		return "wired"
	case 1:
		return "wireless"
	default:
		return "unknown"
	}
}

func nameFor(values map[int]string, value int) string {
	if name, ok := values[value]; ok {
		return name
	}
	return "unknown"
}
