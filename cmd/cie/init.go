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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
//   - --llm-url: LLM API URL for narrative generation
//   - --llm-model: LLM model name
//   - --llm-api-key: LLM API key (optional for local models)
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
	llmURL, llmModel, llmAPIKey             string
}

func runInit(args []string) {
	flags := parseInitFlags(args)
	applyServerIPDefaults(&flags)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get current directory: %v\n", err)
		os.Exit(1)
	}

	configPath := ConfigPath(cwd)
	if _, err := os.Stat(configPath); err == nil && !flags.force {
		fmt.Fprintf(os.Stderr, "Error: %s already exists. Use --force to overwrite.\n", configPath)
		os.Exit(1)
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
	fs.BoolVar(&f.nonInteractive, "y", false, "Non-interactive mode (use defaults)")
	fs.StringVar(&f.projectID, "project-id", "", "Project identifier")
	fs.StringVar(&f.serverIP, "ip", "", "CIE server IP (sets edge-cache to http://IP:30080 and primary-hub to IP:30051)")
	fs.StringVar(&f.edgeCache, "edge-cache", "", "Edge Cache URL (overrides --ip)")
	fs.StringVar(&f.primaryHub, "primary-hub", "", "Primary Hub gRPC address (overrides --ip)")
	fs.StringVar(&f.embeddingProvider, "embedding-provider", "", "Embedding provider (ollama, nomic, mock)")
	fs.StringVar(&f.llmURL, "llm-url", "", "LLM API URL (OpenAI-compatible, e.g., http://localhost:8001/v1)")
	fs.StringVar(&f.llmModel, "llm-model", "", "LLM model name")
	fs.StringVar(&f.llmAPIKey, "llm-api-key", "", "LLM API key (optional for local models)")
	fs.BoolVar(&f.noHook, "no-hook", false, "Skip git hook installation (hook is installed by default)")
	fs.BoolVar(&f.withHook, "hook", false, "Install git hook without prompting (for scripts)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie init [options]

Creates .cie/project.yaml configuration file.

Examples:
  cie init --ip 100.117.59.45           # Configure with Tailscale IP
  cie init --ip 100.117.59.45 -y        # Non-interactive with defaults
  cie init --edge-cache http://myserver:8080
  cie init --hook                       # Also install git hook

Options:
`)
		fs.PrintDefaults()
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
	if f.llmURL != "" {
		cfg.LLM.Enabled = true
		cfg.LLM.BaseURL = f.llmURL
	}
	if f.llmModel != "" {
		cfg.LLM.Model = f.llmModel
	}
	if f.llmAPIKey != "" {
		cfg.LLM.APIKey = f.llmAPIKey
	}
	return cfg
}

func runInteractiveConfig(reader *bufio.Reader, cfg *Config) {
	fmt.Println("CIE Project Configuration")
	fmt.Println("=========================")
	fmt.Println()

	cfg.ProjectID = prompt(reader, "Project ID", cfg.ProjectID)

	fmt.Println()
	fmt.Println("Embedding Providers: ollama, nomic, mock")
	cfg.Embedding.Provider = prompt(reader, "Embedding provider", cfg.Embedding.Provider)
	if cfg.Embedding.Provider == "ollama" {
		cfg.Embedding.BaseURL = prompt(reader, "Ollama URL", cfg.Embedding.BaseURL)
		cfg.Embedding.Model = prompt(reader, "Embedding model", cfg.Embedding.Model)
	}

	promptLLMConfig(reader, cfg)
	fmt.Println()
}

func promptLLMConfig(reader *bufio.Reader, cfg *Config) {
	fmt.Println()
	fmt.Println("LLM Configuration (for analyze narratives)")
	fmt.Println("Configure an OpenAI-compatible LLM to generate narrative explanations.")
	fmt.Println("Leave empty to skip LLM configuration.")
	fmt.Println()

	llmURLInput := prompt(reader, "LLM API URL (e.g., http://localhost:8001/v1)", cfg.LLM.BaseURL)
	if llmURLInput != "" {
		cfg.LLM.Enabled = true
		cfg.LLM.BaseURL = llmURLInput
		cfg.LLM.Model = prompt(reader, "LLM model name", "qwen3-coder")
		cfg.LLM.APIKey = prompt(reader, "LLM API key (optional)", cfg.LLM.APIKey)
		maxTokensStr := prompt(reader, "Max tokens for narrative", "2000")
		if maxTokensStr != "" {
			var maxTokens int
			_, _ = fmt.Sscanf(maxTokensStr, "%d", &maxTokens)
			if maxTokens > 0 {
				cfg.LLM.MaxTokens = maxTokens
			}
		}
	}
}

func saveInitConfig(cwd, configPath string, cfg *Config) {
	cieDir := ConfigDir(cwd)
	if err := os.MkdirAll(cieDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create .cie directory: %v\n", err)
		os.Exit(1)
	}
	if err := SaveConfig(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot save configuration: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s\n", configPath)
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
		fmt.Fprintf(os.Stderr, "Warning: cannot find .git directory: %v\n", err)
		return
	}
	hookPath := filepath.Join(gitDir, "hooks", "post-commit")
	if err := installHook(hookPath, false); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot install git hook: %v\n", err)
	} else {
		fmt.Printf("Git hook installed: %s\n", hookPath)
	}
}

func printNextSteps(noHook bool) {
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and edit .cie/project.yaml if needed")
	fmt.Println("  2. Run 'cie index' to index your repository")
	fmt.Println("  3. Run 'cie status' to verify indexing")
	if noHook {
		fmt.Println()
		fmt.Println("Tip: Run 'cie install-hook' to enable auto-indexing on each commit")
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
