package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UUIDModel replaces gorm.Model to keep your code architecture identical
type UUIDModel struct {
	ID        string         `gorm:"type:uuid;primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // Keeps GORM's native soft-delete working
}

// BeforeCreate is a GORM hook that auto-generates the UUIDv4 on insert
func (m *UUIDModel) BeforeCreate(tx *gorm.DB) (err error) {
	m.ID = uuid.NewString() // Generates standard UUIDv4 string
	return
}

// BetterAuthModel provides a string-based primary key compatible with Node/TS auth libraries
type BetterAuthModel struct {
	ID        string         `gorm:"type:text;primaryKey"` // Safe for Better Auth string tokens
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index;column:deleted_at"` // Preserves your GORM soft-deletes
}

// BeforeCreate hooks into GORM's lifecycle to generate a string ID if empty
func (m *BetterAuthModel) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == "" {
		// Better Auth generates text strings. For Go side creations, 
		// generating a standard stringified UUIDv4 works perfectly over text columns.
		m.ID = uuid.NewString() 
	}
	return
}

type User struct {
	BetterAuthModel
	Username      string `gorm:"unique;not null"`
	Password      string `gorm:"not null"`
	APIKey        string `gorm:"unique;not null;index;column:api_key"`
	
	// Better Auth Mandatory Fields
	Name          string `gorm:"type:text;not null;default:''"`
	Email         string `gorm:"type:text;unique;not null"`
	EmailVerified bool   `gorm:"column:email_verified;default:false;not null"`
	Image         string `gorm:"type:text"`
	IsActivated   bool   `gorm:"column:is_activated;default:false;not null"`
}

// TableName forces GORM to use singular to match Better Auth's default setup
func (User) TableName() string {
	return "user"
}
type Exam struct {
	UUIDModel          // Replaces gorm.Model
	UserID    string   `gorm:"type:uuid"` // Updated from uint to string
	Status    string   `gorm:"default:'pending'"`
	Results   []AnalyzeResult
	OriginalFile string   `gorm:"type:text"`
}

type AnalyzeResult struct {
	UUIDModel          // Replaces gorm.Model
	ExamID           string `gorm:"type:uuid"` // Updated from uint to string
	SampleID         int
	ModelUsed        string `gorm:"not null"`
	PromptTokens     int
	CompletionTokens int
	Response         string `gorm:"type:text"`
}