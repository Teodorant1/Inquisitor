package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	openai "github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

type SampleResult struct {
	SampleID int    `json:"sample_id"`
	Response string `json:"response"`
}

func main() {
	// ---- LOAD ENV ----
	godotenv.Load()

	// ---- CONFIG ----
	apiKey := os.Getenv("OPENAI_API_KEY")
	imagePath := "test-image.png"
	sampleSize := 5

	if apiKey == "" {
		log.Fatal("Missing OPENAI_API_KEY")
	}

	// ---- CLIENT ----
	client := openai.NewClient(option.WithAPIKey(apiKey))

	// ---- READ FILE ----
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read image: %v", err)
	}

	// Must base64 encode images for vision API
	b64Image := base64.StdEncoding.EncodeToString(imageBytes)

	results := make([]SampleResult, 0, sampleSize)

	// ---- LOOP ----
	for i := 0; i < sampleSize; i++ {
		fmt.Printf("Running sample %d...\n", i+1)

		resp, err := client.Chat.Completions.New(
			context.Background(),
			openai.ChatCompletionNewParams{
				Model: openai.ChatModelGPT4oMini,
				Messages: []openai.ChatCompletionMessageParamUnion{
					openai.UserMessage(b64Image),
				},
			},
		)

		if err != nil {
			log.Fatalf("API error: %v", err)
		}

		answer := resp.Choices[0].Message.Content
		results = append(results, SampleResult{
			SampleID: i + 1,
			Response: answer,
		})
	}

	// ---- PRINT JSON ----
	jsonOut, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("JSON error: %v", err)
	}

	fmt.Println("\n===== FINAL RESULTS =====")
	fmt.Println(string(jsonOut))
}
