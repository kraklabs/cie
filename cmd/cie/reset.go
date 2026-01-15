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
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
)

// runReset executes the 'reset' CLI command, deleting all local indexed data.
//
// This is a destructive operation that removes the entire ~/.cie/data/<project_id>/
// directory, clearing all locally stored index data, embeddings, and cached state.
//
// The user must explicitly confirm with the --yes flag to prevent accidental data loss.
//
// Flags:
//   - --yes: Required confirmation flag (no default)
//
// Examples:
//
//	cie reset --yes    Delete all local data (destructive!)
func runReset(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	confirm := fs.Bool("yes", false, "Confirm the reset (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie reset [options]

Description:
  WARNING: This is a destructive operation that deletes all locally
  indexed data for the current project.

  Removes the ~/.cie/data/<project_id>/ directory, including:
  - All indexed code intelligence data
  - Embeddings and call graphs
  - Indexing checkpoints

  Use this if the database is corrupted or you want to start fresh.
  You'll need to re-run 'cie index' after resetting.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Reset requires explicit --yes confirmation
  cie reset --yes

  # This will NOT work (missing confirmation)
  cie reset
  # Error: --yes flag is required to confirm data deletion

Notes:
  This only affects local data. Configuration (.cie/project.yaml) is not deleted.
  To also reset configuration, delete .cie/project.yaml manually or use 'cie init --force'.

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if !*confirm {
		errors.FatalError(errors.NewInputError(
			"Confirmation required",
			"The --yes flag is required to confirm this destructive operation",
			"Run 'cie reset --yes' to confirm that you want to delete all indexed data",
		), false)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		errors.FatalError(err, false) // LoadConfig returns UserError
	}

	// Determine data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Cannot determine home directory",
			"Operating system failed to provide user home directory path",
			"Check your system configuration or set the HOME environment variable",
			err,
		), false)
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
		errors.FatalError(errors.NewPermissionError(
			"Cannot delete data directory",
			fmt.Sprintf("Failed to remove %s - permission denied or file locked", dataDir),
			"Check directory permissions, ensure no other CIE processes are running, and try again",
			err,
		), false)
	}

	fmt.Println("Reset complete. All local indexed data has been deleted.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cie index --full    Reindex the project")
}
