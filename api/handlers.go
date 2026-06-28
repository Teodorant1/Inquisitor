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

// analyzeHandler handles PDF/image analysis requests, archives states, and pushes instantly to async channels
func (s *Server) analyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Unpack credentials and track safety boundaries
	username := r.Header.Get("X-Username")
	apiKeyIDStr := r.Header.Get("X-APIKeyID")
	apiKeyID, err := strconv.ParseUint(apiKeyIDStr, 10, 32)
	if err != nil {
		http.Error(w, `{"error":"invalid or missing API key context"}`, http.StatusBadRequest)
		return
	}

	// 2. Parse multi-part stream (50MB cap)
	if err := r.ParseMultipartForm(50 * 1024 * 1024); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid form: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"missing file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 3. Determine input category accurately by trailing suffix
	inputType := "pdf"
	lowerFilename := strings.ToLower(fileHeader.Filename)
	if strings.HasSuffix(lowerFilename, ".png") || strings.HasSuffix(lowerFilename, ".jpg") || strings.HasSuffix(lowerFilename, ".jpeg") {
		inputType = "image"
	}

	tmpDir := s.config.TempDir
	_ = os.MkdirAll(tmpDir, 0755)

	// FIX 1: Prevent Path Traversal by stripping directory paths via filepath.Base
	safeFilename := filepath.Base(fileHeader.Filename)
	tempFilePath := filepath.Join(tmpDir, safeFilename)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		log.Printf("Error creating local temp file storage: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to save file locally: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_, err = io.Copy(tempFile, file)
	
	// FIX 2: Explicitly close the file handle RIGHT NOW to force buffers to flush onto disk.
	// This prevents the PDF text extraction engine from reading an incomplete or locked 0-byte file descriptor.
	tempFile.Close()

	if err != nil {
		log.Printf("Error streaming data blocks into file: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to fully copy stream data: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 4. Parse custom formatting configs
	var pdfsConfig *printer.PDFConfig
	if cfgStr := r.FormValue("pdf_config"); cfgStr != "" {
		pdfsConfig = &printer.PDFConfig{}
		if err := json.Unmarshal([]byte(cfgStr), pdfsConfig); err != nil {
			log.Printf("Warning: failed to parse incoming custom PDF config: %v", err)
		}
	}

	finalConfig := pdfsConfig
	if finalConfig == nil {
		finalConfig = printer.GetDefaultPDFConfig()
	} else {
		finalConfig = finalConfig.MergeWithDefaults()
	}

	// 5. Execute extraction engine routines
	var questions []string
	if inputType == "pdf" {
		// Now completely safe to execute because the write descriptor was closed above
		questions, err = printer.ReadTextFromPDF(tempFilePath)
		if err != nil {
			log.Printf("Warning: text extraction subsystem hit an error on %s: %v", safeFilename, err)
			questions = []string{} // Continues with empty array to avoid hard crashes
		}
	} else {
		questions = []string{"Image analysis not yet implemented"}
	}

	// 6. Serialize configurations into generic DB formats
	configMap := make(map[string]interface{})
	configJSON, _ := json.Marshal(finalConfig)
	_ = json.Unmarshal(configJSON, &configMap)

	emptyResponses := []string{}
	result, err := db.CreateResult(uint(apiKeyID), username, inputType, tempFilePath, questions, emptyResponses, configMap)
	if err != nil {
		log.Printf("Error registering results history index: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to archive initial calculation indices: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// 7. Register processing tracker instance inside DB (State recovery)
	job, err := db.CreateJob(result.ID)
	if err != nil {
		log.Printf("Error creating tracking task metadata context: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"failed to record task context: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// FIX 3: Push directly to our local worker pool channel to start processing instantly, 
	// bypassing the 2-second database polling lag entirely while keeping our worker limits.
	s.EnqueueJob(job)

	// 8. Return response immediately
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	response := AnalyzeResponse{
		Status:    "accepted",
		ResultID:  result.ID,
		JobID:     job.ID,
		JobStatus: "pending",
	}
	_ = json.NewEncoder(w).Encode(response)
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

// downloadPDFHandler generates a physical PDF on disk, streams it to the browser, 
// and immediately deletes it from the VPS hard drive once the download finishes.
func (s *Server) downloadPDFHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 1. Extract Result ID from the URL path (e.g., /download/42)
    pathParts := strings.Split(r.URL.Path, "/")
    if len(pathParts) < 3 {
        http.Error(w, `{"error":"malformed download path"}`, http.StatusBadRequest)
        return
    }
    idStr := pathParts[2]

    resultID, err := strconv.ParseUint(idStr, 10, 32)
    if err != nil {
        http.Error(w, `{"error":"invalid result id"}`, http.StatusBadRequest)
        return
    }

    // 2. Fetch the text answers out of your database
    result, err := db.GetResultByID(uint(resultID))
    if err != nil {
        http.Error(w, `{"error":"result not found"}`, http.StatusNotFound)
        return
    }

    // 3. Security Check: Make sure the user downloading it owns it
    username := r.Header.Get("X-Username")
    if result.Username != username {
        http.Error(w, `{"error":"unauthorized"}`, http.StatusForbidden)
        return
    }

    var questions []string
    if err := json.Unmarshal(result.QuestionsExtracted, &questions); err != nil {
        log.Printf("Error unmarshaling questions: %v", err)
        questions = []string{}
    }

    // 4. Create the physical file path in your VPS temp directory
    tempDir := s.config.TempDir
    _ = os.MkdirAll(tempDir, 0755)
    
    outFilename := fmt.Sprintf("inquisitor_report_%d.pdf", result.ID)
    tempPDFPath := filepath.Join(tempDir, outFilename)

    // 5. Tell your printer engine to write the file to the disk path
    err = printer.GenerateProtectedPDF(tempPDFPath, questions)
    if err != nil {
        log.Printf("PDF Generation Error: %v", err)
        http.Error(w, `{"error":"failed to generate PDF file"}`, http.StatusInternalServerError)
        return
    }

    // 6. Set headers so the browser triggers a real file saving prompt
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", outFilename))
    w.Header().Set("Content-Type", "application/pdf")

    // 7. Stream the file to the user's browser. 
    // This is synchronous and blocks this specific execution line until the client finishes downloading.
    http.ServeFile(w, r, tempPDFPath)

    // 8. THE PURGE: Now that ServeFile is done transmitting the data bytes over the network wire,
    // we instantly wipe the file from the disk. Zero leaks, zero leftover storage clutter.
    if err := os.Remove(tempPDFPath); err != nil {
        log.Printf("Warning: Failed to auto-delete temporary file %s: %v", tempPDFPath, err)
    }
}