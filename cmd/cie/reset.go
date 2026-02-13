// Copyright 2025 KrakLabs
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
)

// runReset executes the 'reset' CLI command, deleting all local indexed data.
func runReset(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	confirm := fs.Bool("yes", false, "Confirm the reset (required)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie reset [options]

Description:
  WARNING: This is a destructive operation that deletes all locally
  indexed data for the current project.

  Removes the configured data directory for the project
  (default: ~/.cie/data/<project_id>/), including:
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
  # Reset local data
  cie reset --yes

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

	// Load configuration to get project ID
	cfg, err := LoadConfig(configPath)
	if err != nil {
		// If no config, just clean up the data root directory
		dataDir, rootErr := dataRootFromConfig(nil, configPath)
		if rootErr != nil {
			errors.FatalError(rootErr, globals.JSON)
		}
		if err := os.RemoveAll(dataDir); err != nil {
			ui.Warningf("Failed to remove data directory: %v", err)
		}
		ui.Success("CIE data reset complete")
		return
	}

	// Determine data directory
	dataDir, err := projectDataDir(cfg, configPath)
	if err != nil {
		errors.FatalError(err, globals.JSON)
	}

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No local data found for project %s\n", cfg.ProjectID)
		return
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

	ui.Success("Reset complete. All local indexed data has been deleted.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cie index    Reindex the project")
}
