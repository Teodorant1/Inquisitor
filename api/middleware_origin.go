package api

import (
	"log"
	"net/http"
	"strings"
)

// withOriginValidation middleware validates that requests come from allowed domains
func (s *Server) withOriginValidation(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow localhost in dev mode
		if s.config.Env == "dev" {
			// In dev, allow any localhost origin
			origin := r.Header.Get("Origin")
			if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") || origin == "" {
				fn(w, r)
				return
			}
		}

		// In production, check against configured frontend domain
		origin := r.Header.Get("Origin")
		if s.config.FrontendDomain != "" && origin == s.config.FrontendDomain {
			fn(w, r)
			return
		}

		// Also allow direct server access (no Origin header)
		if origin == "" {
			fn(w, r)
			return
		}

		// Reject request from invalid origin
		log.Printf("Rejected request from invalid origin: %s", origin)
		http.Error(w, `{"error":"invalid origin"}`, http.StatusForbidden)
	}
}

// withAuthAndOrigin combines both auth and origin validation
func (s *Server) withAuthAndOrigin(fn http.HandlerFunc) http.HandlerFunc {
	return s.withOriginValidation(s.withAuth(fn))
}
