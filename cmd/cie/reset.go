// Copyright 2025 KrakLabs
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
)

// runReset executes the 'reset' CLI command, deleting all local indexed data.
func runReset(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("reset", flag.ExitOnError)
	confirm := fs.Bool("yes", false, "Confirm the reset (required)")
	docker := fs.Bool("docker", false, "Also reset Docker containers and volumes")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie reset [options]

Description:
  WARNING: This is a destructive operation that deletes all locally
  indexed data for the current project.

  Removes the ~/.cie/data/<project_id>/ directory, including:
  - All indexed code intelligence data
  - Embeddings and call graphs
  - Indexing checkpoints

  With --docker, also stops containers and removes Docker volumes.

  Use this if the database is corrupted or you want to start fresh.
  You'll need to re-run 'cie index' after resetting.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Reset local data only
  cie reset --yes

  # Reset everything including Docker volumes
  cie reset --yes --docker

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

	composeDir, err := getCIEDir()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to find CIE directory",
			err.Error(),
			"",
			err,
		), globals.JSON)
	}

	// Reset Docker if requested
	if *docker {
		composePath := filepath.Join(composeDir, "docker-compose.yml")
		if _, err := os.Stat(composePath); err == nil {
			ui.Header("Resetting Docker Infrastructure")

			if err := checkDocker(); err != nil {
				errors.FatalError(err, globals.JSON)
			}

			ui.Info("Stopping containers and removing volumes...")
			if err := runComposeCommand(composeDir, "down", "-v"); err != nil {
				ui.Warningf("Failed to reset Docker infrastructure: %v", err)
			} else {
				ui.Success("Docker infrastructure reset")
			}
		}
	}

	// Load configuration to get project ID
	cfg, err := LoadConfig(configPath)
	if err != nil {
		// If no config, just clean up the data directory
		dataDir := filepath.Join(composeDir, "data")
		if err := os.RemoveAll(dataDir); err != nil {
			ui.Warningf("Failed to remove data directory: %v", err)
		}
		ui.Success("CIE data reset complete")
		return
	}

	// Determine data directory
	dataDir := filepath.Join(composeDir, "data", cfg.ProjectID)

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
