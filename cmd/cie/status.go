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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
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
// Global flags from main:
//   - --json: Output results as JSON (from globals.JSON)
//   - --quiet: Suppress non-essential output (from globals.Quiet)
//
// Examples:
//
//	cie status           Display formatted status
//	cie status --json    Output as JSON for programmatic use
func runStatus(args []string, configPath string, globals GlobalFlags) {
	// 1. Load configuration first to check for EdgeCache
	cfg, cfgErr := LoadConfig(configPath)

	// 2. Check if we should delegate to remote server
	baseURL := os.Getenv("CIE_BASE_URL")
	if baseURL == "" && cfgErr == nil {
		baseURL = cfg.CIE.EdgeCache
	}

	if baseURL != "" {
		runRemoteStatus(baseURL, configPath, globals)
		return
	}

	if cfgErr != nil {
		errors.FatalError(cfgErr, globals.JSON)
	}

	fs := flag.NewFlagSet("status", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie status [options]

Description:
  Display the current status of the CIE project including indexing
  statistics and database health.

  This queries the local CozoDB database to count indexed entities:
  files, functions, types, embeddings, and call graph edges.

  Use this to verify indexing completed successfully and understand
  the scope of your indexed codebase.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Show human-readable status
  cie status

  # Output as JSON for programmatic use
  cie status --json

  # Pipe to jq for specific field extraction
  cie status --json | jq '.functions'

Output Fields:
  - Files:         Number of source files indexed
  - Functions:     Number of functions/methods extracted
  - Types:         Number of types (structs, interfaces, classes)
  - Embeddings:    Number of semantic embeddings generated
  - Call Edges:    Number of function call relationships

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Cannot determine home directory",
			"Operating system failed to provide user home directory path",
			"Check your system configuration or set the HOME environment variable",
			err,
		), globals.JSON)
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
		if globals.JSON {
			outputStatusJSON(result)
		} else {
			ui.Warningf("Project '%s' not indexed yet.", cfg.ProjectID)
			ui.Info("Run 'cie index' to index the repository.")
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
		errors.FatalError(errors.NewDatabaseError(
			"Cannot open CIE database",
			"The database file may be corrupted, locked by another process, or permission denied",
			"Try running 'cie status' again, or run 'cie reset --yes' to rebuild the index",
			err,
		), globals.JSON)
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

	if globals.JSON {
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
	ui.Header("CIE Project Status (Local)")
	fmt.Printf("%s    %s\n", ui.Label("Project ID:"), result.ProjectID)
	fmt.Printf("%s      %s\n", ui.Label("Data Dir:"), ui.DimText(result.DataDir))
	fmt.Println()

	ui.SubHeader("Entities:")
	fmt.Printf("  Files:         %s\n", ui.CountText(result.Files))
	fmt.Printf("  Functions:     %s\n", ui.CountText(result.Functions))
	fmt.Printf("  Types:         %s\n", ui.CountText(result.Types))
	fmt.Printf("  Embeddings:    %s\n", ui.CountText(result.Embeddings))
	fmt.Printf("  Call Edges:    %s\n", ui.CountText(result.CallEdges))

	if result.Error != "" {
		fmt.Println()
		ui.Warning(result.Error)
	}
}

// runRemoteStatus queries the remote CIE server for project status.
func runRemoteStatus(baseURL, configPath string, globals GlobalFlags) {
	// Load configuration to get project_id
	cfg, err := LoadConfig(configPath)
	if err != nil {
		errors.FatalError(err, globals.JSON)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL + "/v1/status")
	if err != nil {
		errors.FatalError(errors.NewNetworkError(
			"Cannot connect to CIE server",
			fmt.Sprintf("Failed to reach %s/v1/status", baseURL),
			"Check that the CIE server is running and CIE_BASE_URL is correct",
			err,
		), globals.JSON)
	}
	defer resp.Body.Close()

	var serverStatus struct {
		ProjectID string `json:"project_id"`
		DataDir   string `json:"data_dir"`
		RepoPath  string `json:"repo_path"`
		Indexed   bool   `json:"indexed"`
		Files     int    `json:"files"`
		Functions int    `json:"functions"`
		Types     int    `json:"types"`
		Error     string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&serverStatus); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Invalid response from server",
			"Could not parse server response",
			"Check server logs for errors",
			err,
		), globals.JSON)
	}

	result := &StatusResult{
		ProjectID: cfg.ProjectID,
		DataDir:   serverStatus.DataDir,
		Connected: serverStatus.Indexed,
		Files:     serverStatus.Files,
		Functions: serverStatus.Functions,
		Types:     serverStatus.Types,
		Timestamp: time.Now(),
	}

	if !serverStatus.Indexed {
		result.Error = "Project not indexed yet. Run 'cie index' first."
	}

	if globals.JSON {
		outputStatusJSON(result)
	} else {
		printRemoteStatus(result, baseURL)
	}
}

// printRemoteStatus prints the remote server status in a human-readable format.
func printRemoteStatus(result *StatusResult, serverURL string) {
	ui.Header("CIE Project Status (Remote)")
	fmt.Printf("%s    %s\n", ui.Label("Project ID:"), result.ProjectID)
	fmt.Printf("%s      %s\n", ui.Label("Server:"), ui.DimText(serverURL))
	fmt.Println()

	if !result.Connected {
		ui.Warningf("Project '%s' not indexed yet.", result.ProjectID)
		ui.Info("Run 'cie index' to index the repository.")
		return
	}

	ui.SubHeader("Entities:")
	fmt.Printf("  Files:         %s\n", ui.CountText(result.Files))
	fmt.Printf("  Functions:     %s\n", ui.CountText(result.Functions))
	fmt.Printf("  Types:         %s\n", ui.CountText(result.Types))

	if result.Error != "" {
		fmt.Println()
		ui.Warning(result.Error)
	}
}
