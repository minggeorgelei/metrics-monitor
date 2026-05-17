// Package http_snapshot exposes the latest value of every metric over
// HTTP, designed for a WebUI in pull mode. Every (name + tags) pair
// is overwritten in-place each time a new metric arrives, so the
// cache size is bounded by series cardinality rather than time.
//
// Endpoints:
//
//	GET /api/v1/metrics                  — full snapshot, grouped by metric name
//	GET /api/v1/metrics?names=cpu,mem    — full snapshot, filtered to a subset
//	GET /api/v1/metrics/{name}           — only one group (e.g. "cpu")
//	GET /healthz                         — { "status": "ok" }
//	GET /                                — minimal placeholder HTML
//
// The ?names= filter is the WebUI's batching path: panels at the
// same refresh interval share a single fetch by including the union
// of their needed metric names in the query string.
//
// This output composes cleanly with `file` and others — register it
// alongside in TOML and metrics flow to both.
package http_snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/minggeorgelei/metrics-monitor/source/core"
	"github.com/minggeorgelei/metrics-monitor/source/plugins/outputs"
)

// HTTPSnapshot maintains an in-memory map of the latest *Metric per
// (name, tags) pair and serves it over HTTP.
type HTTPSnapshot struct {
	Listen string `toml:"listen"` // e.g. ":8080" or "127.0.0.1:9100"

	// cache is read by HTTP handlers, written by Write — guarded by mu.
	cache map[string]*core.Metric
	mu    sync.RWMutex

	server *http.Server
}

func (*HTTPSnapshot) Name() string { return "http_snapshot" }

// Init runs after TOML decode. Fails loudly on misconfig so the agent
// won't start with a broken output.
func (h *HTTPSnapshot) Init() error {
	if h.Listen == "" {
		return errors.New("http_snapshot: listen address must be set (e.g. \":8080\")")
	}
	h.cache = make(map[string]*core.Metric)
	return nil
}

// Connect starts the HTTP server in a goroutine and returns once
// ListenAndServe has had a chance to fail synchronously (bad port,
// EACCES, ...). The 100ms window catches startup errors without
// blocking the agent indefinitely.
func (h *HTTPSnapshot) Connect() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/api/v1/metrics", h.handleAll)
	mux.HandleFunc("/api/v1/metrics/", h.handleByName)
	mux.HandleFunc("/", h.handleIndex)

	h.server = &http.Server{
		Addr:              h.Listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := h.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http_snapshot listen %s: %w", h.Listen, err)
		}
	case <-time.After(100 * time.Millisecond):
	}
	return nil
}

func (h *HTTPSnapshot) Close() error {
	if h.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.server.Shutdown(ctx)
}

// Write merges incoming metrics into the cache. Older values for the
// same series key are overwritten, so the cache holds exactly the
// most recent observation per series.
func (h *HTTPSnapshot) Write(metrics []*core.Metric) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range metrics {
		h.cache[seriesKey(m)] = m
	}
	return nil
}

// --- HTTP handlers ---------------------------------------------------

// snapshotResponse is the JSON shape served on /api/v1/metrics.
// Metrics grouped by name → UI can render one panel per name.
type snapshotResponse struct {
	Timestamp time.Time                 `json:"timestamp"`
	Metrics   map[string][]*core.Metric `json:"metrics"`
}

func (h *HTTPSnapshot) handleAll(w http.ResponseWriter, r *http.Request) {
	// Optional `?names=foo,bar` filter — when present, only those
	// metric groups are returned. When absent or empty, every group
	// in the cache is included.
	filterNames := parseNamesParam(r.URL.Query().Get("names"))
	writeJSON(w, http.StatusOK, h.buildSnapshot(filterNames))
}

func (h *HTTPSnapshot) handleByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
	name = strings.Trim(name, "/")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "metric name required"})
		return
	}
	snap := h.buildSnapshot(map[string]struct{}{name: {}})
	if len(snap.Metrics[name]) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":     "no metrics with that name",
			"available": h.knownNames(),
		})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// parseNamesParam unpacks a comma-separated metric name list from a
// query parameter. Returns nil for "no filter" (caller treats this
// as "return everything"); a non-nil empty set means "the user asked
// for nothing", which buildSnapshot will also honour by returning an
// empty Metrics map.
func parseNamesParam(raw string) map[string]struct{} {
	if raw == "" {
		return nil
	}
	out := make(map[string]struct{})
	// SplitSeq returns an iterator instead of allocating an intermediate
	// slice — slightly cheaper when the comma-separated list is large.
	for n := range strings.SplitSeq(raw, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			out[n] = struct{}{}
		}
	}
	return out
}

func (*HTTPSnapshot) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (*HTTPSnapshot) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<title>metrics-monitor</title>
<h1>metrics-monitor</h1>
<ul>
  <li><a href="/api/v1/metrics">/api/v1/metrics</a></li>
  <li><a href="/healthz">/healthz</a></li>
</ul>`))
}

// buildSnapshot collects cache entries under read lock. filterNames
// semantics:
//
//	nil          → include every metric in the cache (the legacy
//	               GET /api/v1/metrics path).
//	non-nil set  → include only metrics whose Name appears in the
//	               set. An empty set therefore returns an empty
//	               Metrics map — caller decides whether that's a
//	               404 or a legitimate "nothing matches".
func (h *HTTPSnapshot) buildSnapshot(filterNames map[string]struct{}) snapshotResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := snapshotResponse{
		Timestamp: time.Now().UTC(),
		Metrics:   make(map[string][]*core.Metric),
	}
	for _, m := range h.cache {
		if filterNames != nil {
			if _, ok := filterNames[m.Name]; !ok {
				continue
			}
		}
		out.Metrics[m.Name] = append(out.Metrics[m.Name], m)
	}
	return out
}

func (h *HTTPSnapshot) knownNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set := map[string]struct{}{}
	for _, m := range h.cache {
		set[m.Name] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- helpers --------------------------------------------------------

// seriesKey is a stable, order-independent identity for a (name, tags)
// pair so the cache stays at series cardinality regardless of how
// often metrics arrive.
func seriesKey(m *core.Metric) string {
	if len(m.Tags) == 0 {
		return m.Name
	}
	keys := make([]string, 0, len(m.Tags))
	for k := range m.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(m.Name)
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m.Tags[k])
	}
	return b.String()
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Dev convenience: WebUI on a separate Vite port needs CORS.
	// Tighten before any production exposure.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func init() {
	outputs.Add("http_snapshot", func() core.Output {
		// :9100 is the node_exporter convention; sensible default
		// for a host-metrics agent.
		return &HTTPSnapshot{Listen: ":9100"}
	})
}
