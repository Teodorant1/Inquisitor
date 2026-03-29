package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// withRequestID middleware adds a unique request ID to each request
func (s *Server) withRequestID(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Generate request ID if not present
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Store in header for logging
		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)

		fn(w, r)
	}
}

// generateRequestID creates a unique request identifier
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req_error"
	}
	return "req_" + hex.EncodeToString(b)
}

// getRequestID retrieves the request ID from the request
func getRequestID(r *http.Request) string {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = "unknown"
	}
	return requestID
}

// withRequestIDAndAuth combines request ID and auth middleware
func (s *Server) withRequestIDAndAuth(fn http.HandlerFunc) http.HandlerFunc {
	return s.withRequestID(s.withAuth(fn))
}

// withRequestIDAuthOrigin combines all three middlewares
func (s *Server) withRequestIDAuthOrigin(fn http.HandlerFunc) http.HandlerFunc {
	return s.withRequestID(s.withOriginValidation(s.withAuth(fn)))
}
