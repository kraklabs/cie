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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/pkg/storage"
)

// runQuery executes the 'query' CLI command, running CozoScript queries on the indexed codebase.
//
// It opens the local CozoDB database and executes the provided Datalog query, returning results
// as either formatted tables (default) or JSON for programmatic use.
//
// Global flags from main:
//   - --json: Output results as JSON (from globals.JSON)
//   - --quiet: Suppress non-essential output (from globals.Quiet)
//
// Command-specific flags:
//   - --timeout: Query timeout duration (default: 30s)
//   - --limit: Add :limit clause to query (default: 0, no limit)
//
// Examples:
//
//	cie query '?[name, file] := *cie_function{ name, file_path: file } :limit 10'
//	cie query '?[name] := *cie_function{ name }' --json
//	cie query '?[count(id)] := *cie_function{ id }' --timeout 60s
func runQuery(args []string, configPath string, globals GlobalFlags) {
	// 1. Load configuration first to check for EdgeCache
	cfg, cfgErr := LoadConfig(configPath)

	// 2. Check if we should delegate to remote server
	baseURL := os.Getenv("CIE_BASE_URL")
	if baseURL == "" && cfgErr == nil {
		baseURL = cfg.CIE.EdgeCache
	}

	if baseURL != "" {
		runRemoteQuery(baseURL, args, globals)
		return
	}

	if cfgErr != nil {
		errors.FatalError(cfgErr, globals.JSON)
	}

	fs := flag.NewFlagSet("query", flag.ExitOnError)
	timeout := fs.Duration("timeout", 30*time.Second, "Query timeout")
	limit := fs.Int("limit", 0, "Add :limit to query (0 = no limit)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie query [options] <cozoscript>

Description:
  Execute a CozoScript query against the indexed codebase database.

  CozoScript is a Datalog-based query language that allows powerful
  graph queries over your code structure. Use this for advanced code
  analysis beyond what the MCP tools provide.

  Results can be formatted as tables (default) or JSON for programmatic use.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # List all functions with file paths
  cie query "?[name, file] := *cie_function{ name, file_path: file }" --limit 10

  # Search functions by name pattern (case-insensitive regex)
  cie query "?[name] := *cie_function{ name }, regex_matches(name, '(?i)embed')"

  # Count total files indexed
  cie query "?[count(id)] := *cie_file{ id }"

  # Find all callers of a specific function
  cie query "?[caller] := *cie_calls{ caller_id, callee_id },
    *cie_function{ id: callee_id, name: 'NewPipeline' },
    *cie_function{ id: caller_id, name: caller }"

  # Output as JSON for scripting
  cie query "?[name] := *cie_function{ name }" --json | jq '.rows[][0]'

Notes:
  Query timeout defaults to 30s. Increase with --timeout flag for complex queries.
  See docs/tools-reference.md for complete schema and query patterns.

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		errors.FatalError(errors.NewInputError(
			"Script argument required",
			"No CozoScript query provided",
			"Provide a query: cie query '?[name] := *cie_function{name}'",
		), globals.JSON)
	}

	script := fs.Arg(0)

	// Add limit if specified
	if *limit > 0 {
		script = strings.TrimSpace(script)
		if !strings.Contains(strings.ToLower(script), ":limit") {
			script = fmt.Sprintf("%s :limit %d", script, *limit)
		}
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Cannot determine home directory",
			"Operating system did not provide user home directory path",
			"Check your system configuration or set HOME environment variable",
			err,
		), globals.JSON)
	}
	dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		errors.FatalError(errors.NewDatabaseError(
			fmt.Sprintf("Project '%s' not indexed yet", cfg.ProjectID),
			"The CIE database does not exist for this project",
			"Run 'cie index' to index the repository first",
			err,
		), globals.JSON)
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
			"The database file may be corrupted or locked by another process",
			"Try running 'cie status' to check database health, or 'cie reset' to rebuild",
			err,
		), globals.JSON)
	}
	defer func() { _ = backend.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := backend.Query(ctx, script)
	if err != nil {
		// Distinguish between syntax errors and execution errors
		if strings.Contains(err.Error(), "parse") || strings.Contains(err.Error(), "syntax") {
			errors.FatalError(errors.NewInputError(
				"Invalid CozoScript query syntax",
				fmt.Sprintf("Query parsing failed: %v", err),
				"Check the CozoScript documentation or run 'cie query --help' for examples",
			), globals.JSON)
		}
		errors.FatalError(errors.NewDatabaseError(
			"Query execution failed",
			fmt.Sprintf("Database returned an error: %v", err),
			"Check your query syntax and ensure the database is not corrupted",
			err,
		), globals.JSON)
	}

	// Warn about empty results in non-JSON mode
	if len(result.Rows) == 0 && !globals.JSON {
		fmt.Fprintf(os.Stderr, "Warning: Query returned no results\n")
		fmt.Fprintf(os.Stderr, "Hint: Try broadening your query or verify the database is indexed with 'cie status'\n")
	}

	if globals.JSON {
		outputQueryJSON(result)
	} else {
		printQueryResult(result)
	}
}

