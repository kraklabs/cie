// Copyright 2025 KrakLabs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

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

func main() {
	// Global flags
	var (
		showVersion = flag.Bool("version", false, "Show version and exit")
		mcpMode     = flag.Bool("mcp", false, "Start as MCP server (JSON-RPC over stdio)")
		configPath  = flag.String("config", "", "Path to .cie/project.yaml (default: ./.cie/project.yaml)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `CIE - Code Intelligence Engine CLI (Standalone)

Usage:
  cie <command> [options]

Commands:
  init          Create .cie/project.yaml configuration
  index         Index the current repository
  status        Show project status
  query         Execute CozoScript query
  reset         Reset local project data (destructive!)
  install-hook  Install git post-commit hook for auto-indexing

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
  cie --mcp                          Start as MCP server

Data Storage:
  Data is stored locally in ~/.cie/data/<project_id>/

Environment Variables:
  OLLAMA_HOST        Ollama URL (default: http://localhost:11434)
  OLLAMA_EMBED_MODEL Embedding model (default: nomic-embed-text)

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
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}
