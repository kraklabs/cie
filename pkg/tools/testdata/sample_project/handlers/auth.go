package handlers

import (
	"fmt"
	"github.com/example/sample/internal/db"
)

// HandleAuth handles authentication requests
func HandleAuth(database *db.DB) error {
	fmt.Println("Handling authentication...")

	// Query user credentials
	err := database.Query("SELECT * FROM users WHERE active = true")
	if err != nil {
		return fmt.Errorf("auth query failed: %w", err)
	}

	return nil
}

// HandleLogin handles user login
func HandleLogin(username, password string, database *db.DB) error {
	fmt.Printf("Logging in user: %s\n", username)

	// Validate credentials
	err := database.Query("SELECT * FROM users WHERE username = ?")
	if err != nil {
		return fmt.Errorf("login query failed: %w", err)
	}

	return nil
}
