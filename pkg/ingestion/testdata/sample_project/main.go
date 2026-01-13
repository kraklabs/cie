// Sample Go project for multi-file parsing tests.
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/example/sample/handlers"
)

func main() {
	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/health", handlers.HandleHealth)
	mux.HandleFunc("/api/users", handlers.HandleUsers)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// setupLogging configures application logging.
func setupLogging() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	fmt.Println("Logging configured")
}
