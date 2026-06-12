package http

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

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
