package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
)

const OpenAIAPIURL = "https://api.openai.com/v1"

// AnalyzeQuestionsWithGPT sends questions to ChatGPT for analysis
func AnalyzeQuestionsWithGPT(apiKey string, questions []string) ([]string, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	if len(questions) == 0 {
		return []string{}, nil
	}

	responses := make([]string, 0, len(questions))

	// Analyze each question
	for _, question := range questions {
		if question == "" {
			responses = append(responses, "")
			continue
		}

		// Create prompt for analysis
		prompt := fmt.Sprintf(`Provide a concise analysis of this exam question:

Question: %s

Provide:
1. Key concepts being tested
2. Difficulty level (Easy/Medium/Hard)
3. Common misconceptions
4. Suggested approach to solve

Keep the response brief and practical.`, question)

		// Call OpenAI API via HTTP
		analysis, err := callChatGPT(apiKey, prompt, 500)
		if err != nil {
			log.Printf("Error calling OpenAI for question analysis: %v", err)
			responses = append(responses, fmt.Sprintf("Error: %v", err))
			continue
		}

		responses = append(responses, analysis)
	}

	return responses, nil
}

// BatchAnalyzeQuestionsWithGPT sends all questions in a single prompt for more context-aware analysis
func BatchAnalyzeQuestionsWithGPT(apiKey string, questions []string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	if len(questions) == 0 {
		return "", nil
	}

	// Build prompt with all questions
	prompt := "Analyze these exam questions collectively:\n\n"
	for i, q := range questions {
		prompt += fmt.Sprintf("Question %d: %s\n", i+1, q)
	}

	prompt += `

Provide:
1. Overall exam difficulty and topic coverage
2. Common themes across questions
3. Patterns in question design
4. Estimated exam duration
5. Recommended study focus areas`

	// Call OpenAI API via HTTP
	analysis, err := callChatGPT(apiKey, prompt, 1000)
	if err != nil {
		return "", fmt.Errorf("failed to get OpenAI analysis: %w", err)
	}

	return analysis, nil
}

// AnalyzePDFWithGPT uploads a PDF file and sends it to GPT for analysis
func AnalyzePDFWithGPT(apiKey string, pdfFilePath string, analysisPrompt string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	// Open PDF file
	pdfFile, err := os.Open(pdfFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer pdfFile.Close()

	// Upload PDF to OpenAI Files API
	fileID, err := uploadFileToOpenAI(apiKey, pdfFile, "application/pdf")
	if err != nil {
		return "", fmt.Errorf("failed to upload PDF: %w", err)
	}

	// Create message with uploaded file reference
	if analysisPrompt == "" {
		analysisPrompt = "Please analyze this exam PDF. Extract questions, identify difficulty levels, and provide recommended study areas."
	}

	// Call GPT with file reference via vision/messages API
	analysis, err := callChatGPTWithFile(apiKey, fileID, analysisPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to analyze PDF with GPT: %w", err)
	}

	// Optionally delete file after analysis
	_ = deleteFileFromOpenAI(apiKey, fileID)

	return analysis, nil
}

// callChatGPT makes an HTTP request to OpenAI's chat completions endpoint
func callChatGPT(apiKey string, prompt string, maxTokens int) (string, error) {
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type RequestPayload struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float32   `json:"temperature"`
	}

	type Choice struct {
		Message Message `json:"message"`
	}

	type ResponsePayload struct {
		Choices []Choice `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	payload := RequestPayload{
		Model: "gpt-3.5-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.7,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", OpenAIAPIURL+"/chat/completions", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var respPayload ResponsePayload
	if err := json.Unmarshal(respBody, &respPayload); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if respPayload.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", respPayload.Error.Message)
	}

	if len(respPayload.Choices) > 0 {
		return respPayload.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}

// uploadFileToOpenAI uploads a file to OpenAI's Files API
func uploadFileToOpenAI(apiKey string, file *os.File, mimeType string) (string, error) {
	type FileResponse struct {
		ID    string `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(fileWriter, file)
	if err != nil {
		return "", fmt.Errorf("failed to copy file to form: %w", err)
	}

	// Add purpose field
	writer.WriteField("purpose", "assistants")

	writer.Close()

	req, err := http.NewRequest("POST", OpenAIAPIURL+"/files", body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file to OpenAI: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var fileResp FileResponse
	if err := json.Unmarshal(respBody, &fileResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if fileResp.Error != nil {
		return "", fmt.Errorf("OpenAI file upload error: %s", fileResp.Error.Message)
	}

	return fileResp.ID, nil
}

// callChatGPTWithFile sends a message to GPT referencing an uploaded file
func callChatGPTWithFile(apiKey string, fileID string, prompt string) (string, error) {
	type Message struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}

	type ContentPart struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}

	type RequestPayload struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	type Choice struct {
		Message Message `json:"message"`
	}

	type ResponsePayload struct {
		Choices []Choice `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	// Build message with file reference
	msgContent := []interface{}{
		map[string]string{
			"type": "text",
			"text": prompt,
		},
		map[string]interface{}{
			"type": "document",
			"source": map[string]string{
				"type":    "file",
				"file_id": fileID,
			},
		},
	}

	payload := RequestPayload{
		Model: "gpt-4-turbo",
		Messages: []Message{
			{
				Role:    "user",
				Content: msgContent,
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", OpenAIAPIURL+"/chat/completions", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var respPayload ResponsePayload
	if err := json.Unmarshal(respBody, &respPayload); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if respPayload.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", respPayload.Error.Message)
	}

	if len(respPayload.Choices) > 0 {
		return respPayload.Choices[0].Message.Content.(string), nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}

// deleteFileFromOpenAI deletes a file from OpenAI's Files API
func deleteFileFromOpenAI(apiKey string, fileID string) error {
	req, err := http.NewRequest("DELETE", OpenAIAPIURL+"/files/"+fileID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete file: status %d", resp.StatusCode)
	}

	return nil
}
