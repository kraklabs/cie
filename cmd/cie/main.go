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
// Package main implements the CIE CLI for indexing repositories and querying
// the Code Intelligence Engine.
//
// Usage:
//
//	cie init                      Create .cie/project.yaml configuration
//	cie index                     Index the current repository
//	cie status [--json]           Show project status
//	cie query <script> [--json]   Execute CozoScript query
//	cie --mcp                     Start as MCP server (JSON-RPC over stdio)
package main

import (
	"flag"
	"fmt"
	"os"
)

// Version information (set via ldflags during build)
var (
	version = "dev"     // Version string
	commit  = "unknown" // Git commit hash
	date    = "unknown" // Build date
)

// main is the entry point for the CIE CLI.
//
// It parses global flags, dispatches to command handlers, or starts the MCP server.
//
// Global flags:
//   - --version: Display version information and exit
//   - --mcp: Start as MCP server (JSON-RPC over stdio)
//   - --config: Path to .cie/project.yaml configuration file
//
// Commands:
//   - init: Create .cie/project.yaml configuration
//   - index: Index the current repository
//   - status: Show project status
//   - query: Execute CozoScript query
//   - reset: Reset local project data (destructive!)
//   - install-hook: Install git post-commit hook for auto-indexing
func main() {
	// Global flags
	var (
		showVersion = flag.Bool("version", false, "Show version and exit")
		mcpMode     = flag.Bool("mcp", false, "Start as MCP server (JSON-RPC over stdio)")
		configPath  = flag.String("config", "", "Path to .cie/project.yaml (default: ./.cie/project.yaml)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `CIE - Code Intelligence Engine

CIE provides AI-powered code understanding through semantic search,
call graph analysis, and intelligent querying. It integrates with
Claude Code and other MCP-compatible tools to give AI assistants
deep understanding of your codebase.

Usage:
  cie <command> [options]

Commands:
  init          Create .cie/project.yaml configuration
  index         Index the current repository
  status        Show project status
  query         Execute CozoScript query
  reset         Reset local project data (destructive!)
  install-hook  Install git post-commit hook for auto-indexing
  completion    Generate shell completion script (bash|zsh|fish)

Global Options:
  --mcp         Start as MCP server (JSON-RPC over stdio)
  --config      Path to .cie/project.yaml
  --version     Show version and exit

Examples:
  cie init                           Create configuration interactively
  cie index                          Index current repository
  cie index --full                   Force full re-index
  cie status                         Show project status
  cie status --json                  Output as JSON (for MCP)
  cie query "?[name] := *cie_function{name}"
  cie completion bash                Generate bash completion script
  cie --mcp                          Start as MCP server

Getting Started:
  1. Initialize configuration:  cie init
  2. Index your repository:     cie index
  3. Check indexing status:     cie status
  4. Run as MCP server:         cie --mcp

Data Storage:
  Data is stored locally in ~/.cie/data/<project_id>/

Environment Variables:
  OLLAMA_HOST        Ollama URL (default: http://localhost:11434)
  OLLAMA_EMBED_MODEL Embedding model (default: nomic-embed-text)

For detailed command help: cie <command> --help

`)
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("cie version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built: %s\n", date)
		os.Exit(0)
	}

	// MCP mode takes precedence
	if *mcpMode {
		runMCPServer(*configPath)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "init":
		runInit(cmdArgs)
	case "index":
		runIndex(cmdArgs, *configPath)
	case "status":
		runStatus(cmdArgs, *configPath)
	case "query":
		runQuery(cmdArgs, *configPath)
	case "reset":
		runReset(cmdArgs, *configPath)
	case "install-hook":
		runInstallHook(cmdArgs, *configPath)
	case "completion":
		runCompletion(cmdArgs, *configPath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}
