package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
)

// ValidateAPIKey checks if an API key is valid and active
func ValidateAPIKey(key string) (*APIKey, error) {
	var apiKey APIKey

	result := DB.Where("api_key = ?", key).First(&apiKey)
	if result.Error != nil {
		return nil, fmt.Errorf("API key not found: %w", result.Error)
	}

	// Check if active
	if !apiKey.Active {
		return nil, errors.New("API key is not active")
	}

	// Check if expired
	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, errors.New("API key has expired")
	}

	return &apiKey, nil
}

// CreateResult saves an analysis result to the database
func CreateResult(apiKeyID uint, username string, inputType string, filePath string, 
	questionsExtracted []string, aiResponses []string, pdfConfig map[string]interface{}) (*Result, error) {
	
	// Marshal questions and responses to JSON
	questionsJSON, err := json.Marshal(questionsExtracted)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal questions: %w", err)
	}

	responsesJSON, err := json.Marshal(aiResponses)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal responses: %w", err)
	}

	configJSON, err := json.Marshal(pdfConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal PDF config: %w", err)
	}

	result := &Result{
		APIKeyID:           apiKeyID,
		Username:           username,
		InputType:          inputType,
		InputFilePath:      filePath,
		QuestionsExtracted: datatypes.JSON(questionsJSON),
		AIResponses:        datatypes.JSON(responsesJSON),
		PDFConfigUsed:      datatypes.JSON(configJSON),
	}

	if err := DB.Create(result).Error; err != nil {
		return nil, fmt.Errorf("failed to create result: %w", err)
	}

	return result, nil
}

// GetResultByID retrieves a result by its ID
func GetResultByID(resultID uint) (*Result, error) {
	var result Result

	if err := DB.Preload("APIKey").First(&result, resultID).Error; err != nil {
		return nil, fmt.Errorf("result not found: %w", err)
	}

	return &result, nil
}

// ListResultsByUsername retrieves all results for a user (with pagination)
func ListResultsByUsername(username string, limit int, offset int) ([]Result, int64, error) {
	var results []Result
	var total int64

	query := DB.Where("username = ?", username)

	// Get total count
	if err := query.Model(&Result{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count results: %w", err)
	}

	// Get paginated results
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&results).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch results: %w", err)
	}

	return results, total, nil
}

// ListAPIKeys retrieves all API keys (admin function)
func ListAPIKeys(limit int, offset int) ([]APIKey, int64, error) {
	var keys []APIKey
	var total int64

	query := DB

	// Get total count
	if err := query.Model(&APIKey{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count API keys: %w", err)
	}

	// Get paginated results
	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch API keys: %w", err)
	}

	return keys, total, nil
}

// CreateAPIKey creates a new API key with optional expiration
func CreateAPIKey(key string, username string, expiresAt time.Time) (*APIKey, error) {
	apiKey := &APIKey{
		Key:       key,
		Username:  username,
		Active:    true,
		ExpiresAt: &expiresAt,
		RateLimit: 1000, // Default rate limit
	}

	if err := DB.Create(apiKey).Error; err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return apiKey, nil
}

// GetAPIKeysByUsername retrieves all API keys for a specific user
func GetAPIKeysByUsername(username string) ([]APIKey, error) {
	var keys []APIKey

	if err := DB.Where("username = ?", username).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch API keys for user %s: %w", username, err)
	}

	return keys, nil
}

// GetAllAPIKeys retrieves all API keys
func GetAllAPIKeys() ([]APIKey, error) {
	var keys []APIKey

	if err := DB.Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch API keys: %w", err)
	}

	return keys, nil
}

// DeactivateAPIKey deactivates an API key by ID
func DeactivateAPIKey(keyID uint) error {
	return DB.Model(&APIKey{}).Where("id = ?", keyID).Update("active", false).Error
}

// DeleteAPIKey deletes an API key by ID (permanently removes it)
func DeleteAPIKey(keyID uint) error {
	// Delete related results first
	if err := DB.Where("api_key_id = ?", keyID).Delete(&Result{}).Error; err != nil {
		return fmt.Errorf("failed to delete related results: %w", err)
	}

	return DB.Delete(&APIKey{}, keyID).Error
}
