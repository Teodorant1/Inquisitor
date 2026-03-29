package api

import (
	"fmt"
	"log"
	"net/http"

	"Inquisitor/config"
)

// Server represents the API server
type Server struct {
	mux     *http.ServeMux
	config  *config.Config
	worker  *JobWorker
	cleanup *Cleanup
}

// NewServer creates a new API server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		mux:    http.NewServeMux(),
		config: cfg,
	}
}

// RegisterRoutes registers all API endpoints
func (s *Server) RegisterRoutes() {
	// Health check endpoint (no auth required, but includes request ID)
	s.mux.HandleFunc("/health", s.withRequestID(s.healthHandler))

	// Analysis endpoint (requires auth + origin validation + request ID)
	s.mux.HandleFunc("/analyze", s.withRequestIDAuthOrigin(s.analyzeHandler))

	// Results endpoint (requires auth + request ID)
	s.mux.HandleFunc("/results", s.withRequestIDAndAuth(s.resultsHandler))

	// Job status endpoint (requires auth + origin validation + request ID)
	s.mux.HandleFunc("/job-status/", s.withRequestIDAuthOrigin(s.jobStatusHandler))

	// PDF generation endpoint (requires auth + origin validation + request ID)
	s.mux.HandleFunc("/generate-pdf", s.withRequestIDAuthOrigin(s.generatePDFHandler))
}

// InitializeCleanup starts the background cleanup routine
func (s *Server) InitializeCleanup() {
	s.cleanup = NewCleanup(s.config)
	s.cleanup.Start()
}

// InitializeWorker starts the background job processor
func (s *Server) InitializeWorker(numWorkers int) {
	s.worker = NewJobWorker(s.config.OpenAIKey, numWorkers)
	s.worker.Start()
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.APIHost, s.config.APIPort)
	log.Printf("Starting API server on http://%s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// Health check handler
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// withAuth middleware wraps a handler with API key authentication
func (s *Server) withAuth(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
			return
		}

		// Validate API key
		validatedKey, err := validateAPIKey(apiKey)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		// Add API key to request context for use in handler
		r.Header.Set("X-Username", validatedKey.Username)
		r.Header.Set("X-APIKeyID", fmt.Sprintf("%d", validatedKey.ID))

		fn(w, r)
	}
}
