// Copyright 2025 KrakLabs
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
)

//go:embed embed/docker-compose.yml
var embeddedDockerCompose []byte

// getCIEDir returns the path to ~/.cie directory, creating it if needed.
func getCIEDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cieDir := filepath.Join(home, ".cie")
	if err := os.MkdirAll(cieDir, 0750); err != nil {
		return "", err
	}
	return cieDir, nil
}

// ensureDockerCompose extracts the embedded docker-compose.yml to ~/.cie/
func ensureDockerCompose() (string, error) {
	cieDir, err := getCIEDir()
	if err != nil {
		return "", err
	}

	composePath := filepath.Join(cieDir, "docker-compose.yml")

	// Always write the embedded compose file to ensure it's up to date
	if err := os.WriteFile(composePath, embeddedDockerCompose, 0640); err != nil {
		return "", fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}

	return cieDir, nil
}

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

  The docker-compose.yml is embedded in the binary and extracted to ~/.cie/

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

	// 2. Extract embedded docker-compose.yml
	composeDir, err := ensureDockerCompose()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to extract docker-compose.yml",
			err.Error(),
			"Check that ~/.cie/ is writable",
			err,
		), globals.JSON)
	}

	// 3. Load config to get project info for Docker
	composeEnv := make(map[string]string)
	cfg, err := LoadConfig(configPath)
	if err == nil && cfg.ProjectID != "" {
		composeEnv["CIE_PROJECT_ID"] = cfg.ProjectID
		// Get the project directory (parent of .cie/)
		if configPath != "" {
			projectDir := filepath.Dir(filepath.Dir(configPath))
			composeEnv["CIE_PROJECT_DIR"] = projectDir
		} else {
			// Use current working directory
			if cwd, err := os.Getwd(); err == nil {
				composeEnv["CIE_PROJECT_DIR"] = cwd
			}
		}
		ui.Infof("Project: %s", cfg.ProjectID)
	}

	// 4. Run docker compose up -d from ~/.cie/
	ui.Info("Starting containers...")
	if err := runComposeCommandWithEnv(composeDir, composeEnv, "up", "-d"); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to start containers",
			"Docker Compose up failed",
			"Check Docker logs with: docker compose -f ~/.cie/docker-compose.yml logs",
			err,
		), globals.JSON)
	}

	// 4. Wait for Ollama and check for model
	ui.Info("Verifying embedding model...")
	if err := ensureModel(composeDir, *timeout); err != nil {
		errors.FatalError(err, globals.JSON)
	}
	ui.Success("Embedding model is ready")

	// 5. Final health check for CIE server
	ui.Info("Waiting for CIE server to be ready...")
	if err := waitForHealth("http://localhost:9090/health", *timeout); err != nil {
		errors.FatalError(errors.NewNetworkError(
			"CIE server health check failed",
			"The server did not become healthy within the timeout",
			"Check server logs with: docker compose -f ~/.cie/docker-compose.yml logs cie-server",
			err,
		), globals.JSON)
	}

	ui.Success("CIE infrastructure is up and running!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cie init     Initialize your project")
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

func runComposeCommand(dir string, args ...string) error {
	return runComposeCommandWithEnv(dir, nil, args...)
}

func runComposeCommandWithEnv(dir string, env map[string]string, args ...string) error {
	cmdArgs := append([]string{"compose", "-f", filepath.Join(dir, "docker-compose.yml")}, args...)
	cmd := exec.Command("docker", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Inherit current environment and add custom vars
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd.Run()
}

func ensureModel(composeDir string, timeout time.Duration) error {
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
		ui.Info("Model 'nomic-embed-text' not found. Downloading...")
		if err := runComposeCommand(composeDir, "--profile", "setup", "up"); err != nil {
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
