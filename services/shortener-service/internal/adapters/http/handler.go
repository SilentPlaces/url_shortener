package http

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/arminaray/url_shortener/pkg/httpx"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/application"
	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain"
)

const maxRequestBodyBytes = 64 << 10

type Handler struct {
	useCase *application.ShortenerUseCase
	metrics *httpx.Metrics
}

func NewHandler(useCase *application.ShortenerUseCase) *Handler {
	return NewHandlerWithMetrics(useCase, nil)
}

func NewHandlerWithMetrics(useCase *application.ShortenerUseCase, metrics *httpx.Metrics) *Handler {
	return &Handler{useCase: useCase, metrics: metrics}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/urls", h.instrument("/api/v1/urls", h.handleShortenURL))
	mux.Handle("GET /api/v1/urls/{alias}", h.instrument("/api/v1/urls/{alias}", h.handleGetURL))
	mux.Handle("GET /healthz", h.instrument("/healthz", h.handleHealth))
	mux.Handle("GET /readyz", h.instrument("/readyz", h.handleReady))
	mux.Handle("GET /metrics", promhttp.Handler())
	return httpx.RequestLogging(mux)
}

func (h *Handler) instrument(route string, fn http.HandlerFunc) http.Handler {
	if h.metrics == nil {
		return fn
	}
	return h.metrics.InstrumentFunc(route, fn)
}

func (h *Handler) handleShortenURL(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req application.ShortenURLRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			writeError(w, http.StatusBadRequest, "request body is empty", err)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	req.OriginalURL = strings.TrimSpace(req.OriginalURL)
	req.CustomAlias = strings.TrimSpace(req.CustomAlias)
	if req.OriginalURL == "" {
		writeError(w, http.StatusBadRequest, "original_url is required", domain.ErrOriginalURLRequired)
		return
	}

	result, err := h.useCase.ShortenURL(r.Context(), &req)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) handleGetURL(w http.ResponseWriter, r *http.Request) {
	alias := strings.TrimSpace(r.PathValue("alias"))
	if alias == "" {
		writeError(w, http.StatusBadRequest, "alias is required", domain.ErrAliasRequired)
		return
	}

	result, err := h.useCase.GetURL(r.Context(), alias)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeDomainError(w http.ResponseWriter, err error) {
	var domainErr *domain.Error
	if errors.As(err, &domainErr) {
		switch domainErr.Code {
		case "INVALID_URL", "INVALID_ALIAS", "ALIAS_REQUIRED", "URL_REQUIRED", "RESERVED_ALIAS":
			writeError(w, http.StatusBadRequest, domainErr.Message, domainErr)
		case "ALIAS_TAKEN":
			writeError(w, http.StatusConflict, domainErr.Message, domainErr)
		case "URL_NOT_FOUND":
			writeError(w, http.StatusNotFound, domainErr.Message, domainErr)
		case "EXPIRED":
			writeError(w, http.StatusGone, domainErr.Message, domainErr)
		case "URL_INACTIVE":
			writeError(w, http.StatusForbidden, domainErr.Message, domainErr)
		default:
			writeError(w, http.StatusInternalServerError, "internal error", domainErr)
		}
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error", err)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	writeJSON(w, status, map[string]interface{}{
		"message":   message,
		"error":     err.Error(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
