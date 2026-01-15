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
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/ui"
)

// Version information (set via ldflags during build)
var (
	version = "dev"     // Version string
	commit  = "unknown" // Git commit hash
	date    = "unknown" // Build date
)

// GlobalFlags holds the global CLI flags that apply to all commands.
type GlobalFlags struct {
	JSON     bool // Output in JSON format (for applicable commands)
	NoColor  bool // Disable color output
	Verbose  int  // Verbosity level: 0=normal, 1=-v (info), 2=-vv (debug)
	Quiet    bool // Suppress non-essential output (progress, info messages)
}

// logInfo outputs an informational message to stderr if verbose mode is enabled.
// Messages are suppressed if quiet mode is active.
func logInfo(globals GlobalFlags, format string, args ...interface{}) {
	if !globals.Quiet && globals.Verbose >= 1 {
		fmt.Fprintf(os.Stderr, "[INFO] "+format+"\n", args...)
	}
}

// logDebug outputs a debug message to stderr if debug verbosity is enabled (-vv).
// Debug messages are shown regardless of quiet mode for troubleshooting.
func logDebug(globals GlobalFlags, format string, args ...interface{}) {
	if globals.Verbose >= 2 {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

// logError outputs an error message to stderr unless quiet mode is active.
// Note: Fatal errors should still use errors.FatalError() which handles quiet mode.
func logError(globals GlobalFlags, format string, args ...interface{}) {
	if !globals.Quiet {
		fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
	}
}

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
	// Global flags with short forms
	var (
		showVersion = flag.BoolP("version", "V", false, "Show version and exit")
		mcpMode     = flag.Bool("mcp", false, "Start as MCP server (JSON-RPC over stdio)")
		configPath  = flag.StringP("config", "c", "", "Path to .cie/project.yaml (default: ./.cie/project.yaml)")
		jsonOutput  = flag.Bool("json", false, "Output in JSON format (for applicable commands)")
		noColor     = flag.Bool("no-color", false, "Disable color output")
		verbose     = flag.CountP("verbose", "v", "Increase verbosity (-v for info, -vv for debug)")
		quiet       = flag.BoolP("quiet", "q", false, "Suppress non-essential output (progress, info messages)")
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
  --json            Output in JSON format (for applicable commands)
  --no-color        Disable color output (respects NO_COLOR env var)
  -v, --verbose     Increase verbosity (-v for info, -vv for debug)
  -q, --quiet       Suppress non-essential output (progress, info messages)
  --mcp             Start as MCP server (JSON-RPC over stdio)
  -c, --config      Path to .cie/project.yaml
  -V, --version     Show version and exit

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

	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		*noColor = true
	}

	// Validate conflicting flags
	if *quiet && *verbose > 0 {
		fmt.Fprintf(os.Stderr, "Error: cannot use --quiet and --verbose together\n")
		os.Exit(1)
	}

	// JSON mode auto-enables quiet to prevent progress bars corrupting JSON output
	if *jsonOutput {
		*quiet = true
	}

	// Build GlobalFlags struct
	globals := GlobalFlags{
		JSON:    *jsonOutput,
		NoColor: *noColor,
		Verbose: *verbose,
		Quiet:   *quiet,
	}

	// Initialize color output based on flags
	ui.InitColors(globals.NoColor)

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
		runInit(cmdArgs, globals)
	case "index":
		runIndex(cmdArgs, *configPath, globals)
	case "status":
		runStatus(cmdArgs, *configPath, globals)
	case "query":
		runQuery(cmdArgs, *configPath, globals)
	case "reset":
		runReset(cmdArgs, *configPath, globals)
	case "install-hook":
		runInstallHook(cmdArgs, *configPath, globals)
	case "completion":
		runCompletion(cmdArgs, *configPath, globals)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}
