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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/ui"
)

// runInit executes the 'init' CLI command, creating a .cie/project.yaml configuration file.
//
// It creates the configuration directory, generates a default configuration, and optionally
// prompts the user for customization in interactive mode. The command can also install
// a git post-commit hook for automatic re-indexing.
//
// Flags:
//   - --force: Overwrite existing configuration (default: false)
//   - -y: Non-interactive mode, use all defaults (default: false)
//   - --project-id: Project identifier (default: directory name)
//   - --ip: CIE server IP for Tailscale/NodePort setup (sets edge-cache and primary-hub)
//   - --edge-cache: Edge Cache URL (overrides --ip)
//   - --primary-hub: Primary Hub gRPC address (overrides --ip)
//   - --embedding-provider: Embedding provider (ollama, nomic, mock)
//   - --no-hook: Skip git hook installation
//   - --hook: Install git hook without prompting
//
// Examples:
//
//	cie init                           Interactive setup
//	cie init -y                        Use all defaults
//	cie init --ip 100.117.59.45        Configure with Tailscale IP
//	cie init --hook                    Initialize and install git hook
//
// initFlags holds parsed flags for the init command.
type initFlags struct {
	force, nonInteractive, noHook, withHook bool
	projectID, serverIP, edgeCache          string
	primaryHub, embeddingProvider           string
}

func runInit(args []string, globals GlobalFlags) {
	// Check if we should delegate to remote server
	baseURL := os.Getenv("CIE_BASE_URL")

	if baseURL != "" {
		runRemoteInit(baseURL, args)
		return
	}

	flags := parseInitFlags(args)
	applyServerIPDefaults(&flags)

	cwd, err := os.Getwd()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Cannot access working directory",
			"Failed to determine current directory path",
			"This is unexpected. Please report this issue if it persists",
			err,
		), false)
	}

	configPath := ConfigPath(cwd)
	if _, err := os.Stat(configPath); err == nil && !flags.force {
		errors.FatalError(errors.NewInputError(
			"Configuration already exists",
			fmt.Sprintf("%s already exists in this directory", configPath),
			"Use 'cie init --force' to overwrite the existing configuration",
		), false)
	}

	cfg := createInitConfig(cwd, flags)
	reader := bufio.NewReader(os.Stdin)

	if !flags.nonInteractive {
		runInteractiveConfig(reader, cfg)
	}

	saveInitConfig(cwd, configPath, cfg)
	handleHookInstallation(reader, flags)
	printNextSteps(flags.noHook)
}

