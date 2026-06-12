package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const HeaderRequestID = "X-Request-ID"

type ctxKey int

const requestIDKey ctxKey = iota

func NewRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf[:])
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status = code
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wrote {
		s.status = http.StatusOK
		s.wrote = true
	}
	return s.ResponseWriter.Write(b)
}

func RequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(HeaderRequestID)
		if requestID == "" {
			requestID = NewRequestID()
		}

		w.Header().Set(HeaderRequestID, requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d",
			requestID, r.Method, r.URL.Path, rec.status, time.Since(start).Milliseconds())
	})
}

type Metrics struct {
	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec
}

func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total HTTP requests grouped by method, route, and status.",
		}, []string{"method", "route", "status"}),
		latency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds, grouped by method and route.",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 14),
		}, []string{"method", "route"}),
	}
}

func (m *Metrics) Instrument(route string, next http.Handler) http.Handler {
	tracer := otel.Tracer("github.com/arminaray/url_shortener/pkg/httpx")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), r.Method+" "+route,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", route),
				attribute.String("http.target", r.URL.Path),
			),
		)
		defer span.End()

		if rid := RequestIDFromContext(ctx); rid != "" {
			span.SetAttributes(attribute.String("request.id", rid))
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		elapsed := time.Since(start).Seconds()

		span.SetAttributes(attribute.Int("http.status_code", rec.status))
		m.requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		m.latency.WithLabelValues(r.Method, route).Observe(elapsed)
	})
}

func (m *Metrics) InstrumentFunc(route string, h http.HandlerFunc) http.Handler {
	return m.Instrument(route, h)
}
