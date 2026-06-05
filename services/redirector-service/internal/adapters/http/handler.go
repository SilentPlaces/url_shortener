package http

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/arminaray/url_shortener/services/redirector-service/internal/application"
)

type Handler struct {
	service *application.RedirectService
}

func NewHandler(service *application.RedirectService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("GET /readyz", h.handleReady)
	mux.HandleFunc("GET /{alias}", h.handleRedirect)
	return withRequestLogging(mux)
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

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}

		start := time.Now()
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
		log.Printf("request_id=%s method=%s path=%s duration_ms=%d",
			requestID, r.Method, r.URL.Path, time.Since(start).Milliseconds())
	})
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf[:])
}
