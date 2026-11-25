package cp450

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

type fakeDeviceServer struct {
	server *httptest.Server

	mu           sync.Mutex
	session      string
	loginCount   int
	validUser    string
	validPass    string
	forceExpired bool
}

func newFakeDeviceServer(t *testing.T) *fakeDeviceServer {
	t.Helper()
	device := &fakeDeviceServer{
		session:   "session-1",
		validUser: "admin",
		validPass: "test-password",
	}
	device.server = httptest.NewServer(http.HandlerFunc(device.serveHTTP))
	t.Cleanup(device.server.Close)
	return device
}

func (d *fakeDeviceServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/cgi-bin/cstecgi.cgi":
		if r.URL.Query().Get("action") == "login" {
			d.login(w, r)
			return
		}
		d.api(w, r)
	case "/formLoginAuth.htm":
		d.mu.Lock()
		session := d.session
		d.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "SESSION_ID", Value: session, Path: "/"})
		http.Redirect(w, r, "/home.asp", http.StatusFound)
	case "/home.asp":
		w.WriteHeader(http.StatusOK)
	case "/login.asp":
		_, _ = w.Write([]byte(`<form id="loginfrm"></form>`))
	default:
		http.NotFound(w, r)
	}
}

func (d *fakeDeviceServer) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.Form.Get("username") != d.validUser || r.Form.Get("password") != d.validPass {
		http.Redirect(w, r, "/login.asp", http.StatusFound)
		return
	}

	d.mu.Lock()
	d.loginCount++
	d.mu.Unlock()
	location := "/formLoginAuth.htm?authCode=1&userName=" + url.QueryEscape(d.validUser) +
		"&password=" + url.QueryEscape(d.validPass) + "&goURL=home.asp&action=login"
	http.Redirect(w, r, location, http.StatusFound)
}

func (d *fakeDeviceServer) api(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	session := d.session
	forceExpired := d.forceExpired
	if d.forceExpired {
		d.forceExpired = false
		d.session = fmt.Sprintf("session-%d", d.loginCount+1)
	}
	d.mu.Unlock()

	cookie, err := r.Cookie("SESSION_ID")
	if err != nil || cookie.Value != session || forceExpired {
		http.Redirect(w, r, "/login.asp", http.StatusFound)
		return
	}

	var payload struct {
		Topic string `json:"topicurl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	switch payload.Topic {
	case "setting/getSysStatusCfg":
		_, _ = w.Write([]byte(`{"productName":"CP450","fmVersion":"V4.1.0cu.747","operationMode":1,"upTime":"0;1;2;3","lanRx":10,"lanTx":20}`))
	case "setting/getDeviceInfo":
		_, _ = w.Write([]byte(`[{"idx":1,"ip":"10.1.1.10","mac":"aa:bb:cc:dd:ee:ff","type":1}]`))
	case "setting/getWiFiStaInfo":
		_, _ = w.Write([]byte(`[{"ip":"10.1.1.10","mac":"aa:bb:cc:dd:ee:ff","mode":"11ac","bw":"40M","dbm":60,"rssi":90,"time":"300"}]`))
	default:
		http.Error(w, "unknown topic", http.StatusBadRequest)
	}
}

func TestClientSnapshot(t *testing.T) {
	t.Parallel()
	device := newFakeDeviceServer(t)
	client, err := NewClient(device.server.URL, "admin", "test-password", 2*time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Status.ProductName != "CP450" || len(snapshot.Devices) != 1 || len(snapshot.Stations) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Stations[0].Time.Int() != 300 {
		t.Fatalf("station time = %d, want 300", snapshot.Stations[0].Time.Int())
	}
}

func TestClientRelogsAfterSessionExpiry(t *testing.T) {
	t.Parallel()
	device := newFakeDeviceServer(t)
	client, err := NewClient(device.server.URL, "admin", "test-password", 2*time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.Snapshot(context.Background()); err != nil {
		t.Fatalf("first Snapshot() error = %v", err)
	}
	device.mu.Lock()
	device.forceExpired = true
	device.mu.Unlock()
	if _, err := client.Snapshot(context.Background()); err != nil {
		t.Fatalf("second Snapshot() error = %v", err)
	}

	device.mu.Lock()
	loginCount := device.loginCount
	device.mu.Unlock()
	if loginCount != 2 {
		t.Fatalf("login count = %d, want 2", loginCount)
	}
}

func TestClientRejectsWrongPassword(t *testing.T) {
	t.Parallel()
	device := newFakeDeviceServer(t)
	client, err := NewClient(device.server.URL, "admin", "wrong", 2*time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Snapshot(context.Background()); err == nil {
		t.Fatal("Snapshot() error = nil, want authentication error")
	}
}
