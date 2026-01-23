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

// runStop executes the 'stop' CLI command, which stops the Docker infrastructure.
func runStop(args []string, globals GlobalFlags) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie stop [options]

Description:
  Stop the CIE infrastructure. This stops the Ollama and CIE Server containers
  but preserves all data (indexes, embeddings, etc.).

  To also remove all data, use 'cie reset --docker' instead.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  cie stop
`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	ui.Header("Stopping CIE Infrastructure")

	// Check if docker is running
	if err := checkDocker(); err != nil {
		errors.FatalError(err, globals.JSON)
	}

	// Run docker compose down
	ui.Info("Stopping containers...")
	if err := runCommand("docker", "compose", "down"); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to stop containers",
			"Docker Compose down failed",
			"Check Docker logs for details",
			err,
		), globals.JSON)
	}

	ui.Success("CIE infrastructure stopped")
}