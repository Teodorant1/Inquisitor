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
)

var DB *gorm.DB

func main() {


	godotenv.Load()
	initDB()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/exams/upload-mutate", AuthMiddleware(handleUploadAndMutate))
	mux.HandleFunc("/api/exams/download", AuthMiddleware(handleDownloadPDF))
	mux.HandleFunc("/api/exams/analyze", AuthMiddleware(handleAIAnalyze))

	log.Println("Inquisitor API Engine running smoothly on :8080...")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func initDB() {
	dsn := os.Getenv("DATABASE_URL")
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to establish PostgreSQL connection: %v", err)
	}

	// 1. Run your schema synchronizations first
	DB.AutoMigrate(&models.User{}, &models.Exam{}, &models.AnalyzeResult{})

	// 2. Safely extract the underlying generic sql.DB handle
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("Failed to extract underlying SQL connection pool: %v", err)
	}

	// 3. Configure connection rules for Neon's transaction pooler
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

		r.Header.Set("X-User-ID", fmt.Sprintf("%d", user.ID))
		next.ServeHTTP(w, r)
	}
}

func handleUploadAndMutate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart payload safely up to 32MB
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

	userID, _ := strconv.Atoi(r.Header.Get("X-User-ID"))

	// 1. Extract and evaluate standalone 'usedefault' field
	useDefaultStr := r.FormValue("usedefault")
	useDefault := true // Default behavior if parameter isn't passed down
	if strings.ToLower(useDefaultStr) == "false" {
		useDefault = false
	}

	// 2. Extract and parse structured 'pdfconfig' field JSON data payload
	var cfg printer.PDFConfig
	configJSON := r.FormValue("pdfconfig")
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			http.Error(w, fmt.Sprintf("Invalid structural configuration JSON: %v", err), http.StatusBadRequest)
			return
		}
	}

	iterations, _ := strconv.Atoi(r.FormValue("iterations"))
	if iterations <= 0 {
		iterations = 1
	}

	os.MkdirAll("./storage", os.ModePerm)
	origPath := filepath.Join("./storage", fmt.Sprintf("user_%d_orig_%s", userID, handler.Filename))
	
	out, err := os.Create(origPath)
	if err != nil {
		http.Error(w, "Storage operational error", http.StatusInternalServerError)
		return
	}
	io.Copy(out, file)
	out.Close()

	extractedLines, err := printer.ExtractUploadedPDF(origPath)
	if err != nil {
		os.Remove(origPath)
		http.Error(w, fmt.Sprintf("Failed to parse PDF: %v", err), http.StatusInternalServerError)
		return
	}

	exam := models.Exam{
		UserID: uint(userID),
		Status: "processing",
	}
	DB.Create(&exam)

	apiKey := os.Getenv("OPENAI_API_KEY")
	var wg sync.WaitGroup
	resultChan := make(chan models.AnalyzeResult, iterations)
	fullTextContent := strings.Join(extractedLines, "\n")

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response, modelUsed, promptTokens, completionTokens, err := sendTextAnalysisRequest(apiKey, fullTextContent)
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

	for res := range resultChan {
		DB.Create(&res)
	}

	mutatedFilename := fmt.Sprintf("user_%d_mutated_%s", userID, handler.Filename)
	mutatedPath := filepath.Join("./storage", mutatedFilename)

	// Pass parsed struct properties and useDefault value directly to mutated layout engine
	err = printer.GenerateDynamicProtectedPDF(mutatedPath, extractedLines, cfg, useDefault)
	if err != nil {
		os.Remove(origPath)
		http.Error(w, "Failed creating modified engine file", http.StatusInternalServerError)
		return
	}

	os.Remove(origPath)

	exam.Status = "processed"
	DB.Save(&exam)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Analysis and mutation completed successfully. Original deleted.",
		"exam_id": exam.ID,
	})
}

func sendTextAnalysisRequest(apiKey string, text string) (string, string, int, int, error) {
	payload := map[string]interface{}{
		"model": "gpt-5.1",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": fmt.Sprintf("Analyze this exam content for academic vulnerability: %s", text),
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

func handleDownloadPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	examIDStr := r.URL.Query().Get("exam_id")
	if examIDStr == "" {
		http.Error(w, "Missing exam_id parameter", http.StatusBadRequest)
		return
	}

	userID := r.Header.Get("X-User-ID")

	var exam models.Exam
	if err := DB.Where("id = ? AND user_id = ?", examIDStr, userID).First(&exam).Error; err != nil {
		http.Error(w, "Exam not found or unauthorized", http.StatusNotFound)
		return
	}

	matches, _ := filepath.Glob(filepath.Join("./storage", fmt.Sprintf("user_%s_mutated_*", userID)))
	if len(matches) == 0 {
		http.Error(w, "Mutated file not found or already downloaded", http.StatusNotFound)
		return
	}

	targetFilePath := matches[0]

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(targetFilePath)))
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, targetFilePath)

	go func() {
		if err := os.Remove(targetFilePath); err != nil {
			log.Printf("Deferred deletion failed for %s: %v", targetFilePath, err)
		} else {
			log.Printf("Successfully purged mutated file: %s", targetFilePath)
		}
	}()
}

// Endpoint 3: Receives a multi-part file directly, sends it to OpenAI files, runs analysis loop, and purges
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

	userID, _ := strconv.Atoi(r.Header.Get("X-User-ID"))
	iterations, _ := strconv.Atoi(r.FormValue("iterations"))
	if iterations <= 0 {
		iterations = 1
	}

	// Save file locally to stream it out to OpenAI safely
	os.MkdirAll("./storage", os.ModePerm)
	tempPath := filepath.Join("./storage", fmt.Sprintf("user_%d_analyze_%s", userID, handler.Filename))
	out, err := os.Create(tempPath)
	if err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}
	io.Copy(out, file)
	out.Close()

	// Track operation under a clean Exam instance entry
	exam := models.Exam{
		UserID: uint(userID),
		Status: "analyzing_file",
	}
	DB.Create(&exam)

	apiKey := os.Getenv("OPENAI_API_KEY")
	
	// Upload the target file binary straight to OpenAI's server infrastructure
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

	// Delete the local copy instantly right here
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

	err = writer.WriteField("purpose", "assistants")
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
						"type": "text",
						"text": "Analyze this file for security and academic integrity metrics.",
					},
					{
						"type": "file",
						"file": map[string]interface{}{
							"file_id": fileID,
						},
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