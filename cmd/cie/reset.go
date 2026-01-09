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
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func runReset(args []string, configPath string) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	confirm := fs.Bool("yes", false, "Confirm the reset (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie reset [options]

Resets the local project data, clearing all indexed data.
This is useful before a full re-index to ensure a clean slate.

WARNING: This operation is destructive and cannot be undone!

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if !*confirm {
		fmt.Fprintf(os.Stderr, "Error: you must pass --yes to confirm the reset\n")
		fmt.Fprintf(os.Stderr, "This will delete all indexed data for the project.\n")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get home directory: %v\n", err)
		os.Exit(1)
	}
	dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No local data found for project %s\n", cfg.ProjectID)
		os.Exit(0)
	}

	fmt.Printf("Resetting project %s (deleting %s)...\n", cfg.ProjectID, dataDir)

	// Delete the data directory
	if err := os.RemoveAll(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to delete data: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Reset complete. All local indexed data has been deleted.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cie index --full    Reindex the project")
}
