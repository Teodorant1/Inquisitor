package db

import (
	"fmt"
	"time"
)

// LogError stores an error event in the database
func LogError(requestID, endpoint, errorCode, errorMsg, stackTrace string, statusCode int, apiKeyID *uint, username string) error {
	errorLog := &ErrorLog{
		RequestID:    requestID,
		APIKeyID:     apiKeyID,
		Username:     username,
		Endpoint:     endpoint,
		ErrorCode:    errorCode,
		ErrorMessage: errorMsg,
		StackTrace:   stackTrace,
		StatusCode:   statusCode,
	}

	if err := DB.Create(errorLog).Error; err != nil {
		return fmt.Errorf("failed to log error: %w", err)
	}

	return nil
}

// GetErrorByRequestID retrieves an error log by request ID
func GetErrorByRequestID(requestID string) (*ErrorLog, error) {
	var errorLog ErrorLog

	if err := DB.Where("request_id = ?", requestID).First(&errorLog).Error; err != nil {
		return nil, fmt.Errorf("error log not found: %w", err)
	}

	return &errorLog, nil
}

// ListErrorsByUsername retrieves error logs for a user (paginated)
func ListErrorsByUsername(username string, limit int, offset int) ([]ErrorLog, int64, error) {
	var errors []ErrorLog
	var total int64

	query := DB.Where("username = ?", username)

	if err := query.Model(&ErrorLog{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count errors: %w", err)
	}

	if err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&errors).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch errors: %w", err)
	}

	return errors, total, nil
}

// CleanupOldErrorLogs deletes error logs older than the specified duration
func CleanupOldErrorLogs(maxAge time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-maxAge)

	result := DB.Where("created_at < ?", cutoffTime).Delete(&ErrorLog{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup error logs: %w", result.Error)
	}

	return result.RowsAffected, nil
}
