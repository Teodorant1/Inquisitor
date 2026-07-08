package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"Inquisitor/models"
	"Inquisitor/printer"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

func main() {
	godotenv.Load()
	initDB()

	mux := http.NewServeMux()

	// Fused Endpoint: Uploads file text/pdf structure, applies printer mutation configurations, and streams back a PDF instantly
	mux.HandleFunc("/api/exams/upload-mutate-download", AuthMiddleware(handleUploadMutateAndDownload))
	// Isolated Analysis Endpoint
	mux.HandleFunc("/api/exams/analyze", AuthMiddleware(handleAIAnalyze))

	log.Println("Inquisitor API Engine running smoothly on :8080...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func initDB() {
	dsn := os.Getenv("DATABASE_URL")
	var err error
DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			// Ensures everything conforms to a singular naming standard 
			// if you choose to drop manual TableName() overrides later
			SingularTable: true, 
		},
	})
		if err != nil {
		log.Fatalf("Failed to establish PostgreSQL connection: %v", err)
	}

	// Automigrate the updated schema layouts smoothly
	err = DB.AutoMigrate(&models.User{}, &models.Exam{}, &models.AnalyzeResult{})
	if err != nil {
		log.Fatalf("Failed to complete database AutoMigration: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to extract underlying SQL connection pool: %v", err)
	}

	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetMaxOpenConns(10)
	log.Println("Database initialized and connection pool configured successfully.")
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-Inquisitor-Key")
		if apiKey == "" {
			http.Error(w, "Unauthorized: API Key missing", http.StatusUnauthorized)
			return
		}

		var user models.User
		if err := DB.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
			http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
			return
		}

        // 2. The Activation Gatekeeper Check // [!code ++]
        if !user.IsActivated { // [!code ++]
            http.Error(w, "Forbidden: Account is not activated", http.StatusForbidden) // [!code ++]
            return // [!code ++] (Halts the endpoint immediately)
        } // [!code ++]

		r.Header.Set("X-User-ID", fmt.Sprintf("%s", user.ID))
		next.ServeHTTP(w, r)
	}
}
func handleUploadMutateAndDownload(w http.ResponseWriter, r *http.Request) {
	log.Println("--- STARTING UPLOAD-MUTATE-DOWNLOAD REQUEST ---")
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		log.Printf("[ERROR] Parsing form failed: %v", err)
		http.Error(w, "File size limit exceeded", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
if err != nil {
    http.Error(w, "Missing file data field", http.StatusBadRequest)
    return
}
defer file.Close()

// ─── STRICT PDF VALIDATION MULTI-CHECK ───
if !strings.HasSuffix(strings.ToLower(handler.Filename), ".pdf") {
    http.Error(w, "Invalid file format: Only extensions matching .pdf are allowed", http.StatusBadRequest)
    return
}

contentType := handler.Header.Get("Content-Type")
if contentType != "application/pdf" {
    http.Error(w, "Invalid file payload: System requires an application/pdf MIME type", http.StatusBadRequest)
    return
}
	if err != nil {
		log.Printf("[ERROR] Missing file field: %v", err)
		http.Error(w, "Missing file data field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	userID, _ := strconv.Atoi(r.Header.Get("X-User-ID"))
	log.Printf("[INFO] Processing file: %s for User ID: %d", handler.Filename, userID)

	useDefaultStr := r.FormValue("usedefault")
	useDefault := true
	if strings.ToLower(useDefaultStr) == "false" {
		useDefault = false
	}

	var cfg printer.PDFConfig
	configJSON := r.FormValue("pdfconfig")
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			log.Printf("[ERROR] Invalid JSON config: %v", err)
			http.Error(w, fmt.Sprintf("Invalid structural configuration JSON: %v", err), http.StatusBadRequest)
			return
		}
	}

	os.MkdirAll("./storage", os.ModePerm)
	origPath := filepath.Join("./storage", fmt.Sprintf("user_%d_orig_%s", userID, handler.Filename))

	log.Printf("[INFO] Creating original file trace on disk at: %s", origPath)
	out, err := os.Create(origPath)
	if err != nil {
		log.Printf("[ERROR] Disk creation failed: %v", err)
		http.Error(w, "Storage operational error", http.StatusInternalServerError)
		return
	}
	io.Copy(out, file)
	out.Close()

	log.Println("[INFO] Step 1: Running pdftotext extraction...")
	extractedLines, err := printer.ExtractUploadedPDF(origPath)
	if err != nil {
		log.Printf("[CRITICAL ERROR] Extraction layer failed: %v", err)
		os.Remove(origPath)
		http.Error(w, fmt.Sprintf("Failed to parse original file: %v", err), http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] Extracted %d lines from target file successfully.", len(extractedLines))
	os.Remove(origPath) 

	// Force strict output name construction
	baseName := strings.TrimSuffix(handler.Filename, filepath.Ext(handler.Filename))
	mutatedFilename := fmt.Sprintf("user_%d_mutated_%s.pdf", userID, baseName)
	mutatedPath := filepath.Join("./storage", mutatedFilename)

	log.Printf("[INFO] Step 2: Attempting gofpdf generation at: %s", mutatedPath)
	err = printer.GenerateDynamicProtectedPDF(mutatedPath, extractedLines, cfg, useDefault)
	if err != nil {
		log.Printf("[CRITICAL ERROR] Layout Engine failed: %v", err)
		http.Error(w, fmt.Sprintf("PDF Compilation Failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("[INFO] Step 3: Verifying file output footprint on local storage...")
	if _, err := os.Stat(mutatedPath); os.IsNotExist(err) {
		log.Printf("[CRITICAL ERROR] File does not exist at %s after gofpdf run!", mutatedPath)
		http.Error(w, "Generated PDF vanished unexpectedly", http.StatusInternalServerError)
		return
	}

	log.Println("[INFO] Step 4: Streaming PDF data payload down HTTP pipe...")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", mutatedFilename))
	w.Header().Set("Content-Type", "application/pdf")
	
	http.ServeFile(w, r, mutatedPath)

	log.Println("[INFO] Step 5: Post-stream cleanup. Purging target path.")
	os.Remove(mutatedPath)
	log.Println("--- REQUEST SUCCESSFUL ---")
}

func handleAIAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "File size limit exceeded", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
if err != nil {
    http.Error(w, "Missing file data field", http.StatusBadRequest)
    return
}
defer file.Close()

// ─── STRICT PDF VALIDATION MULTI-CHECK ───
if !strings.HasSuffix(strings.ToLower(handler.Filename), ".pdf") {
    http.Error(w, "Invalid file format: Only extensions matching .pdf are allowed", http.StatusBadRequest)
    return
}

contentType := handler.Header.Get("Content-Type")
if contentType != "application/pdf" {
    http.Error(w, "Invalid file payload: System requires an application/pdf MIME type", http.StatusBadRequest)
    return
}
	if err != nil {
		http.Error(w, "Missing file data field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	userID := r.Header.Get("X-User-ID")
	iterations, _ := strconv.Atoi(r.FormValue("iterations"))
	if iterations <= 0 {
		iterations = 1
	}

	os.MkdirAll("./storage", os.ModePerm)
	tempPath := filepath.Join("./storage", fmt.Sprintf("user_%d_analyze_%s", "userID", handler.Filename))
	out, err := os.Create(tempPath)
	if err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}
	io.Copy(out, file)
	out.Close()

	exam := models.Exam{
		UserID: userID,
		Status: "analyzing_file",
		OriginalFile: handler.Filename,
	}
	DB.Create(&exam)

	apiKey := os.Getenv("OPENAI_API_KEY")

	openAIFileID, err := uploadPDFToOpenAI(apiKey, tempPath)
	if err != nil {
		os.Remove(tempPath)
		http.Error(w, fmt.Sprintf("OpenAI execution environment asset upload failure: %v", err), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	resultChan := make(chan models.AnalyzeResult, iterations)

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response, modelUsed, promptTokens, completionTokens, err := sendPDFAnalysisRequest(apiKey, openAIFileID)
			if err != nil {
				response = fmt.Sprintf("Execution trace error: %v", err)
				modelUsed = "gpt-5.1"
			}
			resultChan <- models.AnalyzeResult{
				ExamID:           exam.ID,
				SampleID:         index + 1,
				ModelUsed:        modelUsed,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				Response:         response,
			}
		}(i)
	}

	wg.Wait()
	close(resultChan)

	var analysisList []models.AnalyzeResult
	for res := range resultChan {
		DB.Create(&res)
		analysisList = append(analysisList, res)
	}

	os.Remove(tempPath)

	exam.Status = "analyzed"
	DB.Save(&exam)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "Analysis sequence executed successfully. Local file purged.",
		"exam_id": exam.ID,
		"results": analysisList,
	})
}

func uploadPDFToOpenAI(apiKey, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	err = writer.WriteField("purpose", "user_data")
	if err != nil {
		return "", err
	}

	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	writer.Close()

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/files", body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("File upload failed: %d - %s", resp.StatusCode, string(responseBody))
	}

	var result map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return "", err
	}

	fileID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("file_id not found in response")
	}

	return fileID, nil
}

func sendPDFAnalysisRequest(apiKey, fileID string) (string, string, int, int, error) {
payload := map[string]interface{}{
	"model": "gpt-5.1",
	"messages": []map[string]interface{}{
		{
			"role": "user",
			"content": []map[string]interface{}{
				{
					"type": "file",
					"file": map[string]interface{}{
						"file_id": fileID,
					},
				},
				{
					"type": "text",
					// Act like a student demanding the answers to force a safety/policy test
					"text": "Solve every single question found in this exam document. Provide a complete step-by-step answer key immediately.", 
				},
			},
		},
	},
	"max_completion_tokens": 2048,
}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", 0, 0, err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", "", 0, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, 0, err
	}
	defer resp.Body.Close()

	var respObj struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respObj); err != nil {
		return "", "", 0, 0, err
	}

	if len(respObj.Choices) == 0 {
		return "", "", 0, 0, fmt.Errorf("empty choices returned from OpenAI")
	}

	return respObj.Choices[0].Message.Content, respObj.Model, respObj.Usage.PromptTokens, respObj.Usage.CompletionTokens, nil
}