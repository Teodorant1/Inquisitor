package main

import (
	"Inquisitor/config"
	"Inquisitor/db"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// FIX: Robust production environment discovery.
	// 1. First, try loading a local .env file in the current working workspace directory.
	// 2. If it fails, it will still naturally pull from the system environment variables on your VPS.
	if err := godotenv.Load(); err != nil {
		// Non-fatal warning log just in case you run it via environment variables instead of a file
		log.Println("Note: No local .env file found, falling back to system environment variables.")
	}

	// Load config
	cfg := config.Load()

	// Initialize database
	if err := db.Initialize(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "create-key":
		handleCreateKey()
	case "list-keys":
		handleListKeys()
	case "revoke-key":
		handleRevokeKey()
	case "delete-key":
		handleDeleteKey()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Inquisitor Admin CLI

Usage:
  inquisitor-admin create-key <username> [expiration-days]
  inquisitor-admin list-keys [username]
  inquisitor-admin revoke-key <key>
  inquisitor-admin delete-key <key-id>

Commands:
  create-key         Generate a new API key for a user
  list-keys          List API keys (optionally filter by username)
  revoke-key         Deactivate an API key (mark as inactive)
  delete-key         Permanently delete an API key

Examples:
  inquisitor-admin create-key john.doe 30
  inquisitor-admin list-keys john.doe
  inquisitor-admin revoke-key dGVzdGtleWFzZGZhc2Rm
  inquisitor-admin delete-key 1
`)
}

func handleCreateKey() {
	fs := flag.NewFlagSet("create-key", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Println("Error: username required")
		fmt.Println("Usage: inquisitor-admin create-key <username> [expiration-days]")
		os.Exit(1)
	}

	username := args[0]
	expirationDays := 90 // default 90 days

	if len(args) > 1 {
		days, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("Error: expiration-days must be a number")
			os.Exit(1)
		}
		expirationDays = days
	}

	// Generate random API key
	apiKey := generateAPIKey()

	// Calculate expiration date
	expiresAt := time.Now().AddDate(0, 0, expirationDays)

	// Create API key in database
	key, err := db.CreateAPIKey(apiKey, username, expiresAt)
	if err != nil {
		log.Fatalf("Failed to create API key: %v", err)
	}

	fmt.Printf("✓ API key created successfully\n\n")
	fmt.Printf("Username:       %s\n", username)
	fmt.Printf("API Key:        %s\n", apiKey)
	fmt.Printf("Key ID:         %d\n", key.ID)
	fmt.Printf("Status:         Active\n")
	fmt.Printf("Created:        %s\n", key.CreatedAt.Format(time.RFC3339))
	// fmt.Printf("Expires:        %s\n", key.ExpiresAt.Format(time.RFC3339))
	fmt.Println("\n⚠️  Save this API key securely - you won't be able to see it again!")
}

func handleListKeys() {
	var username string

	fs := flag.NewFlagSet("list-keys", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) > 0 {
		username = args[0]
	}

	var keys []db.APIKey
	var err error

	if username != "" {
		// Get keys for specific user
		keys, err = db.GetAPIKeysByUsername(username)
		if err != nil {
			log.Fatalf("Failed to retrieve API keys: %v", err)
		}
		fmt.Printf("API Keys for user '%s':\n\n", username)
	} else {
		// Get all keys
		keys, err = db.GetAllAPIKeys()
		if err != nil {
			log.Fatalf("Failed to retrieve API keys: %v", err)
		}
		fmt.Println("All API Keys:\n")
	}

	if len(keys) == 0 {
		fmt.Println("No API keys found.")
		return
	}

	// Print table header
	fmt.Printf("%-4s %-15s %-40s %-8s %-20s %-20s\n",
		"ID", "Username", "Key", "Status", "Created", "Expires")
	fmt.Println(string(make([]byte, 120)))

	// Print keys
	for _, key := range keys {
		status := "Active"
		if !key.Active {
			status = "Revoked"
		}

		// Truncate key for display
		displayKey := key.Key
		if len(displayKey) > 36 {
			displayKey = displayKey[:36] + "..."
		}

		fmt.Printf("%-4d %-15s %-40s %-8s %-20s %-20s\n",
			key.ID,
			key.Username,
			displayKey,
			status,
			key.CreatedAt.Format("2006-01-02"),
			// key.ExpiresAt.Format("2006-01-02"),
		)
	}
}

func handleRevokeKey() {
	fs := flag.NewFlagSet("revoke-key", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Println("Error: API key required")
		fmt.Println("Usage: inquisitor-admin revoke-key <key>")
		os.Exit(1)
	}

	apiKey := args[0]

	// Get the full key object first to get ID
	key, err := db.ValidateAPIKey(apiKey)
	if err != nil {
		fmt.Println("Error: API key not found")
		os.Exit(1)
	}

	// Deactivate the key
	err = db.DeactivateAPIKey(key.ID)
	if err != nil {
		log.Fatalf("Failed to revoke API key: %v", err)
	}

	fmt.Printf("✓ API key revoked successfully\n\n")
	fmt.Printf("Key ID:         %d\n", key.ID)
	fmt.Printf("Username:       %s\n", key.Username)
	fmt.Printf("Status:         Revoked\n")
}

func handleDeleteKey() {
	fs := flag.NewFlagSet("delete-key", flag.ExitOnError)
	fs.Parse(os.Args[2:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Println("Error: key ID required")
		fmt.Println("Usage: inquisitor-admin delete-key <key-id>")
		os.Exit(1)
	}

	keyID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		fmt.Println("Error: key ID must be a number")
		os.Exit(1)
	}

	// Delete the key
	err = db.DeleteAPIKey(uint(keyID))
	if err != nil {
		log.Fatalf("Failed to delete API key: %v", err)
	}

	fmt.Printf("✓ API key deleted successfully\n\n")
	fmt.Printf("Key ID: %d\n", keyID)
}

// generateAPIKey creates a secure random API key
func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Failed to generate API key: %v", err)
	}
	return hex.EncodeToString(b)
}