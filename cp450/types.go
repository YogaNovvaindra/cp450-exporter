package cp450

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Number accepts the numeric and quoted-numeric values returned by different
// CP450 pages and firmware modes.
type Number float64

func (n *Number) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || bytes.Equal(data, []byte(`""`)) {
		*n = 0
		return nil
	}

	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse quoted number %q: %w", value, err)
		}
		*n = Number(parsed)
		return nil
	}

	parsed, err := strconv.ParseFloat(string(data), 64)
	if err != nil {
		return fmt.Errorf("parse number %q: %w", string(data), err)
	}
	*n = Number(parsed)
	return nil
}

func (n Number) Float64() float64 { return float64(n) }
func (n Number) Int() int         { return int(n) }

// Status is the subset of getSysStatusCfg used by the exporter. Secret fields
// intentionally have no destination and are discarded during decoding.
type Status struct {
	OperationMode Number `json:"operationMode"`
	UpTime        string `json:"upTime"`
	FMVersion     string `json:"fmVersion"`
	BuildTime     string `json:"buildTime"`
	ProductName   string `json:"productName"`
	HardModel     string `json:"hardModel"`

	LANIP      string `json:"lanIp"`
	LANMask    string `json:"lanMask"`
	LANMAC     string `json:"lanMac"`
	LANRx      Number `json:"lanRx"`
	LANTx      Number `json:"lanTx"`
	DHCPServer Number `json:"dhcpServer"`

	WANConnStatus string `json:"wanConnStatus"`
	WANConnTime   string `json:"wanConnTime"`
	WANMode       Number `json:"wanMode"`
	WANIP         string `json:"wanIp"`
	WANMask       string `json:"wanMask"`
	WANGateway    string `json:"wanGw"`
	WANMAC        string `json:"wanMac"`
	WANRx         Number `json:"wanRx"`
	WANTx         Number `json:"wanTx"`

	WiFiOff       Number `json:"wifiOff"`
	WiFiOff2      Number `json:"wifiOff2"`
	WiFiOff3      Number `json:"wifiOff3"`
	Band          Number `json:"band"`
	Channel       Number `json:"channel"`
	AutoChannel   Number `json:"autoChannel"`
	WLANRx        Number `json:"wlanRx"`
	WLANTx        Number `json:"wlanTx"`
	SSID1         string `json:"ssid1"`
	SSID2         string `json:"ssid2"`
	SSID3         string `json:"ssid3"`
	BSSID1        string `json:"bssid1"`
	BSSID2        string `json:"bssid2"`
	BSSID3        string `json:"bssid3"`
	StationNum1   Number `json:"staNum1"`
	StationNum2   Number `json:"staNum2"`
	StationNum3   Number `json:"staNum3"`
	AuthMode      string `json:"authMode"`
	Encryption    string `json:"encrypType"`
	PortLinkState string `json:"portLinkStatus"`

	APClientEnable Number `json:"apcliEnable"`
	APClientStatus string `json:"apcliStatus"`
	APClientSSID   string `json:"apcliSsid"`
	APClientBSSID  string `json:"apcliBssid"`
	APClientRSSI   Number `json:"apcliRssi"`
}

type Device struct {
	Index Number `json:"idx"`
	IP    string `json:"ip"`
	MAC   string `json:"mac"`
	Type  Number `json:"type"`
}

type Station struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Mode      string `json:"mode"`
	Bandwidth string `json:"bw"`
	DBM       Number `json:"dbm"`
	RSSI      Number `json:"rssi"`
	Time      Number `json:"time"`
}

type Snapshot struct {
	Status   Status
	Devices  []Device
	Stations []Station
}
