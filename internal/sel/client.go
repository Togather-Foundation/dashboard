// Package sel is a typed client for a SEL node's HTTP API.
//
// It deliberately separates two tiers:
//
//   - Public methods (Events, Places) need no credential and hit /api/v1/*.
//   - Admin methods (ScraperSources, DailyUsage) attach a bearer JWT that this
//     package obtains and refreshes itself.
//
// Per docs/rfc-001-auth-posture.md the admin token never leaves the server: it
// is held here, in process, and is not exposed to the browser in any form.
package sel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	node string // host only, e.g. "staging.toronto.togather.foundation"
	http *http.Client

	// Admin credentials. Empty when the dashboard runs in public-only mode.
	email    string
	password string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

type Option func(*Client)

// WithAdminCredentials enables the admin tier. Without it, admin methods
// return ErrNoAdminCredentials and the dashboard degrades to public panels.
func WithAdminCredentials(email, password string) Option {
	return func(c *Client) {
		c.email, c.password = email, password
	}
}

func New(node string, opts ...Option) *Client {
	c := &Client{
		node: strings.TrimPrefix(strings.TrimPrefix(node, "https://"), "http://"),
		http: &http.Client{Timeout: 20 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

type Error struct {
	Status int
	Body   string
	URL    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("sel: %s returned %d: %s", e.URL, e.Status, truncate(e.Body, 240))
}

var ErrNoAdminCredentials = fmt.Errorf("sel: no admin credentials configured")

func (c *Client) HasAdmin() bool { return c.email != "" && c.password != "" }

func (c *Client) do(ctx context.Context, path string, q url.Values, auth bool, out any) error {
	u := url.URL{Scheme: "https", Host: c.node, Path: path}
	if q != nil {
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	if auth {
		tok, err := c.adminToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sel: %s: %w", u.String(), err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode != http.StatusOK {
		return &Error{Status: res.StatusCode, Body: string(body), URL: u.String()}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("sel: decoding %s: %w", u.String(), err)
	}
	return nil
}

// adminToken returns a cached JWT, logging in when absent or near expiry.
func (c *Client) adminToken(ctx context.Context) (string, error) {
	if !c.HasAdmin() {
		return "", ErrNoAdminCredentials
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Refresh a minute early so a token cannot expire mid-request.
	if c.token != "" && time.Now().Before(c.tokenExp.Add(-time.Minute)) {
		return c.token, nil
	}

	payload, err := json.Marshal(loginRequest{Email: c.email, Password: c.password})
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "https", Host: c.node, Path: "/admin/login"}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("sel: admin login: %w", err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		// Deliberately does not echo the response body — it is an auth failure
		// and the body may carry detail that should not reach a log.
		return "", &Error{Status: res.StatusCode, URL: u.String(), Body: "admin login rejected"}
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("sel: decoding admin login: %w", err)
	}
	if lr.Token == "" {
		return "", fmt.Errorf("sel: admin login returned an empty token")
	}

	c.token = lr.Token
	c.tokenExp = lr.ExpiresAt
	if c.tokenExp.IsZero() {
		// Spec allows expiresAt to be absent; re-login hourly rather than never.
		c.tokenExp = time.Now().Add(time.Hour)
	}
	return c.token, nil
}

// ---------- public tier ----------

// Events lists events. Note the API expects camelCase parameter names; the
// snake_case aliases documented for the MCP tools are rejected here (the node
// reports them in the response's `warnings` array rather than failing).
func (c *Client) Events(ctx context.Context, q url.Values) (*EventListResponse, error) {
	var out EventListResponse
	if err := c.do(ctx, "/api/v1/events", q, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Places(ctx context.Context, q url.Values) (*PlaceListResponse, error) {
	var out PlaceListResponse
	if err := c.do(ctx, "/api/v1/places", q, false, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------- admin tier ----------

// ScraperSources reports per-source ingestion health.
//
// UNVERIFIED: written from the OpenAPI schema. No admin credentials were
// available when this was authored, so the response shape has not been
// confirmed against a live node.
func (c *Client) ScraperSources(ctx context.Context) (*ScraperSourcesResponse, error) {
	var out ScraperSourcesResponse
	if err := c.do(ctx, "/admin/scraper/sources", nil, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DailyUsage reports request and error counts per day.
//
// UNVERIFIED: see ScraperSources.
func (c *Client) DailyUsage(ctx context.Context, from, to string) (*DailyUsageResponse, error) {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	var out DailyUsageResponse
	if err := c.do(ctx, "/admin/reports/daily-usage", q, true, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
