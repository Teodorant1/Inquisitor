package api

import (
	"log"
	"time"

	"Inquisitor/config"
	"Inquisitor/db"
)

// Cleanup runs background maintenance tasks
type Cleanup struct {
	config *config.Config
	ticker *time.Ticker
}

// NewCleanup creates a new cleanup routine
func NewCleanup(cfg *config.Config) *Cleanup {
	return &Cleanup{
		config: cfg,
	}
}

// Start begins the cleanup routine
func (c *Cleanup) Start() {
	if !c.config.CleanupEnabled {
		log.Println("Cleanup disabled")
		return
	}

	c.ticker = time.NewTicker(c.config.CleanupInterval)
	go c.cleanupLoop()
	log.Printf("Cleanup routine started (interval: %v, max age: %v)", c.config.CleanupInterval, c.config.MaxFileAge)
}

// cleanupLoop runs periodic cleanup tasks
func (c *Cleanup) cleanupLoop() {
	for range c.ticker.C {
		c.runCleanup()
	}
}

// runCleanup executes all cleanup tasks
func (c *Cleanup) runCleanup() {
	// Clean up old temp files
	deleted, err := db.CleanupOldFiles(c.config.TempDir, c.config.MaxFileAge)
	if err != nil {
		log.Printf("Error during file cleanup: %v", err)
	} else if deleted > 0 {
		log.Printf("Cleaned up %d old temp files", deleted)
	}

	// Clean up old error logs (keep for 7 days)
	errorsCleaned, err := db.CleanupOldErrorLogs(7 * 24 * time.Hour)
	if err != nil {
		log.Printf("Error during error log cleanup: %v", err)
	} else if errorsCleaned > 0 {
		log.Printf("Cleaned up %d old error logs", errorsCleaned)
	}
}

// Stop stops the cleanup routine
func (c *Cleanup) Stop() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
}
