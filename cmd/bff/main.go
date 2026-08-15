// Command bff is the dashboard's backend-for-frontend.
//
// It exists for one reason: to hold the SEL admin credential server-side so it
// never reaches a browser. See docs/rfc-001-auth-posture.md.
//
// It proxies a fixed, enumerated set of read-only upstream operations. There is
// deliberately no general-purpose passthrough — adding an endpoint is an
// explicit edit here, so the privileged surface stays small enough to audit.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Togather-Foundation/dashboard/internal/coverage"
	"github.com/Togather-Foundation/dashboard/internal/sel"
)

type config struct {
	addr     string
	node     string
	email    string
	password string
	webDir   string
	tz       string
}

func loadConfig() config {
	c := config{
		addr:     env("DASHBOARD_ADDR", "127.0.0.1:8080"),
		node:     env("SEL_NODE", "staging.toronto.togather.foundation"),
		email:    os.Getenv("SEL_ADMIN_EMAIL"),
		password: os.Getenv("SEL_ADMIN_PASSWORD"),
		webDir:   env("DASHBOARD_WEB_DIR", "web"),
		tz:       env("DASHBOARD_TZ", "America/Toronto"),
	}
	flag.StringVar(&c.addr, "addr", c.addr, "listen address")
	flag.StringVar(&c.node, "node", c.node, "SEL node host")
	flag.StringVar(&c.webDir, "web", c.webDir, "static asset directory")
	flag.Parse()
	return c
}

func main() {
	cfg := loadConfig()
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("dashboard: ")

	// The MVP has no browser-facing authentication. Until it does, refuse to
	// listen anywhere but loopback — otherwise this would publish an
	// unauthenticated window onto admin data. See RFC 001, "Open questions".
	if err := guardBinding(cfg.addr); err != nil {
		log.Fatalf("refusing to start: %v", err)
	}

	loc, err := time.LoadLocation(cfg.tz)
	if err != nil {
		log.Fatalf("unknown timezone %q: %v", cfg.tz, err)
	}

	var opts []sel.Option
	if cfg.email != "" && cfg.password != "" {
		opts = append(opts, sel.WithAdminCredentials(cfg.email, cfg.password))
	}
	client := sel.New(cfg.node, opts...)

	s := &server{client: client, loc: loc, node: cfg.node}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/coverage", s.handleCoverage)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/usage", s.handleUsage)
	mux.HandleFunc("GET /api/provenance", s.handleProvenance)
	mux.Handle("GET /", http.FileServer(http.Dir(cfg.webDir)))

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           logging(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("node=%s admin=%v tz=%s", cfg.node, client.HasAdmin(), cfg.tz)
	if !client.HasAdmin() {
		log.Printf("no SEL_ADMIN_EMAIL/SEL_ADMIN_PASSWORD set — admin panels will report as unavailable")
	}
	log.Printf("listening on http://%s", cfg.addr)
	log.Fatal(srv.ListenAndServe())
}

// guardBinding blocks non-loopback listeners while browser auth is unimplemented.
func guardBinding(addr string) error {
	if os.Getenv("DASHBOARD_ALLOW_PUBLIC_BIND") == "i-understand" {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("cannot parse addr %q: %w", addr, err)
	}
	if host == "" {
		return errors.New("addr must specify a host; use 127.0.0.1 for local use")
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return fmt.Errorf("addr %q is not loopback and this build has no browser authentication; "+
		"implement sessions (RFC 001) or set DASHBOARD_ALLOW_PUBLIC_BIND=i-understand", addr)
}

type server struct {
	client *sel.Client
	loc    *time.Location
	node   string
}

// panelResponse wraps every panel so the frontend can render a partial page.
// A failing admin panel must not blank the public ones.
type panelResponse struct {
	OK       bool   `json:"ok"`
	Data     any    `json:"data,omitempty"`
	Error    string `json:"error,omitempty"`
	Reason   string `json:"reason,omitempty"` // machine-readable: no_credentials, upstream, blocked
	Upstream string `json:"upstream,omitempty"`
}

func (s *server) writePanel(w http.ResponseWriter, p panelResponse, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writePanel(w, panelResponse{OK: true, Data: map[string]any{
		"node":       s.node,
		"adminReady": s.client.HasAdmin(),
		"time":       time.Now().UTC(),
	}}, http.StatusOK)
}

func (s *server) handleCoverage(w http.ResponseWriter, r *http.Request) {
	horizon := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 180 {
			horizon = n
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	rep, err := coverage.Build(ctx, s.client, horizon, s.loc)
	if err != nil {
		s.writePanel(w, panelResponse{
			OK: false, Reason: "upstream",
			Error:    "Could not read events from the node.",
			Upstream: err.Error(),
		}, http.StatusBadGateway)
		return
	}
	s.writePanel(w, panelResponse{OK: true, Data: rep}, http.StatusOK)
}

func (s *server) handleSources(w http.ResponseWriter, r *http.Request) {
	if !s.client.HasAdmin() {
		s.writePanel(w, noCredentials("Source health"), http.StatusOK)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	res, err := s.client.ScraperSources(ctx)
	if err != nil {
		s.writePanel(w, upstreamFailure(err), http.StatusOK)
		return
	}
	s.writePanel(w, panelResponse{OK: true, Data: res.Items}, http.StatusOK)
}

func (s *server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if !s.client.HasAdmin() {
		s.writePanel(w, noCredentials("API usage"), http.StatusOK)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	to := time.Now().In(s.loc)
	from := to.AddDate(0, 0, -29)
	res, err := s.client.DailyUsage(ctx, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		s.writePanel(w, upstreamFailure(err), http.StatusOK)
		return
	}
	s.writePanel(w, panelResponse{OK: true, Data: res.Daily}, http.StatusOK)
}

// handleProvenance is a stub.
//
// The public API declares a sel:* provenance vocabulary (confidence,
// licenseStatus, sourceUrl, ingestedAt, scraperSource…) in the @context of
// every response and emits none of it — Togather-Foundation/server#22. Until
// that lands there is nothing to aggregate on the public tier, and the admin
// equivalents are not yet mapped. This reports the blockage rather than
// inventing numbers.
func (s *server) handleProvenance(w http.ResponseWriter, r *http.Request) {
	s.writePanel(w, panelResponse{
		OK:     false,
		Reason: "blocked",
		Error: "Provenance fields are declared in the API's @context but never emitted, " +
			"so there is nothing to report yet.",
		Upstream: "https://github.com/Togather-Foundation/server/issues/22",
	}, http.StatusOK)
}

func noCredentials(panel string) panelResponse {
	return panelResponse{
		OK:     false,
		Reason: "no_credentials",
		Error: panel + " needs SEL admin credentials. Set SEL_ADMIN_EMAIL and " +
			"SEL_ADMIN_PASSWORD; see .env.example.",
	}
}

func upstreamFailure(err error) panelResponse {
	var se *sel.Error
	msg := "The node rejected the request."
	if errors.As(err, &se) && se.Status == http.StatusUnauthorized {
		msg = "The node rejected these admin credentials."
	}
	return panelResponse{OK: false, Reason: "upstream", Error: msg, Upstream: err.Error()}
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
