package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"Inquisitor/config"
	"Inquisitor/db"
)

// Server represents the API server
type Server struct {
	mux     *http.ServeMux
	config  *config.Config
	worker  *JobWorker
	cleanup *Cleanup
	httpSrv *http.Server
}

// NewServer creates a new API server
func NewServer(cfg *config.Config) *Server {
	return &Server{
		mux:    http.NewServeMux(),
		config: cfg,
	}
}
// EnqueueJob hands a job directly to the worker pool without waiting for the DB ticker
func (s *Server) EnqueueJob(job *db.Job) {
    if s.worker != nil {
        s.worker.Submit(job)
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
	s.mux.HandleFunc("/generate-pdf", s.withRequestIDAuthOrigin(s.downloadPDFHandler))
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

// Start starts the HTTP server and blocks until shutdown
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.APIHost, s.config.APIPort)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("Starting API server on http://%s", addr)
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully shuts down the server with a timeout
func (s *Server) Shutdown() error {
	if s.httpSrv == nil {
		return nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	log.Println("Shutting down API server...")
	return s.httpSrv.Shutdown(ctx)
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
