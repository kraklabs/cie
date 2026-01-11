// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For commercial licensing, contact: licensing@kraklabs.com
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kraklabs/cie/pkg/storage"
)

// StatusResult represents the project status for JSON output.
type StatusResult struct {
	ProjectID  string    `json:"project_id"`
	DataDir    string    `json:"data_dir"`
	Connected  bool      `json:"connected"`
	Files      int       `json:"files"`
	Functions  int       `json:"functions"`
	Types      int       `json:"types"`
	Embeddings int       `json:"embeddings"`
	CallEdges  int       `json:"call_edges"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// runStatus executes the 'status' CLI command, displaying project index statistics.
//
// It queries the local CozoDB database to count indexed files, functions, types,
// embeddings, and call graph edges. This helps users verify that indexing completed
// successfully and understand the scope of their indexed codebase.
//
// Flags:
//   - --json: Output results as JSON (default: false)
//
// Examples:
//
//	cie status           Display formatted status
//	cie status --json    Output as JSON for programmatic use
func runStatus(args []string, configPath string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie status [options]

Shows local project status.

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		if *jsonOutput {
			outputStatusJSON(&StatusResult{
				Connected: false,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		if *jsonOutput {
			outputStatusJSON(&StatusResult{
				ProjectID: cfg.ProjectID,
				Connected: false,
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
	dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)

	result := &StatusResult{
		ProjectID: cfg.ProjectID,
		DataDir:   dataDir,
		Timestamp: time.Now(),
	}

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		result.Connected = false
		result.Error = "Project not indexed yet. Run 'cie index' first."
		if *jsonOutput {
			outputStatusJSON(result)
		} else {
			fmt.Printf("Project '%s' not indexed yet.\n", cfg.ProjectID)
			fmt.Println("Run 'cie index' to index the repository.")
		}
		os.Exit(0)
	}

	// Open local backend
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		DataDir:   dataDir,
		Engine:    "rocksdb",
		ProjectID: cfg.ProjectID,
	})
	if err != nil {
		result.Connected = false
		result.Error = fmt.Sprintf("Cannot open database: %v", err)
		if *jsonOutput {
			outputStatusJSON(result)
		} else {
			fmt.Fprintf(os.Stderr, "Error: cannot open database: %v\n", err)
		}
		os.Exit(1)
	}
	defer func() { _ = backend.Close() }()

	result.Connected = true
	ctx := context.Background()

	// Query counts
	result.Files = queryLocalCount(ctx, backend, "cie_file", "id")
	result.Functions = queryLocalCount(ctx, backend, "cie_function", "id")
	result.Types = queryLocalCount(ctx, backend, "cie_type", "id")
	result.Embeddings = queryLocalCount(ctx, backend, "cie_function_embedding", "function_id")
	result.CallEdges = queryLocalCount(ctx, backend, "cie_calls", "id")

	if *jsonOutput {
		outputStatusJSON(result)
	} else {
		printLocalStatus(result)
	}
}

// queryLocalCount queries the local CozoDB database to count rows in a table.
//
// It executes a Datalog query to count distinct values of the specified primary key field.
// Returns 0 if the query fails or returns no results.
//
// Parameters:
//   - ctx: Context for the query
//   - backend: EmbeddedBackend with CozoDB connection
//   - table: CozoDB table name (e.g., "cie_function")
//   - pkField: Primary key field name to count (e.g., "id")
func queryLocalCount(ctx context.Context, backend *storage.EmbeddedBackend, table, pkField string) int {
	script := fmt.Sprintf("?[count(%s)] := *%s { %s }", pkField, table, pkField)
	result, err := backend.Query(ctx, script)
	if err != nil {
		return 0
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return 0
	}

	switch v := result.Rows[0][0].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

// outputStatusJSON writes the status result as formatted JSON to stdout.
//
// Used when the --json flag is provided for programmatic consumption
// or integration with other tools.
func outputStatusJSON(result *StatusResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
}

// printLocalStatus prints the status result as formatted text to stdout.
//
// Displays project information and entity counts in a human-readable format.
// This is the default output when --json is not specified.
func printLocalStatus(result *StatusResult) {
	fmt.Println("CIE Project Status (Local)")
	fmt.Println("==========================")
	fmt.Printf("Project ID:    %s\n", result.ProjectID)
	fmt.Printf("Data Dir:      %s\n", result.DataDir)
	fmt.Println()

	fmt.Println("Entities:")
	fmt.Printf("  Files:         %d\n", result.Files)
	fmt.Printf("  Functions:     %d\n", result.Functions)
	fmt.Printf("  Types:         %d\n", result.Types)
	fmt.Printf("  Embeddings:    %d\n", result.Embeddings)
	fmt.Printf("  Call Edges:    %d\n", result.CallEdges)

	if result.Error != "" {
		fmt.Printf("\nWarning: %s\n", result.Error)
	}
}
