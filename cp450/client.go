package cp450

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

var errSessionExpired = errors.New("CP450 session expired")

const maxResponseSize = 4 << 20

// Scraper is implemented by a CP450 client and by test fakes.
type Scraper interface {
	Snapshot(context.Context) (Snapshot, error)
}

// Client handles the CP450 login redirect, session cookie, and read calls.
type Client struct {
	baseURL  *url.URL
	username string
	password string
	timeout  time.Duration

	mu         sync.Mutex
	jar        http.CookieJar
	httpClient *http.Client
	loggedIn   bool
}

func NewClient(baseURL, username, password string, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid CP450 URL %q", baseURL)
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("CP450 username and password are required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("CP450 timeout must be greater than zero")
	}

	client := &Client{
		baseURL:  parsed,
		username: username,
		password: password,
		timeout:  timeout,
	}
	if err := client.resetSession(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) Snapshot(ctx context.Context) (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loggedIn {
		if err := c.login(ctx); err != nil {
			return Snapshot{}, err
		}
	}

	var snapshot Snapshot
	if err := c.callWithRetry(ctx, "setting/getSysStatusCfg", &snapshot.Status); err != nil {
		return Snapshot{}, fmt.Errorf("read system status: %w", err)
	}
	if err := c.callWithRetry(ctx, "setting/getDeviceInfo", &snapshot.Devices); err != nil {
		return Snapshot{}, fmt.Errorf("read connected devices: %w", err)
	}
	if err := c.callWithRetry(ctx, "setting/getWiFiStaInfo", &snapshot.Stations); err != nil {
		return Snapshot{}, fmt.Errorf("read Wi-Fi stations: %w", err)
	}
	return snapshot, nil
}

func (c *Client) callWithRetry(ctx context.Context, topic string, destination any) error {
	if err := c.call(ctx, topic, destination); err == nil {
		return nil
	} else if !errors.Is(err, errSessionExpired) {
		return err
	}

	if err := c.resetSession(); err != nil {
		return err
	}
	if err := c.login(ctx); err != nil {
		return err
	}
	return c.call(ctx, topic, destination)
}

func (c *Client) login(ctx context.Context) error {
	form := url.Values{
		"username": {c.username},
		"password": {c.password},
	}
	endpoint := c.resolve("/cgi-bin/cstecgi.cgi") + "?action=login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create CP450 login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/json")
	req.Header.Set("User-Agent", "cp450-exporter")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("log in to CP450: %w", err)
	}
	_, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read CP450 login response: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close CP450 login response: %w", closeErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("CP450 login returned HTTP %d", resp.StatusCode)
	}

	for _, cookie := range c.jar.Cookies(c.baseURL) {
		if cookie.Name == "SESSION_ID" && cookie.Value != "" {
			c.loggedIn = true
			return nil
		}
	}
	return fmt.Errorf("CP450 login did not create a SESSION_ID cookie")
}

func (c *Client) call(ctx context.Context, topic string, destination any) error {
	payload, err := json.Marshal(map[string]string{"topicurl": topic})
	if err != nil {
		return fmt.Errorf("encode CP450 request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.resolve("/cgi-bin/cstecgi.cgi"),
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create CP450 API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,text/plain")
	req.Header.Set("User-Agent", "cp450-exporter")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call CP450 topic %s: %w", topic, err)
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read CP450 topic %s: %w", topic, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close CP450 topic %s response: %w", topic, closeErr)
	}
	if resp.Request != nil && resp.Request.URL.Path == "/login.asp" {
		c.loggedIn = false
		return errSessionExpired
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("CP450 topic %s returned HTTP %d", topic, resp.StatusCode)
	}
	if err := json.Unmarshal(body, destination); err != nil {
		if bytes.Contains(body, []byte("loginfrm")) || bytes.Contains(body, []byte("/login.asp")) {
			c.loggedIn = false
			return errSessionExpired
		}
		return fmt.Errorf("decode CP450 topic %s: %w", topic, err)
	}
	return nil
}

func (c *Client) resetSession() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("create CP450 cookie jar: %w", err)
	}
	c.jar = jar
	c.httpClient = &http.Client{
		Jar:     jar,
		Timeout: c.timeout,
	}
	c.loggedIn = false
	return nil
}

func (c *Client) resolve(path string) string {
	resolved := *c.baseURL
	resolved.Path = path
	resolved.RawPath = ""
	resolved.RawQuery = ""
	resolved.Fragment = ""
	return resolved.String()
}
