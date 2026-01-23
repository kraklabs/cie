package handlers

import (
	"fmt"
	"github.com/example/sample/internal/db"
)

// HandleUser handles user management requests
func HandleUser(database *db.DB) error {
	fmt.Println("Handling user operations...")

	// Get user details
	user, err := GetUser("user123", database)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	fmt.Printf("Retrieved user: %s\n", user)
	return nil
}

// GetUser retrieves a user by ID
func GetUser(userID string, database *db.DB) (string, error) {
	fmt.Printf("Getting user: %s\n", userID)

	// Query user from database
	err := database.Query("SELECT * FROM users WHERE id = ?")
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	return userID, nil
}
