// Copyright 2025 KrakLabs
//
// Sample Go code for testing parsing and indexing.

package example

import (
	"context"
	"fmt"
)

// User represents a user in the system.
type User struct {
	ID   string
	Name string
	Email string
}

// UserService handles user operations.
type UserService interface {
	GetUser(ctx context.Context, id string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
}

// userServiceImpl implements UserService.
type userServiceImpl struct {
	db Database
}

// NewUserService creates a new user service.
func NewUserService(db Database) UserService {
	return &userServiceImpl{db: db}
}

// GetUser retrieves a user by ID.
func (s *userServiceImpl) GetUser(ctx context.Context, id string) (*User, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	user, err := s.db.Query(ctx, "SELECT * FROM users WHERE id = ?", id)
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	return user, nil
}

// CreateUser creates a new user.
func (s *userServiceImpl) CreateUser(ctx context.Context, user *User) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}

	if err := s.db.Insert(ctx, "users", user); err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

// Database interface for data access.
type Database interface {
	Query(ctx context.Context, query string, args ...any) (*User, error)
	Insert(ctx context.Context, table string, data any) error
}
