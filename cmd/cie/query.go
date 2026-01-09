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
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kraklabs/cie/pkg/storage"
)

func runQuery(args []string, configPath string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	timeout := fs.Duration("timeout", 30*time.Second, "Query timeout")
	limit := fs.Int("limit", 0, "Add :limit to query (0 = no limit)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie query [options] <cozoscript>

Executes a CozoScript query against the local CIE database.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # List all functions
  cie query "?[name, file_path] := *cie_function { name, file_path }" --limit 10

  # Search by name
  cie query "?[name, file_path] := *cie_function { name, file_path }, regex_matches(name, '(?i)embed')"

  # Count files
  cie query "?[count(id)] := *cie_file { id }"

  # Find callers of a function
  cie query "?[caller] := *cie_calls { caller_id, callee_id }, *cie_function { id: callee_id, name: 'NewPipeline' }, *cie_function { id: caller_id, name: caller }"

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if fs.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: script argument required\n")
		fs.Usage()
		os.Exit(1)
	}

	script := fs.Arg(0)

	// Add limit if specified
	if *limit > 0 {
		script = strings.TrimSpace(script)
		if !strings.Contains(strings.ToLower(script), ":limit") {
			script = fmt.Sprintf("%s :limit %d", script, *limit)
		}
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		if *jsonOutput {
			outputQueryError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		if *jsonOutput {
			outputQueryError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
	dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := fmt.Errorf("project '%s' not indexed yet. Run 'cie index' first", cfg.ProjectID)
		if *jsonOutput {
			outputQueryError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	// Open local backend
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		DataDir:   dataDir,
		Engine:    "rocksdb",
		ProjectID: cfg.ProjectID,
	})
	if err != nil {
		if *jsonOutput {
			outputQueryError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: cannot open database: %v\n", err)
		}
		os.Exit(1)
	}
	defer backend.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := backend.Query(ctx, script)
	if err != nil {
		if *jsonOutput {
			outputQueryError(err)
		} else {
			fmt.Fprintf(os.Stderr, "Error: query failed: %v\n", err)
		}
		os.Exit(1)
	}

	if *jsonOutput {
		outputQueryJSON(result)
	} else {
		printQueryResult(result)
	}
}

func outputQueryError(err error) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{
		"error": err.Error(),
	})
}

func outputQueryJSON(result *storage.QueryResult) {
	output := map[string]any{
		"headers": result.Headers,
		"rows":    result.Rows,
		"count":   len(result.Rows),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func printQueryResult(result *storage.QueryResult) {
	if len(result.Rows) == 0 {
		fmt.Println("No results")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Print headers
	for i, h := range result.Headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, strings.ToUpper(h))
	}
	fmt.Fprintln(w)

	// Print separator
	for i := range result.Headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, "---")
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range result.Rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, formatCell(cell))
		}
		fmt.Fprintln(w)
	}

	w.Flush()

	fmt.Printf("\n(%d rows)\n", len(result.Rows))
}

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
