package main

import (
	"fmt"
	"github.com/example/sample/handlers"
	"github.com/example/sample/internal/db"
)

func main() {
	database := db.NewDB()

	// Handle authentication
	handlers.HandleAuth(database)

	// Handle user operations
	handlers.HandleUser(database)

	fmt.Println("Application started successfully")
}