// outputQueryJSON writes query results as formatted JSON to stdout.
//
// Includes column headers and rows. Used when --json flag is provided.
func outputQueryJSON(result *storage.QueryResult) {
	output := map[string]any{
		"headers": result.Headers,
		"rows":    result.Rows,
		"count":   len(result.Rows),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
}

// printQueryResult prints query results as a formatted table to stdout.
//
// Uses tab-aligned columns for readability. This is the default output format
// when --json is not specified.
func printQueryResult(result *storage.QueryResult) {
	if len(result.Rows) == 0 {
		fmt.Println("No results")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	for i, h := range result.Headers {
		if i > 0 {
			_, _ = fmt.Fprint(w, "\t")
		}
		_, _ = fmt.Fprint(w, strings.ToUpper(h))
	}
	_, _ = fmt.Fprintln(w)

	// Print separator
	for i := range result.Headers {
		if i > 0 {
			_, _ = fmt.Fprint(w, "\t")
		}
		_, _ = fmt.Fprint(w, "---")
	}
	_, _ = fmt.Fprintln(w)

	// Print rows
	for _, row := range result.Rows {
		for i, cell := range row {
			if i > 0 {
				_, _ = fmt.Fprint(w, "\t")
			}
			_, _ = fmt.Fprint(w, formatCell(cell))
		}
		_, _ = fmt.Fprintln(w)
	}

	_ = w.Flush()

	fmt.Printf("\n(%d rows)\n", len(result.Rows))
}

// formatCell formats a single cell value for display in the query result table.
//
// Handles various CozoDB value types including strings, numbers, booleans, and null.
// Returns a string representation suitable for terminal output.
func formatCell(v any) string {
	switch val := v.(type) {
	case string:
		// Truncate long strings
		if len(val) > 60 {
			return val[:57] + "..."
		}
		return val
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%.2f", val)
	case nil:
		return "<null>"
	default:
		s := fmt.Sprintf("%v", val)
		if len(s) > 60 {
			return s[:57] + "..."
		}
		return s
	}
}

// runRemoteQuery executes a query on the remote CIE server.
func runRemoteQuery(baseURL string, args []string, globals GlobalFlags) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	timeout := fs.Duration("timeout", 30*time.Second, "Query timeout")
	limit := fs.Int("limit", 0, "Add :limit to query (0 = no limit)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		errors.FatalError(errors.NewInputError(
			"Script argument required",
			"No CozoScript query provided",
			"Provide a query: cie query '?[name] := *cie_function{name}'",
		), globals.JSON)
	}

	script := fs.Arg(0)
	if *limit > 0 {
		script = strings.TrimSpace(script)
		if !strings.Contains(strings.ToLower(script), ":limit") {
			script = fmt.Sprintf("%s :limit %d", script, *limit)
		}
	}

	payload := map[string]any{
		"script":  script,
		"timeout": timeout.Seconds(),
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: *timeout + 2*time.Second}
	resp, err := client.Post(baseURL+"/v1/query", "application/json", bytes.NewReader(body))
	if err != nil {
		errors.FatalError(errors.NewNetworkError(
			"Cannot connect to CIE server",
			fmt.Sprintf("Failed to reach %s/v1/query", baseURL),
			"Check that the CIE server is running and CIE_BASE_URL is correct",
			err,
		), globals.JSON)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		errors.FatalError(errors.NewDatabaseError(
			"Remote query failed",
			errResp.Error,
			"Check your query syntax and server logs",
			nil,
		), globals.JSON)
	}

	var result storage.QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Invalid response from server",
			"Could not parse server response",
			"Check server logs for errors",
			err,
		), globals.JSON)
	}

	if globals.JSON {
		outputQueryJSON(&result)
	} else {
		printQueryResult(&result)
	}
}