func parseInitFlags(args []string) initFlags {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	var f initFlags
	fs.BoolVar(&f.force, "force", false, "Overwrite existing configuration")
	fs.BoolVarP(&f.nonInteractive, "yes", "y", false, "Non-interactive mode (use defaults)")
	fs.StringVar(&f.projectID, "project-id", "", "Project identifier")
	fs.StringVar(&f.serverIP, "ip", "", "CIE server IP (sets edge-cache to http://IP:30080 and primary-hub to IP:30051)")
	fs.StringVar(&f.edgeCache, "edge-cache", "", "Edge Cache URL (overrides --ip)")
	fs.StringVar(&f.primaryHub, "primary-hub", "", "Primary Hub gRPC address (overrides --ip)")
	fs.StringVar(&f.embeddingProvider, "embedding-provider", "", "Embedding provider (ollama, nomic, mock)")
	fs.BoolVar(&f.noHook, "no-hook", false, "Skip git hook installation (hook is installed by default)")
	fs.BoolVar(&f.withHook, "hook", false, "Install git hook without prompting (for scripts)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie init [options]

Description:
  Create a .cie/project.yaml configuration file for the current repository.

  By default, runs in interactive mode with prompts for each setting.
  Use -y for non-interactive mode with sensible defaults.

  The configuration defines:
  - Project identifier and data storage location
  - Embedding provider (ollama, nomic, openai, mock)
  - Indexing behavior (exclusions, batch size, etc.)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Interactive setup with prompts
  cie init

  # Non-interactive with all defaults
  cie init -y

  # Configure for Tailscale/NodePort setup (sets edge-cache and primary-hub)
  cie init --ip 100.117.59.45

  # Configure with custom Edge Cache URL
  cie init --edge-cache http://myserver:8080

  # Initialize and install git hook for auto-indexing
  cie init --hook

  # Custom project ID (default: directory name)
  cie init --project-id my-awesome-project

Notes:
  Configuration is stored in .cie/project.yaml in the repository root.
  You can edit this file manually or re-run init with --force to recreate.

`)
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	return f
}

func applyServerIPDefaults(f *initFlags) {
	if f.serverIP != "" {
		if f.edgeCache == "" {
			f.edgeCache = fmt.Sprintf("http://%s:30080", f.serverIP)
		}
		if f.primaryHub == "" {
			f.primaryHub = fmt.Sprintf("%s:30051", f.serverIP)
		}
	}
}

func createInitConfig(cwd string, f initFlags) *Config {
	pid := f.projectID
	if pid == "" {
		pid = filepath.Base(cwd)
	}
	cfg := DefaultConfig(pid)
	if f.edgeCache != "" {
		cfg.CIE.EdgeCache = f.edgeCache
	}
	if f.primaryHub != "" {
		cfg.CIE.PrimaryHub = f.primaryHub
	}
	if f.embeddingProvider != "" {
		cfg.Embedding.Provider = f.embeddingProvider
	}
	return cfg
}

func runInteractiveConfig(reader *bufio.Reader, cfg *Config) {
	ui.Header("CIE Project Configuration")
	fmt.Println()

	cfg.ProjectID = prompt(reader, "Project ID", cfg.ProjectID)

	fmt.Println()
	ui.Info("Embedding Providers: ollama, nomic, mock")
	cfg.Embedding.Provider = prompt(reader, "Embedding provider", cfg.Embedding.Provider)
	if cfg.Embedding.Provider == "ollama" {
		cfg.Embedding.BaseURL = prompt(reader, "Ollama URL", cfg.Embedding.BaseURL)
		cfg.Embedding.Model = prompt(reader, "Embedding model", cfg.Embedding.Model)
	}

	fmt.Println()
}

func saveInitConfig(cwd, configPath string, cfg *Config) {
	cieDir := ConfigDir(cwd)
	if err := os.MkdirAll(cieDir, 0750); err != nil {
		errors.FatalError(errors.NewPermissionError(
			"Cannot create .cie directory",
			fmt.Sprintf("Permission denied creating directory: %s", cieDir),
			"Check directory permissions or run with appropriate privileges",
			err,
		), false)
	}
	if err := SaveConfig(cfg, configPath); err != nil {
		errors.FatalError(errors.NewPermissionError(
			"Cannot save configuration file",
			fmt.Sprintf("Failed to write %s", configPath),
			"Check directory permissions and available disk space",
			err,
		), false)
	}
	ui.Successf("Created %s", configPath)
	addToGitignore(cwd)
}

func handleHookInstallation(reader *bufio.Reader, f initFlags) {
	if f.noHook {
		return
	}
	shouldInstall := f.withHook
	if !f.withHook && !f.nonInteractive {
		fmt.Println()
		hookAnswer := prompt(reader, "Install git hook for auto-indexing? (Y/n)", "y")
		hookAnswer = strings.ToLower(strings.TrimSpace(hookAnswer))
		shouldInstall = hookAnswer != "n" && hookAnswer != "no"
	} else if f.nonInteractive {
		shouldInstall = true
	}

	if !shouldInstall {
		return
	}
	gitDir, err := findGitDir()
	if err != nil {
		ui.Warningf("cannot find .git directory: %v", err)
		return
	}
	hookPath := filepath.Join(gitDir, "hooks", "post-commit")
	if err := installHook(hookPath, false); err != nil {
		ui.Warningf("cannot install git hook: %v", err)
	} else {
		ui.Successf("Git hook installed: %s", hookPath)
	}
}

func printNextSteps(noHook bool) {
	fmt.Println()
	ui.SubHeader("Next steps:")
	fmt.Printf("  1. Review and edit %s if needed\n", ui.DimText(".cie/project.yaml"))
	fmt.Printf("  2. Run '%s' to index your repository\n", ui.Cyan.Sprint("cie index"))
	fmt.Printf("  3. Run '%s' to verify indexing\n", ui.Cyan.Sprint("cie status"))
	if noHook {
		fmt.Println()
		ui.Infof("Tip: Run '%s' to enable auto-indexing on each commit", ui.Cyan.Sprint("cie install-hook"))
	}
}

// prompt displays an interactive prompt and reads user input from stdin.
//
// If the user presses Enter without providing input, the defaultValue is returned.
// This is used during interactive configuration setup.
//
// Parameters:
//   - reader: bufio.Reader for reading from stdin
//   - label: Prompt label to display to the user
//   - defaultValue: Value to return if user presses Enter (shown in brackets)
//
// Returns the user's input or the default value if input is empty.
func prompt(reader *bufio.Reader, label, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultValue
	}
	return input
}

// addToGitignore adds .cie/ to the project's .gitignore file if not already present.
//
// It safely appends the entry to .gitignore, avoiding duplicates. If .gitignore does
// not exist or cannot be modified, the function silently returns without error.
//
// The function checks for various .cie/ patterns (.cie, .cie/, /.cie, /.cie/) to
// avoid adding duplicate entries.
//
// Parameters:
//   - dir: Directory containing the .gitignore file
func addToGitignore(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Check if .gitignore exists
	content, err := os.ReadFile(gitignorePath) //nolint:gosec // G304: gitignorePath built from repo dir
	if err != nil {
		if os.IsNotExist(err) {
			// No .gitignore, nothing to do
			return
		}
		return
	}

	// Check if .cie/ is already in .gitignore
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == ".cie/" || line == ".cie" || line == "/.cie/" || line == "/.cie" {
			return // Already present
		}
	}

	// Append .cie/ to .gitignore
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0600) //nolint:gosec // G304: gitignorePath built from repo dir
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	// Add newline if file doesn't end with one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		_, _ = f.WriteString("\n")
	}

	_, _ = f.WriteString("\n# CIE configuration\n.cie/\n")
	fmt.Println("Added .cie/ to .gitignore")
}

// runRemoteInit delegates initialization to the remote CIE server.
func runRemoteInit(baseURL string, args []string) {
	flags := parseInitFlags(args)

	// Build request payload
	payload := map[string]string{
		"project_id":         flags.projectID,
		"embedding_provider": flags.embeddingProvider,
	}
	if payload["embedding_provider"] == "" {
		payload["embedding_provider"] = "ollama"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Failed to encode request",
			"Could not serialize init request to JSON",
			"This is a bug. Please report it.",
			err,
		), false)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(baseURL+"/v1/init", "application/json", bytes.NewReader(body))
	if err != nil {
		errors.FatalError(errors.NewNetworkError(
			"Cannot connect to CIE server",
			fmt.Sprintf("Failed to reach %s/v1/init", baseURL),
			"Check that the CIE server is running and CIE_BASE_URL is correct",
			err,
		), false)
	}
	defer resp.Body.Close()

	var result struct {
		OK         bool   `json:"ok"`
		ProjectID  string `json:"project_id"`
		ConfigPath string `json:"config_path"`
		Error      string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		errors.FatalError(errors.NewInternalError(
			"Invalid response from server",
			"Could not parse server response",
			"Check server logs for errors",
			err,
		), false)
	}

	if resp.StatusCode != http.StatusOK || !result.OK {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = fmt.Sprintf("server returned status %d", resp.StatusCode)
		}
		errors.FatalError(errors.NewInternalError(
			"Server initialization failed",
			errMsg,
			"Check server logs for details",
			nil,
		), false)
	}

	// Create local .cie/project.yaml with project_id for CLI commands
	cwd, err := os.Getwd()
	if err == nil {
		localCfg := DefaultConfig(result.ProjectID)
		localCfg.CIE.EdgeCache = baseURL // Store server URL
		cieDir := ConfigDir(cwd)
		if err := os.MkdirAll(cieDir, 0750); err == nil {
			configPath := ConfigPath(cwd)
			_ = SaveConfig(localCfg, configPath)
		}
	}

	ui.Successf("Project initialized on server: %s", result.ProjectID)
	ui.Infof("Using CIE server at %s", baseURL)
	fmt.Printf("Config saved at: %s\n", result.ConfigPath)
	fmt.Println()
	ui.SubHeader("Next steps:")
	fmt.Printf("  1. Run '%s' to index the repository\n", ui.Cyan.Sprint("cie index"))
	fmt.Printf("  2. Run '%s' to verify indexing\n", ui.Cyan.Sprint("cie status"))
}