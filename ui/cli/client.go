// Package cli implements dietctl, a read-plus-whitelisted-write command line
// client for the Diet Tracker API. All write restrictions are enforced here so
// the coach agent cannot rewrite historical data.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// defaultBaseURL is the production deployment. Override with DIET_API_URL.
	defaultBaseURL = "https://diet-tracker-v2.fly.dev"

	cookieFileName = "dietctl-cookies.json"
	requestTimeout = 60 * time.Second
	maxResponse    = 64 << 20
)

// ErrSessionExpired is returned when the access cookie is rejected and the
// refresh attempt also fails. The message deliberately tells the operator how
// to recover.
var ErrSessionExpired = errors.New("session expired or not logged in; run `dietctl login`")

// Config holds the resolved endpoint and on-disk session location.
type Config struct {
	BaseURL    string
	SessionDir string
	CookieFile string
}

// LoadConfig resolves the API endpoint and session directory from the
// environment, mirroring ui/tui/config.
func LoadConfig() (*Config, error) {
	baseURL := os.Getenv("DIET_API_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid DIET_API_URL: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "diet-tracker-v2")
	return &Config{
		BaseURL:    baseURL,
		SessionDir: dir,
		CookieFile: filepath.Join(dir, cookieFileName),
	}, nil
}

// Client is an HTTP client with cookie persistence and one automatic refresh
// retry on 401.
type Client struct {
	baseURL    string
	cookieFile string
	http       *http.Client
	jar        *CookieJar
}

// NewClient builds a client and loads any persisted session cookies.
func NewClient() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.SessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}
	// Make sure an existing, overly-permissive directory is tightened.
	_ = os.Chmod(cfg.SessionDir, 0o700)

	jar := NewCookieJar(cfg.CookieFile)
	if err := jar.Load(); err != nil {
		return nil, fmt.Errorf("load session cookies: %w", err)
	}

	return &Client{
		baseURL:    cfg.BaseURL,
		cookieFile: cfg.CookieFile,
		jar:        jar,
		http: &http.Client{
			Timeout: requestTimeout,
			Jar:     jar,
		},
	}, nil
}

// BaseURL exposes the resolved endpoint (used by help/tests).
func (c *Client) BaseURL() string { return c.baseURL }

// SaveCookies persists the current jar, creating the file with mode 0600.
func (c *Client) SaveCookies() error { return c.jar.Save() }

// ClearSession removes all in-memory and on-disk cookies.
func (c *Client) ClearSession() error {
	c.jar.Clear()
	if err := os.Remove(c.cookieFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Response is a fully-read HTTP response.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// APIError represents a non-2xx response. The API always uses {"error":"..."}.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Err converts a non-2xx response into an APIError.
func (r *Response) Err() error {
	if r.StatusCode >= 200 && r.StatusCode < 300 {
		return nil
	}
	return &APIError{StatusCode: r.StatusCode, Message: apiMessage(r)}
}

func apiMessage(r *Response) string {
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(r.Body, &payload); err == nil && payload.Error != "" {
		return payload.Error
	}
	if text := strings.TrimSpace(string(r.Body)); text != "" {
		return text
	}
	return http.StatusText(r.StatusCode)
}

// Get performs an authenticated GET.
func (c *Client) Get(path string, query url.Values) (*Response, error) {
	return c.Do(http.MethodGet, path, query, nil, nil)
}

// Post performs an authenticated POST. Authentication endpoints get no refresh
// retry.
func (c *Client) Post(path string, query url.Values, body any) (*Response, error) {
	return c.Do(http.MethodPost, path, query, body, nil)
}

// Do performs a request, retrying once through /v1/auth/refresh after a 401.
// extraHeaders is optional and may be nil.
func (c *Client) Do(method, path string, query url.Values, body any, extraHeaders map[string]string) (*Response, error) {
	allowRefresh := path != "/v1/auth/login" &&
		path != "/v1/auth/refresh" &&
		path != "/v1/auth/logout"

	res, err := c.doOnce(method, path, query, body, extraHeaders)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusUnauthorized || !allowRefresh {
		return res, nil
	}

	if err := c.refresh(); err != nil {
		return nil, ErrSessionExpired
	}
	res, err = c.doOnce(method, path, query, body, extraHeaders)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, ErrSessionExpired
	}
	return res, nil
}

// refresh exchanges the refresh cookie for a new access cookie. The endpoint
// takes no body.
func (c *Client) refresh() error {
	res, err := c.doOnce(http.MethodPost, "/v1/auth/refresh", nil, nil, nil)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ErrSessionExpired
	}
	return nil
}

func (c *Client) doOnce(method, path string, query url.Values, body any, extraHeaders map[string]string) (*Response, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Persist any Set-Cookie changes (login, refresh, logout).
	_ = c.jar.Save()

	return &Response{StatusCode: resp.StatusCode, Header: resp.Header.Clone(), Body: data}, nil
}
