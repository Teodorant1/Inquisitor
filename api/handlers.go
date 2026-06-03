package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Inquisitor/db"
	"Inquisitor/printer"
)

// AnalyzeRequest represents the request for the /analyze endpoint
type AnalyzeRequest struct {
	PDFConfig *printer.PDFConfig `json:"pdf_config,omitempty"`
}

// AnalyzeResponse represents the response from the /analyze endpoint
type AnalyzeResponse struct {
	Status    string `json:"status"` // "accepted" or "error"
	ResultID  uint   `json:"result_id"`
	JobID     uint   `json:"job_id"`
	JobStatus string `json:"job_status"` // "pending", "processing", "completed", "failed"
	Error     string `json:"error,omitempty"`
}

// JobStatusResponse represents the response from checking job status
type JobStatusResponse struct {
	Status       string   `json:"status"` // "pending", "processing", "completed", "failed"
	ResultID     uint     `json:"result_id"`
	Questions    []string `json:"questions,omitempty"`
	Responses    []string `json:"responses,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
}

// GeneratePDFRequest represents the request to generate a PDF
type GeneratePDFRequest struct {
	Questions []string           `json:"questions"` // Array of exam questions
	Config    *printer.PDFConfig `json:"config,omitempty"`  // Optional custom PDF config
}

// analyzeHandler handles PDF/image analysis requests
func (s *Server) analyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get username and API key ID from context (set by auth middleware)
	username := r.Header.Get("X-Username")
	apiKeyIDStr := r.Header.Get("X-APIKeyID")
	apiKeyID, _ := strconv.ParseUint(apiKeyIDStr, 10, 32)

	// Parse multipart form
	err := r.ParseMultipartForm(50 * 1024 * 1024) // 50MB max
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid form: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Determine file type
	inputType := "pdf"
	if strings.Contains(fileHeader.Filename, ".png") || strings.Contains(fileHeader.Filename, ".jpg") || strings.Contains(fileHeader.Filename, ".jpeg") {
		inputType = "image"
	}

	// Create temp directory if it doesn't exist
	tmpDir := s.config.TempDir
	os.MkdirAll(tmpDir, 0755)

	// Save uploaded file
	tempFilePath := filepath.Join(tmpDir, fileHeader.Filename)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to save file: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to copy file: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Parse optional PDF config
	var pdfsConfig *printer.PDFConfig
	if cfgStr := r.FormValue("pdf_config"); cfgStr != "" {
		pdfsConfig = &printer.PDFConfig{}
		if err := json.Unmarshal([]byte(cfgStr), pdfsConfig); err != nil {
			log.Printf("Warning: failed to parse PDF config: %v", err)
		}
	}

	// Merge with defaults
	finalConfig := pdfsConfig
	if finalConfig == nil {
		finalConfig = printer.GetDefaultPDFConfig()
	} else {
		finalConfig = finalConfig.MergeWithDefaults()
	}

	// Process file and extract questions
	var questions []string
	if inputType == "pdf" {
		// Extract questions from PDF
		questions, err = printer.ReadTextFromPDF(tempFilePath)
		if err != nil {
			log.Printf("Warning: failed to extract text from PDF: %v", err)
			questions = []string{} // Continue with empty questions
		}
	} else {
		// For images, placeholder
		questions = []string{"Image analysis not yet implemented"}
	}

	// Archive result in database with empty responses initially
	configMap := make(map[string]interface{})
	configJSON, _ := json.Marshal(finalConfig)
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		log.Printf("Warning: failed to unmarshal config: %v", err)
		// Continue with empty map
	}

	emptyResponses := []string{}
	result, err := db.CreateResult(uint(apiKeyID), username, inputType, tempFilePath, questions, emptyResponses, configMap)
	if err != nil {
		log.Printf("Error creating result: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to archive result: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Create async job for GPT analysis
	job, err := db.CreateJob(result.ID)
	if err != nil {
		log.Printf("Error creating job: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to create job: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return 202 Accepted with job details
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := AnalyzeResponse{
		Status:    "accepted",
		ResultID:  result.ID,
		JobID:     job.ID,
		JobStatus: "pending",
	}
	json.NewEncoder(w).Encode(response)

	// Clean up temp file (optional - keep for debugging)
	// os.Remove(tempFilePath)
}

// ResultsResponse represents paginated results
type ResultsResponse struct {
	Status  string     `json:"status"`
	Results []ResultItem `json:"results"`
	Total   int64      `json:"total"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
}

