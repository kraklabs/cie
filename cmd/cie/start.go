// Copyright 2025 KrakLabs
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
)

// runStart executes the 'start' CLI command, which manages the Docker infrastructure.
// It checks if Docker is running, starts the services, performs setup if needed,
// and runs health checks.
func runStart(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	timeout := fs.Duration("timeout", 2*time.Minute, "Total timeout for start and health checks")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie start [options]

Description:
  Start the CIE infrastructure using Docker Compose. This command:
  1. Verifies that Docker is running.
  2. Starts the Ollama and CIE Server containers.
  3. Checks if the embedding model is installed, running setup if necessary.
  4. Waits for all services to be healthy.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  cie start
  cie start --timeout 5m
`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	ui.Header("Starting CIE Infrastructure")

	// 1. Check if docker is installed and daemon is running
	if err := checkDocker(); err != nil {
		errors.FatalError(err, globals.JSON)
	}
	ui.Success("Docker is running")

	// 2. Run docker compose up -d
	ui.Info("Starting containers...")
	if err := runCommand("docker", "compose", "up", "-d"); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to start containers",
			"Docker Compose up failed",
			"Check docker-compose.yml and your Docker logs",
			err,
		), globals.JSON)
	}

	// 3. Wait for Ollama and check for model
	ui.Info("Verifying embedding model...")
	if err := ensureModel(*timeout); err != nil {
		errors.FatalError(err, globals.JSON)
	}
	ui.Success("Embedding model is ready")

	// 4. Final health check for CIE server
	ui.Info("Waiting for CIE server to be ready...")
	if err := waitForHealth("http://localhost:9090/health", *timeout); err != nil {
		errors.FatalError(errors.NewNetworkError(
			"CIE server health check failed",
			"The server did not become healthy within the timeout",
			"Check server logs with: docker compose logs cie-server",
			err,
		), globals.JSON)
	}

	ui.Success("CIE infrastructure is up and running!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cie index    Index your repository")
	fmt.Println("  cie status   Check indexing status")
}

func checkDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return errors.NewInternalError(
			"Docker is not running",
			"Failed to execute 'docker info'",
			"Make sure Docker Desktop (or Engine) is installed and started",
			err,
		)
	}
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureModel(timeout time.Duration) error {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for Ollama")
		}

		// Check if Ollama is responsive
		resp, err := http.Get("http://localhost:11434/api/tags")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()

		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		modelFound := false
		for _, m := range tags.Models {
			if strings.HasPrefix(m.Name, "nomic-embed-text") {
				modelFound = true
				break
			}
		}

		if modelFound {
			return nil
		}

		// Model not found, run setup
		ui.Info("Model 'nomic-embed-text' not found. Running setup...")
		if err := runCommand("docker", "compose", "--profile", "setup", "up"); err != nil {
			return errors.NewInternalError(
				"Setup failed",
				"Docker Compose setup profile failed",
				"Check your internet connection and Docker logs",
				err,
			)
		}

		// Check again after setup
		continue
	}
}

func waitForHealth(url string, timeout time.Duration) error {
	start := time.Now()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for health check")
		}

		resp, err := client.Get(url)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			resp.Body.Close()
		}

		time.Sleep(2 * time.Second)
	}
}
