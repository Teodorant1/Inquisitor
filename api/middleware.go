package api

import (
	"Inquisitor/db"
)

// validateAPIKey checks if the API key is valid
func validateAPIKey(key string) (*db.APIKey, error) {
	return db.ValidateAPIKey(key)
}
