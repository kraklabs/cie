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

// Package main implements the CIE (Code Intelligence Engine) CLI.
//
// CIE is a code intelligence tool that indexes codebases and provides
// semantic understanding through the Model Context Protocol (MCP).
// It can be used standalone for local code queries or as an MCP server
// for AI assistants like Claude Code and Cursor.
//
// # Quick Start
//
// Initialize a new project in your repository:
//
//	cd /path/to/your/project
//	cie init
//
// Index your codebase:
//
//	cie index
//
// Check indexing status:
//
//	cie status
//
// Query the indexed code with CozoScript:
//
//	cie query '?[name, file_path] := *cie_function{ name, file_path } :limit 10'
//
// Start as an MCP server for AI assistants:
//
//	cie --mcp
//
// # Commands
//
// The CLI provides these main commands:
//
//	init           Initialize a new CIE project (creates .cie/project.yaml)
//	index          Index the current repository for code intelligence
//	status         Show project status (files, functions, types indexed)
//	query          Execute CozoScript queries on the indexed codebase
//	reset          Reset local project data (destructive operation)
//	install-hook   Install git post-commit hook for automatic re-indexing
//
// Global flags:
//
//	--version      Show version information and exit
//	--mcp          Start as MCP server (JSON-RPC over stdio)
//	--config PATH  Path to .cie/project.yaml configuration file
//
// # MCP Server Mode
//
// When run with the --mcp flag, CIE operates as a Model Context Protocol
// server, exposing 20+ tools for code intelligence to AI assistants.
//
// MCP tools include:
//
//	cie_semantic_search      Find code by meaning using embeddings
//	cie_grep                 Fast literal text search
//	cie_find_function        Find functions by name
//	cie_find_callers         Find what calls a function
//	cie_find_callees         Find what a function calls
//	cie_analyze              Answer architectural questions
//	cie_list_endpoints       List HTTP/REST endpoints
//	cie_trace_path           Trace call paths from entry points
//	cie_find_type            Find types, interfaces, structs
//	cie_find_implementations Find interface implementations
//	cie_directory_summary    Summarize directory structure
//	cie_verify_absence       Verify patterns don't exist (security audits)
//	... and more
//
// Configure CIE in your MCP client (e.g., Claude Code):
//
//	{
//	  "mcpServers": {
//	    "cie": {
//	      "command": "cie",
//	      "args": ["--mcp"],
//	      "env": {
//	        "CIE_PROJECT_ID": "my-project",
//	        "CIE_BASE_URL": "http://localhost:8080"
//	      }
//	    }
//	  }
//	}
//
// # Configuration
//
// CIE is configured through a local .cie/project.yaml file and environment
// variables. The init command creates a default configuration file.
//
// Environment variables (override config file):
//
//	CIE_PROJECT_ID         Project identifier (default: directory name)
//	CIE_BASE_URL           Edge Cache URL (default: http://localhost:8080)
//	CIE_PRIMARY_HUB        Primary Hub gRPC address (default: localhost:50051)
//	CIE_CONFIG_PATH        Explicit path to project.yaml
//
// Embedding provider settings:
//
//	OLLAMA_HOST            Ollama API URL (default: http://localhost:11434)
//	OLLAMA_EMBED_MODEL     Embedding model (default: nomic-embed-text)
//
// # Data Storage
//
// Indexed data is stored locally in:
//
//	~/.cie/data/<project_id>/
//
// This includes RocksDB databases for local storage and CozoDB for
// Datalog-based queries. Use the reset command to clear local data.
//
// # Git Integration
//
// The install-hook command adds a post-commit hook that automatically
// triggers background re-indexing after each commit, keeping the index
// up-to-date without manual intervention.
//
// See cie --help for complete usage information.
package main
