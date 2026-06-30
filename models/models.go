package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null"`
	Password string `gorm:"not null"`
	APIKey   string `gorm:"unique;not null;index"`
}

type Exam struct {
	gorm.Model
	UserID uint
	Status string `gorm:"default:'pending'"` // processed, analyzed, failed
	Results []AnalyzeResult
}

type AnalyzeResult struct {
	gorm.Model
	ExamID           uint
	SampleID         int
	ModelUsed        string `gorm:"not null"`
	PromptTokens     int
	CompletionTokens int
	Response         string `gorm:"type:text"`
}