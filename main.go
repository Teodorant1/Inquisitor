package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

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
	log.Println("Step 1: Generating exam PDF with lines using pdfcpu...")
	printer.ExecuteWorkflow()

	log.Println("\nExam PDF workflow completed successfully!")
	 analyze_main()
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

	// ---- READ FILE AND ENCODE ----
	imagePath := "test-image.png"
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read image: %v", err)
	}

	b64Image := base64.StdEncoding.EncodeToString(imageBytes)

	// ---- PARALLEL REQUESTS WITH GOROUTINES ----
	results := make([]SampleResult, sampleSize)
	var wg sync.WaitGroup
	resultsChan := make(chan SampleResult, sampleSize)

	for i := range sampleSize {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			fmt.Printf("Running sample %d...\n", index+1)

			answer, err := sendVisionRequest(apiKey, b64Image, index+1)
			if err != nil {
				log.Printf("API error for sample %d: %v", index+1, err)
				answer = fmt.Sprintf("Error: %v", err)
			}

			// fmt.Printf("Response at index %d: %s\n", index, answer)
			resultsChan <- SampleResult{
				SampleID: index + 1,
				Response: answer,
			}
		}(i)
	}

	// ---- WAIT FOR ALL GOROUTINES TO COMPLETE ----
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// ---- COLLECT RESULTS ----
	for result := range resultsChan {
		results[result.SampleID-1] = result
	}

	// ---- PRINT JSON ----
	jsonOut, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("JSON error: %v", err)
	}

	fmt.Println("\n===== FINAL RESULTS =====")
	fmt.Println(string(jsonOut))
}
