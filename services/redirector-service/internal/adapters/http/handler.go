package http

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/arminaray/url_shortener/pkg/httpx"
	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

type Handler struct {
	service *application.RedirectService
	metrics *httpx.Metrics
}

func NewHandler(service *application.RedirectService) *Handler {
	return NewHandlerWithMetrics(service, nil)
}

func NewHandlerWithMetrics(service *application.RedirectService, metrics *httpx.Metrics) *Handler {
	return &Handler{service: service, metrics: metrics}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", h.instrument("/healthz", h.handleHealth))
	mux.Handle("GET /readyz", h.instrument("/readyz", h.handleReady))
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.Handle("GET /{alias}", h.instrument("/{alias}", h.handleRedirect))
	return httpx.RequestLogging(mux)
}

func (h *Handler) instrument(route string, fn http.HandlerFunc) http.Handler {
	if h.metrics == nil {
		return fn
	}
	return h.metrics.InstrumentFunc(route, fn)
}

func (h *Handler) handleRedirect(w http.ResponseWriter, r *http.Request) {
	alias := strings.ToLower(strings.TrimSpace(r.PathValue("alias")))
	if alias == "" {
		http.Error(w, "alias is required", http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.Resolve(r.Context(), alias)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		case errors.Is(err, application.ErrExpired):
			http.Error(w, "gone", http.StatusGone)
		case errors.Is(err, application.ErrInactive):
			http.Error(w, "inactive", http.StatusForbidden)
		default:
			log.Printf("redirect error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}
