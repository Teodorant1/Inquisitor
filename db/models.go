package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
)

// APIKey represents an API key for the SaaS service
type APIKey struct {
	ID        uint              `gorm:"primaryKey"`
	Key       string            `gorm:"column:api_key;uniqueIndex;not null"`
	Username  string            `gorm:"column:username;not null"`
	Active    bool              `gorm:"column:active;default:true"`
	CreatedAt time.Time         `gorm:"column:created_at;autoCreateTime"`
	ExpiresAt *time.Time        `gorm:"column:expires_at"`
	RateLimit int               `gorm:"column:rate_limit;default:1000"` // requests per day
	Results   []Result          `gorm:"foreignKey:APIKeyID"`
}

// TableName specifies the table name
func (APIKey) TableName() string {
	return "api_keys"
}

// Result represents an archived analysis result
type Result struct {
    ID                 uint            `gorm:"primaryKey"`
    APIKeyID           uint            `gorm:"column:api_key_id;not null;index"`
    Username           string          `gorm:"column:username;not null"`
    InputType          string          `gorm:"column:input_type;not null"` 
    InputFilePath      string          `gorm:"column:input_file_path"`
    
    // Put these back to datatypes.JSON to stop the compiler errors!
    QuestionsExtracted datatypes.JSON  `gorm:"column:questions_extracted;type:text"` 
    AIResponses        datatypes.JSON  `gorm:"column:ai_responses;type:text"`        
    
    PDFConfigUsed      datatypes.JSON  `gorm:"column:pdf_config_used;type:text"` 
    CreatedAt          time.Time       `gorm:"column:created_at;autoCreateTime"`
    APIKey             APIKey          `gorm:"foreignKey:APIKeyID"`
}
// TableName specifies the table name
func (Result) TableName() string {
	return "results"
}

// StringSliceJSON is a type to help marshal/unmarshal string slices to/from JSON
type StringSliceJSON []string

func (s StringSliceJSON) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *StringSliceJSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion failed")
	}
	return json.Unmarshal(bytes, &s)
}

// ErrorLog represents an error event for tracking and debugging
type ErrorLog struct {
	ID           uint      `gorm:"primaryKey"`
	RequestID    string    `gorm:"column:request_id;index"`
	APIKeyID     *uint     `gorm:"column:api_key_id"`
	Username     string    `gorm:"column:username;index"`
	Endpoint     string    `gorm:"column:endpoint"`
	ErrorCode    string    `gorm:"column:error_code"` // e.g. "err_pdf_001"
	ErrorMessage string    `gorm:"column:error_message;type:text"`
	StackTrace   string    `gorm:"column:stack_trace;type:text"`
	StatusCode   int       `gorm:"column:status_code"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName specifies the table name
func (ErrorLog) TableName() string {
	return "error_logs"
}

// Job represents a background processing job for async analysis
type Job struct {
	ID           uint       `gorm:"primaryKey"`
	ResultID     uint       `gorm:"column:result_id;uniqueIndex;not null"`
	Status       string     `gorm:"column:status;not null;default:pending"` // pending, processing, completed, failed
	ErrorMessage string     `gorm:"column:error_message;type:text"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime"`
	StartedAt    *time.Time `gorm:"column:started_at"`
	CompletedAt  *time.Time `gorm:"column:completed_at"`
	Result       Result     `gorm:"foreignKey:ResultID"`
}

// TableName specifies the table name
func (Job) TableName() string {
	return "jobs"
}
