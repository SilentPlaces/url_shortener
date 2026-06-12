package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestNewRequestID_Hex32Chars(t *testing.T) {
	id := NewRequestID()
	if len(id) != 32 {
		t.Fatalf("expected 32-char hex id, got %d: %q", len(id), id)
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Fatalf("expected hex char, got %q in %q", c, id)
		}
	}
}

func TestRequestLogging_AssignsRequestIDWhenMissing(t *testing.T) {
	var seenID string
	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenID = RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(res, req)

	if seenID == "" {
		t.Fatalf("expected handler to see a request ID in context")
	}
	if got := res.Header().Get(HeaderRequestID); got != seenID {
		t.Fatalf("expected response header to echo request id, got %q vs %q", got, seenID)
	}
}

func TestRequestLogging_PropagatesProvidedRequestID(t *testing.T) {
	handler := RequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "abc-123" {
			t.Errorf("expected propagated request id, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderRequestID, "abc-123")
	handler.ServeHTTP(res, req)
}

func TestMetricsInstrument_RecordsLatencyAndStatus(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "test", Subsystem: "http", Name: "requests_total",
		}, []string{"method", "route", "status"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "test", Subsystem: "http", Name: "request_duration_seconds",
		}, []string{"method", "route"}),
	}
	reg.MustRegister(m.requests, m.latency)

	handler := m.Instrument("/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(context.Background())
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", res.Code)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var sawCounter bool
	for _, mf := range mfs {
		if mf.GetName() == "test_http_requests_total" {
			for _, m := range mf.GetMetric() {
				labels := map[string]string{}
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				if labels["status"] == "418" && labels["route"] == "/test" {
					sawCounter = true
				}
			}
		}
	}
	if !sawCounter {
		t.Fatalf("expected counter for 418/test, found none")
	}

	metricsHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	res2 := httptest.NewRecorder()
	metricsHandler.ServeHTTP(res2, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if res2.Code != http.StatusOK {
		t.Fatalf("metrics endpoint returned %d", res2.Code)
	}
	body, _ := io.ReadAll(res2.Body)
	if !strings.Contains(string(body), "test_http_requests_total") {
		t.Fatalf("metrics output missing test counter: %s", body)
	}
}