// ResultItem represents a single result in the list
type ResultItem struct {
	ID                uint     `json:"id"`
	InputType         string   `json:"input_type"`
	QuestionsCount    int      `json:"questions_count"`
	ResponsesCount    int      `json:"responses_count"`
	CreatedAt         string   `json:"created_at"`
}

// resultsHandler retrieves paginated results for the authenticated user
func (s *Server) resultsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("X-Username")

	// Parse pagination params
	limit := 20
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Query database
	results, total, err := db.ListResultsByUsername(username, limit, offset)
	if err != nil {
		log.Printf("Error fetching results: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to fetch results: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Build response
	items := make([]ResultItem, len(results))
	for i, r := range results {
		var questions []string
		var responses []string
		if err := json.Unmarshal(r.QuestionsExtracted, &questions); err != nil {
			log.Printf("Warning: failed to unmarshal questions: %v", err)
			questions = []string{}
		}
		if err := json.Unmarshal(r.AIResponses, &responses); err != nil {
			log.Printf("Warning: failed to unmarshal responses: %v", err)
			responses = []string{}
		}

		items[i] = ResultItem{
			ID:             r.ID,
			InputType:      r.InputType,
			QuestionsCount: len(questions),
			ResponsesCount: len(responses),
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := ResultsResponse{
		Status:  "success",
		Results: items,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}
	json.NewEncoder(w).Encode(response)
}

// jobStatusHandler retrieves the status of an async job
func (s *Server) jobStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract result ID from URL path
	resultIDStr := strings.TrimPrefix(r.URL.Path, "/job-status/")
	resultID, err := strconv.ParseUint(resultIDStr, 10, 32)
	if err != nil || resultIDStr == "" {
		http.Error(w, `{"error":"invalid result_id"}`, http.StatusBadRequest)
		return
	}

	// Get the job
	job, err := db.GetJobByResultID(uint(resultID))
	if err != nil {
		http.Error(w, `{"error":"job not found"}`, http.StatusNotFound)
		return
	}

	// Get the result
	result, err := db.GetResultByID(uint(resultID))
	if err != nil {
		http.Error(w, `{"error":"result not found"}`, http.StatusNotFound)
		return
	}

	// Parse questions and responses
	var questions []string
	var responses []string
	if err := json.Unmarshal(result.QuestionsExtracted, &questions); err != nil {
		log.Printf("Warning: failed to unmarshal questions: %v", err)
		questions = []string{}
	}
	if err := json.Unmarshal(result.AIResponses, &responses); err != nil {
		log.Printf("Warning: failed to unmarshal responses: %v", err)
		responses = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := JobStatusResponse{
		Status:    job.Status,
		ResultID:  result.ID,
		Questions: questions,
		Responses: responses,
	}

	if job.Status == "failed" {
		response.ErrorMessage = job.ErrorMessage
	}

	json.NewEncoder(w).Encode(response)
}

// generatePDFHandler creates and returns a protected PDF
func (s *Server) generatePDFHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req GeneratePDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if len(req.Questions) == 0 {
		http.Error(w, `{"error":"questions array cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// Use custom config or defaults
	config := req.Config
	if config == nil {
		config = printer.GetDefaultPDFConfig()
	} else {
		config = config.MergeWithDefaults()
	}

	// Generate PDF in memory (using temp file)
	tempDir := s.config.TempDir
	os.MkdirAll(tempDir, 0755)

	tempPDFPath := filepath.Join(tempDir, fmt.Sprintf("generated_%d.pdf", time.Now().UnixNano()))

	// Generate the PDF
	printer.GenerateProtectedPDF(tempPDFPath, req.Questions)

	// Read the generated PDF
	pdfBytes, err := os.ReadFile(tempPDFPath)
	if err != nil {
		log.Printf("Error reading generated PDF: %v", err)
		http.Error(w, `{"error":"failed to read generated PDF"}`, http.StatusInternalServerError)
		return
	}

	// Clean up temp file
	defer os.Remove(tempPDFPath)

	// Return PDF
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Header().Set("Content-Disposition", "attachment; filename=exam_protected.pdf")
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}
