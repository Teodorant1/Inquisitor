package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Initialize sets up the database connection and runs migrations
func Initialize(databaseURL string) error {
	if databaseURL == "" {
		databaseURL = "inquisitor.db" // default SQLite file
	}

	var db *gorm.DB
	var err error

	// Detect database type and connect
	if strings.HasPrefix(databaseURL, "postgres") {
		// PostgreSQL connection
		db, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	} else {
		// SQLite connection
		db, err = gorm.Open(sqlite.Open(databaseURL), &gorm.Config{})
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// Run auto-migrations
	err = db.AutoMigrate(&APIKey{}, &Result{}, &Job{}, &ErrorLog{})
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	DB = db
	log.Printf("Database initialized successfully: %s", databaseURL)
	return nil
}

// Close closes the database connection
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Reset drops and recreates all tables (dev/testing only)
func Reset() error {
	return DB.Migrator().DropTable(&ErrorLog{}, &Job{}, &Result{}, &APIKey{})
}

// CleanupOldFiles deletes uploaded files older than the specified duration
func CleanupOldFiles(tempDir string, maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read temp directory: %w", err)
	}

	cutoffTime := time.Now().Add(-maxAge)
	deleted := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoffTime) {
				filePath := filepath.Join(tempDir, entry.Name())
				if err := os.Remove(filePath); err == nil {
					deleted++
				}
			}
		}
	}

	return deleted, nil
}
