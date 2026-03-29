package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	// Database
	DatabaseURL string

	// API
	APIPort int
	APIHost string

	// File handling
	TempDir string

	// OpenAI
	OpenAIKey string

	// CORS
	FrontendDomain string // Domain to accept requests from (e.g., https://app.example.com)

	// Environment
	Env string // "dev" or "prod"

	// Startup mode
	CLIMode bool // if true, run CLI mode; otherwise run API server

	// Cleanup
	CleanupEnabled bool          // Enable automatic temp file cleanup
	CleanupInterval time.Duration // How often to run cleanup (default 1 hour)
	MaxFileAge     time.Duration // Max age of temp files before deletion (default 24 hours)
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	return &Config{
		DatabaseURL:     getEnv("DATABASE_URL", "inquisitor.db"),
		APIPort:         getEnvInt("API_PORT", 8080),
		APIHost:         getEnv("API_HOST", "127.0.0.1"),
		TempDir:         getEnv("TEMP_DIR", "./tmp"),
		OpenAIKey:       getEnv("OPENAI_API_KEY", ""),
		FrontendDomain:  getEnv("FRONTEND_DOMAIN", "http://localhost:3000"),
		Env:             getEnv("ENVIRONMENT", "dev"),
		CLIMode:         getEnvBool("CLI_MODE", false),
		CleanupEnabled:  getEnvBool("CLEANUP_ENABLED", true),
		CleanupInterval: getEnvDuration("CLEANUP_INTERVAL", 1*time.Hour),
		MaxFileAge:      getEnvDuration("MAX_FILE_AGE", 24*time.Hour),
	}
}

// Helper functions
func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}
