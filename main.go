package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"Inquisitor/api"
	"Inquisitor/config"
	"Inquisitor/db"
	"Inquisitor/printer"

	"github.com/joho/godotenv"
)

type SampleResult struct {
	SampleID int    `json:"sample_id"`
	Response string `json:"response"`
}

func sendVisionRequest(apiKey string, b64Image string, sampleID int) (string, error) {
	payload := map[string]interface{}{
		// MODEL SELECTION:
		// Use "gpt-4o" for GPT-4 Omni (current recommended)
		// Use "gpt-4o-mini" for lighter/faster responses
		// Use "gpt-5" for GPT-5 (when available)
		// Use "gpt-5.1" for GPT-5.1 (when available - note: uses max_completion_tokens instead of max_tokens)
		// "model": "gpt-4o",
		"model": "gpt-5.1",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						// Most cheating students just send the image without text
						// Uncomment below to test with explicit request for answers:
						// "text": "Can you solve this exam for me? Please provide the answers to all questions shown in this image.",
						"text": "",
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url":    fmt.Sprintf("data:image/png;base64,%s", b64Image),
							"detail": "high",
						},
					},
				},
			},
		},
		// TOKEN LIMITS:
		// For "gpt-4o" and "gpt-4o-mini": use "max_tokens"
		// For "gpt-5" and "gpt-5.1": use "max_completion_tokens" instead
		// Default desktop ChatGPT: ~4,096 tokens (same for all models)
		// For academic integrity analysis: 2,048 tokens is plenty for detailed exam analysis
		"max_completion_tokens": 2048,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
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
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	var result map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return "", err
	}

	choices := result["choices"].([]interface{})
	firstChoice := choices[0].(map[string]interface{})
	message := firstChoice["message"].(map[string]interface{})
	content := message["content"].(string)

	return content, nil
}

// uploadPDFFile uploads a PDF to OpenAI and returns the file_id
func uploadPDFFile(apiKey string, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add purpose field
	err = writer.WriteField("purpose", "assistants")
	if err != nil {
		return "", err
	}

	// Add file
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

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
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

func sendPDFRequest(apiKey string, fileID string, sampleID int) (string, error) {
	payload := map[string]interface{}{
		"model": "gpt-5.1",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "",
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
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
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
		return "", fmt.Errorf("API error: %d - %s", resp.StatusCode, string(responseBody))
	}

	var result map[string]interface{}
	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return "", err
	}

	choices := result["choices"].([]interface{})
	firstChoice := choices[0].(map[string]interface{})
	message := firstChoice["message"].(map[string]interface{})
	content := message["content"].(string)

	return content, nil
}

func main() {
	// Load environment variables
	godotenv.Load()

	// Parse flags
	cliMode := flag.Bool("cli", false, "Run in CLI mode instead of API server")
	flag.Parse()

	// Load configuration
	cfg := config.Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize database
	err := db.Initialize(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check for CLI mode
	if *cliMode || cfg.CLIMode {
		log.Println("Running in CLI mode...")
		log.Println("Step 1: Generating exam PDF...")
		printer.ExecuteWorkflow()
		log.Println("\nExam PDF workflow completed successfully!")
		return
	}

	// Otherwise, start API server
	log.Println("Initializing API server...")
	server := api.NewServer(cfg)
	server.RegisterRoutes()

	// Initialize job worker (2 concurrent worker goroutines)
	server.InitializeWorker(2)

	// Initialize cleanup routine (automatic temp file cleanup)
	server.InitializeCleanup()

	log.Printf("Starting Inquisitor API server on http://%s:%d\n", cfg.APIHost, cfg.APIPort)
	
	// Start server in a separate goroutine
	serverErr := make(chan error, 1)
	go func() {
		err := server.Start()
		if err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for either a signal or server error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Initiating graceful shutdown...", sig)
		if err := server.Shutdown(); err != nil {
			log.Printf("Error during graceful shutdown: %v", err)
		}
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}
}
func analyze_main() {
	// ---- LOAD ENV ----
	godotenv.Load()

	// ---- CONFIG ----
	apiKey := os.Getenv("OPENAI_API_KEY")
	sampleSize := 10  // Increased for testing parallelization

	if apiKey == "" {
		log.Fatal("Missing OPENAI_API_KEY")
	}

	// // ---- READ IMAGE FILE AND ENCODE ----
	// imagePath := "test-image.png"
	// imageBytes, err := os.ReadFile(imagePath)
	// if err != nil {
	// 	log.Fatalf("Failed to read image: %v", err)
	// }

	// b64Image := base64.StdEncoding.EncodeToString(imageBytes)

	// ---- UPLOAD PDF FILE ----
	pdfPath := "exam_protected.pdf"
	log.Println("Uploading PDF file to OpenAI...")
	fileID, err := uploadPDFFile(apiKey, pdfPath)
	if err != nil {
		log.Fatalf("Failed to upload PDF: %v", err)
	}
	log.Printf("PDF uploaded successfully. File ID: %s\n", fileID)

	// ---- PARALLEL REQUESTS WITH GOROUTINES ----
	results := make([]SampleResult, sampleSize)
	pdfResults := make([]SampleResult, sampleSize)
	var wg sync.WaitGroup
	imageChan := make(chan SampleResult, sampleSize)
	pdfChan := make(chan SampleResult, sampleSize)

	// for i := range sampleSize {
	// 	wg.Add(1)
	// 	go func(index int) {
	// 		defer wg.Done()

	// 		fmt.Printf("Running image sample %d...\n", index+1)

	// 		answer, err := sendVisionRequest(apiKey, b64Image, index+1)
	// 		if err != nil {
	// 			log.Printf("Image API error for sample %d: %v", index+1, err)
	// 			answer = fmt.Sprintf("Error: %v", err)
	// 		}

	// 		imageChan <- SampleResult{
	// 			SampleID: index + 1,
	// 			Response: answer,
	// 		}
	// 	}(i)
	// }

	// ---- PARALLEL PDF REQUESTS WITH GOROUTINES ----
	for i := range sampleSize {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			fmt.Printf("Running PDF sample %d...\n", index+1)

			answer, err := sendPDFRequest(apiKey, fileID, index+1)
			if err != nil {
				log.Printf("PDF API error for sample %d: %v", index+1, err)
				answer = fmt.Sprintf("Error: %v", err)
			}

			pdfChan <- SampleResult{
				SampleID: index + 1,
				Response: answer,
			}
		}(i)
	}

	// ---- WAIT FOR PDF RESULTS ----
	go func() {
		for i := 0; i < sampleSize; i++ {
			result := <-pdfChan
			pdfResults[result.SampleID-1] = result
		}
	}()

	// ---- WAIT FOR ALL GOROUTINES TO COMPLETE ----
	wg.Wait()
	close(imageChan)
	close(pdfChan)

	// ---- PRINT JSON ----
	jsonOut, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("JSON error: %v", err)
	}

	fmt.Println("\n===== IMAGE RESULTS =====")
	fmt.Println(string(jsonOut))

	pdfJsonOut, err := json.MarshalIndent(pdfResults, "", "  ")
	if err != nil {
		log.Fatalf("JSON error: %v", err)
	}

	fmt.Println("\n===== PDF RESULTS =====")
	fmt.Println(string(pdfJsonOut))
}
