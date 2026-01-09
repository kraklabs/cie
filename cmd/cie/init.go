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

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "Overwrite existing configuration")
	nonInteractive := fs.Bool("y", false, "Non-interactive mode (use defaults)")
	projectID := fs.String("project-id", "", "Project identifier")
	serverIP := fs.String("ip", "", "CIE server IP (sets edge-cache to http://IP:30080 and primary-hub to IP:30051)")
	edgeCache := fs.String("edge-cache", "", "Edge Cache URL (overrides --ip)")
	primaryHub := fs.String("primary-hub", "", "Primary Hub gRPC address (overrides --ip)")
	embeddingProvider := fs.String("embedding-provider", "", "Embedding provider (ollama, nomic, mock)")
	llmURL := fs.String("llm-url", "", "LLM API URL (OpenAI-compatible, e.g., http://localhost:8001/v1)")
	llmModel := fs.String("llm-model", "", "LLM model name")
	llmAPIKey := fs.String("llm-api-key", "", "LLM API key (optional for local models)")
	noHook := fs.Bool("no-hook", false, "Skip git hook installation (hook is installed by default)")
	withHook := fs.Bool("hook", false, "Install git hook without prompting (for scripts)")

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

	// If --ip is provided, construct URLs from it
	// Uses NodePort: Edge Cache on 30080, Primary Hub gRPC on 30051
	if *serverIP != "" {
		if *edgeCache == "" {
			*edgeCache = fmt.Sprintf("http://%s:30080", *serverIP)
		}
		if *primaryHub == "" {
			*primaryHub = fmt.Sprintf("%s:30051", *serverIP)
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get current directory: %v\n", err)
		os.Exit(1)
	}

	configPath := ConfigPath(cwd)

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "Error: %s already exists. Use --force to overwrite.\n", configPath)
		os.Exit(1)
	}

	// Determine project ID
	pid := *projectID
	if pid == "" {
		pid = filepath.Base(cwd)
	}

	// Create default config
	cfg := DefaultConfig(pid)

	// Apply flag overrides
	if *edgeCache != "" {
		cfg.CIE.EdgeCache = *edgeCache
	}
	if *primaryHub != "" {
		cfg.CIE.PrimaryHub = *primaryHub
	}
	if *embeddingProvider != "" {
		cfg.Embedding.Provider = *embeddingProvider
	}
	if *llmURL != "" {
		cfg.LLM.Enabled = true
		cfg.LLM.BaseURL = *llmURL
	}
	if *llmModel != "" {
		cfg.LLM.Model = *llmModel
	}
	if *llmAPIKey != "" {
		cfg.LLM.APIKey = *llmAPIKey
	}

	// Create reader for interactive prompts
	reader := bufio.NewReader(os.Stdin)

	// Interactive mode
	if !*nonInteractive {
		fmt.Println("CIE Project Configuration")
		fmt.Println("=========================")
		fmt.Println()

		// Project ID
		cfg.ProjectID = prompt(reader, "Project ID", cfg.ProjectID)

		// CIE Edge Cache URL
		cfg.CIE.EdgeCache = prompt(reader, "CIE Edge Cache URL", cfg.CIE.EdgeCache)

		// CIE Primary Hub address
		cfg.CIE.PrimaryHub = prompt(reader, "CIE Primary Hub (gRPC)", cfg.CIE.PrimaryHub)

		// Embedding provider
		fmt.Println()
		fmt.Println("Embedding Providers: ollama, nomic, mock")
		cfg.Embedding.Provider = prompt(reader, "Embedding provider", cfg.Embedding.Provider)

		if cfg.Embedding.Provider == "ollama" {
			cfg.Embedding.BaseURL = prompt(reader, "Ollama URL", cfg.Embedding.BaseURL)
			cfg.Embedding.Model = prompt(reader, "Embedding model", cfg.Embedding.Model)
		}

		// LLM Configuration (optional)
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
				fmt.Sscanf(maxTokensStr, "%d", &maxTokens)
				if maxTokens > 0 {
					cfg.LLM.MaxTokens = maxTokens
				}
			}
		}

		fmt.Println()
	}

	// Create .cie directory
	cieDir := ConfigDir(cwd)
	if err := os.MkdirAll(cieDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create .cie directory: %v\n", err)
		os.Exit(1)
	}

	// Save config
	if err := SaveConfig(cfg, configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot save configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created %s\n", configPath)

	// Add .cie/ to .gitignore if it exists
	addToGitignore(cwd)

	// Git hook installation (default: install unless --no-hook)
	shouldInstallHook := !*noHook
	if !*noHook && !*nonInteractive && !*withHook {
		// Interactive prompt for hook installation (default: Y)
		fmt.Println()
		hookAnswer := prompt(reader, "Install git hook for auto-indexing? (Y/n)", "y")
		hookAnswer = strings.ToLower(strings.TrimSpace(hookAnswer))
		shouldInstallHook = hookAnswer != "n" && hookAnswer != "no"
	}

	if shouldInstallHook {
		gitDir, err := findGitDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot find .git directory: %v\n", err)
		} else {
			hookPath := filepath.Join(gitDir, "hooks", "post-commit")
			if err := installHook(hookPath, false); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: cannot install git hook: %v\n", err)
			} else {
				fmt.Printf("Git hook installed: %s\n", hookPath)
			}
		}
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Review and edit .cie/project.yaml if needed")
	fmt.Println("  2. Run 'cie index' to index your repository")
	fmt.Println("  3. Run 'cie status' to verify indexing")
	if !shouldInstallHook {
		fmt.Println()
		fmt.Println("Tip: Run 'cie install-hook' to enable auto-indexing on each commit")
	}
}

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

func addToGitignore(dir string) {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Check if .gitignore exists
	content, err := os.ReadFile(gitignorePath)
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
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// Add newline if file doesn't end with one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		f.WriteString("\n")
	}

	f.WriteString("\n# CIE configuration\n.cie/\n")
	fmt.Println("Added .cie/ to .gitignore")
}
