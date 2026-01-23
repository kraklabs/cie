package db

import "fmt"

// DB represents a database connection
type DB struct {
	connected bool
}

// NewDB creates a new database instance
func NewDB() *DB {
	fmt.Println("Creating new database connection...")
	return &DB{connected: true}
}

// Query executes a database query
func (d *DB) Query(query string) error {
	if !d.connected {
		return fmt.Errorf("database not connected")
	}
	fmt.Printf("Executing query: %s\n", query)
	return nil
}

// Exec executes a database command
func (d *DB) Exec(command string) error {
	if !d.connected {
		return fmt.Errorf("database not connected")
	}
	fmt.Printf("Executing command: %s\n", command)
	return nil
}

// Close closes the database connection
func (d *DB) Close() error {
	fmt.Println("Closing database connection...")
	d.connected = false
	return nil
}
