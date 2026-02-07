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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/pkg/storage"
	"github.com/kraklabs/cie/pkg/tools"
)

const (
	mcpVersion    = "1.8.0" // Standalone function interface dispatch, trace waypoints
	mcpServerName = "cie"
)

// cieInstructions is the MCP instructions text sent to agents on initialize.
// It guides AI agents on how to use CIE tools effectively for code intelligence.
const cieInstructions = `CIE (Code Intelligence Engine) gives you deep understanding of any indexed codebase. It indexes source code into a searchable graph with functions, types, call relationships, and semantic embeddings. Use CIE tools to navigate, search, and analyze code faster than reading files manually.

## CRITICAL: Always Use English for Queries

All CIE tool queries MUST be in English. The keyword boost algorithm matches query terms against English function/type names. Non-English terms will not activate the boost and will produce poor results.

## Quick Reference — Best Tool for Each Task

| Task | Best Tool | Example |
|------|-----------|---------|
| Find exact text like '.GET(', 'r.POST(' | cie_grep | text=".GET(" |
| List HTTP/REST endpoints | cie_list_endpoints | path_pattern="apps/gateway" |
| Trace call path to a function | cie_trace_path | target="RegisterRoutes" |
| Semantic/meaning-based search | cie_semantic_search | query="authentication logic" |
| Architectural questions | cie_analyze | question="What are the entry points?" |
| Find function by name | cie_find_function | name="BuildRouter" |
| What calls a function? | cie_find_callers | function_name="HandleAuth" |
| What does a function call? | cie_find_callees | function_name="HandleAuth" |
| Get function source code | cie_get_function_code | function_name="BuildRouter" |
| Find interface implementations | cie_find_implementations | interface_name="Repository" |
| Find type/interface/struct | cie_find_type | name="UserService" |
| Explore directory structure | cie_directory_summary | path="internal/cie" |
| Check index health | cie_index_status | (no args = check entire index) |
| Function git commit history | cie_function_history | function_name="HandleAuth" |
| Find when code was introduced | cie_find_introduction | code_snippet="jwt.Generate()" |
| Function code ownership/blame | cie_blame_function | function_name="Parse" |
| Verify patterns do NOT exist | cie_verify_absence | patterns=["api_key","secret"] |
| List gRPC services & RPCs | cie_list_services | path_pattern="api/proto" |
| Raw CozoScript query | cie_raw_query | (call cie_schema first) |

## Recommended Workflow

Follow this progression for most code exploration tasks:

1. **Orient** — Start with cie_directory_summary or cie_list_files to understand project structure.
2. **Search** — Use cie_grep for exact text, cie_semantic_search for concepts, or cie_find_function for known names.
3. **Navigate** — Follow the call graph with cie_find_callers, cie_find_callees, or cie_trace_path.
4. **Inspect** — Read specific function code with cie_get_function_code (use full_code=true for long functions).
5. **Analyze** — For architectural questions that span multiple functions, use cie_analyze.

## Tool Categories and When to Use Each

### Text Search Tools (exact matches)

**cie_grep** — Your go-to for finding exact text patterns. Ultra-fast, no regex. Use for:
- Code patterns: text=".GET(", text="func main", text="import"
- Multi-pattern batch search: texts=["access_token", "refresh_token", "secret"]
- Scoping: path="internal/cie", exclude_pattern="_test[.]go"

**cie_search_text** — Regex-capable search within indexed functions. Slower than cie_grep but supports regex. Use for:
- Complex patterns: pattern="(?i)handler.*error"
- Searching specific scopes: search_in="signature" (only function signatures)
- Use literal=true for exact code patterns (avoids regex escaping issues)

**cie_verify_absence** — Security audit tool. Verifies patterns do NOT exist. Returns PASS/FAIL. Use for:
- Checking for hardcoded secrets: patterns=["api_key", "password", "secret"]
- Scoping to sensitive areas: path="ui/src"

### Semantic Search Tools (meaning-based)

**cie_semantic_search** — Search by meaning using vector embeddings. Use when you don't know exact function names. Key parameters:
- query: Natural language description (e.g., "function that handles user authentication")
- role: Filter by code role — "source" (default, excludes tests), "handler", "router", "entry_point", "test"
- path_pattern: Scope to directory (e.g., "apps/gateway")
- exclude_paths: Remove noise (e.g., "metrics|telemetry|dlq")
- min_similarity: Set threshold (0.7 = high confidence only)
- Confidence indicators in results: 🟢 High (≥75%), 🟡 Medium (50-75%), 🔴 Low (<50%)

**cie_analyze** — Architectural Q&A with LLM narrative. Use for high-level questions that span multiple functions. Combines semantic search with keyword boosting and generates a narrative answer. Use for:
- "What are the main entry points?"
- "How does authentication work?"
- "What's the architecture of the gateway?"
- Scope with path_pattern for focused analysis.

### Code Navigation Tools

**cie_find_function** — Find functions by name. Handles Go receiver syntax (searching "Batch" finds "Batcher.Batch"). Use exact_match=true for precise lookups, include_code=true to get source inline.

**cie_get_function_code** — Get full source code of a function. Always use full_code=true for long functions — without it, output may be truncated.

**cie_find_callers** — Who calls this function? Set include_indirect=true for transitive callers (callers of callers).

**cie_find_callees** — What does this function call? Shows all outgoing dependencies.

**cie_get_call_graph** — Combined view: both callers and callees in one call.

**cie_trace_path** — Trace execution path from entry point to target function. Auto-detects entry points (main for Go, index exports for JS/TS, __main__ for Python). Use source parameter to trace between arbitrary functions. Increase max_depth for deeply nested targets.

### Type & Interface Tools

**cie_find_type** — Find types, structs, interfaces, classes by name. Filter by kind: "struct", "interface", "class", "type_alias".

**cie_find_implementations** — Find concrete types that implement an interface. Works for Go (struct method matching) and TypeScript (implements keyword).

### Architecture Discovery Tools

**cie_directory_summary** — Overview of a directory: files with their main exported functions. Start here when exploring an unfamiliar module.

**cie_list_files** — List all indexed files. Filter by language, path, or role. Good for understanding project layout.

**cie_list_functions_in_file** — All functions in a specific file. Useful after finding a file via cie_list_files.

**cie_get_file_summary** — All entities (functions, types, constants) in a file. More detailed than list_functions_in_file.

**cie_list_endpoints** — HTTP/REST endpoints from Go frameworks (Gin, Echo, Chi, Fiber, net/http). Returns [Method] [Path] [Handler] [File].

**cie_list_services** — gRPC service definitions and RPC methods from .proto files.

### Git History Tools

**cie_function_history** — Git commit history for a specific function. Use since="2024-01-01" to filter by date. Use path_pattern to disambiguate functions with the same name in different files.

**cie_find_introduction** — Find the commit that first introduced a code pattern (git pickaxe). Use for understanding when and why code was added.

**cie_blame_function** — Code ownership breakdown by author. Shows who wrote what percentage. Use show_lines=true for line-by-line detail.

### Database Tools

**cie_schema** — Get the CIE database schema, tables, fields, and example queries. Call this FIRST before using cie_raw_query.

**cie_raw_query** — Execute raw CozoScript (Datalog) queries. Powerful but requires knowledge of the schema. Always call cie_schema first.

**cie_index_status** — Check index health. Use this FIRST when searches return no results — the path might not be indexed.

## Common Parameters

Several tools share these parameters:

- **path_pattern**: Regex to scope search to a directory (e.g., "apps/gateway", "internal/cie"). Most tools support this.
- **exclude_pattern**: Regex to exclude files. Use [.] instead of \. for literal dots (e.g., "_test[.]go" not "_test\.go"). Combine with | for multiple patterns: "_test[.]go|[.]pb[.]go".
- **role**: Filter by code role. Values: "source" (excludes tests/generated), "test", "generated", "any". Default is usually "source".
- **limit**: Cap the number of results. Increase if you need more context; decrease for faster responses.

## Common Mistakes to Avoid

1. **Non-English queries** — All queries must be in English. Non-English terms won't match function names.
2. **Using cie_grep for regex** — cie_grep is literal-only. Use cie_search_text for regex patterns.
3. **Backslash in exclude_pattern** — Use [.] instead of \. for literal dots. CozoScript handles escaping differently.
4. **Not using full_code=true** — Long functions get truncated by default. Add full_code=true for complete output.
5. **Skipping cie_index_status** — When searches return nothing, check the index first. The path might not be indexed.
6. **Not excluding tests** — Most search tools default to role="source", but cie_grep does not filter by role. Use exclude_pattern="_test[.]go" with cie_grep to skip test files.
7. **Using cie_analyze for simple lookups** — cie_analyze is slow (calls LLM). Use cie_find_function or cie_grep for direct lookups.
8. **Not scoping searches** — Broad queries produce noisy results. Use path_pattern to focus on the relevant module.`

// jsonRPCRequest represents a JSON-RPC 2.0 request from the MCP client.
//
// The MCP protocol uses JSON-RPC 2.0 for all client-server communication.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"` // Request parameters (tool-specific)
}

// jsonRPCResponse represents a JSON-RPC 2.0 response to the MCP client.
//
// Contains either a result (on success) or an error (on failure), never both.
type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"` // Error details (if request failed)
}

// rpcError represents a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"` // Additional error data (optional)
}

// mcpServerInfo provides server identification for MCP protocol handshake.
type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpCapabilities struct {
	Tools map[string]any `json:"tools,omitempty"` // Tool capabilities declaration
}

// mcpInitializeResult is the response to the MCP initialize request.
//
// Sent during the initial handshake to declare protocol version, capabilities,
// server information, and usage instructions for AI agents.
type mcpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    mcpCapabilities `json:"capabilities"`
	ServerInfo      mcpServerInfo   `json:"serverInfo"`     // Server identification
	Instructions    string          `json:"instructions"`   // Usage instructions for AI agents
}

// mcpTool describes a single tool exposed by the MCP server.
//
// Each tool has a name, description, and JSON Schema defining its input parameters.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"` // JSON Schema for tool parameters
}

// mcpToolsListResult is the response to the tools/list request.
//
// Lists all tools exposed by the CIE MCP server.
type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"` // Tool-specific arguments
}

// mcpToolResult is the result of a tool execution.
//
// Contains the tool's output as an array of content blocks (typically text).
type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"` // True if tool execution failed
}

// mcpContent represents a single content block in a tool result.
//
// MCP supports multiple content types; CIE uses text content exclusively.
type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"` // Content text
}

// mcpServer maintains state for the running MCP server instance.
//
// Holds the CIE client for database queries, embedding configuration for
// semantic search, custom role patterns from the project configuration,
// and git executor for git history tools.
type mcpServer struct {
	client         tools.Querier
	projectID      string                 // Project ID for error messages
	mode           string                 // "embedded" or "remote" for logging
	embeddingURL   string
	embeddingModel string
	customRoles    map[string]RolePattern // Custom role patterns from config
	gitExecutor    tools.GitRunner        // Git executor for history tools (may be nil)
}

// runMCPServer starts the CIE Model Context Protocol server.
//
// It initializes a JSON-RPC 2.0 server over stdin/stdout, exposes 20+ code intelligence
// tools to AI assistants, and handles all MCP protocol messages including initialization,
// tool listing, and tool execution.
//
// The server runs indefinitely until stdin is closed or an unrecoverable error occurs.
//
// MCP Protocol Flow:
//  1. Client sends initialize request
//  2. Server responds with capabilities and server info
//  3. Client sends tools/list to discover available tools
//  4. Client sends tools/call requests to invoke specific tools
//  5. Server executes tool and returns results as content blocks
//
// Available tools include:
//   - Semantic search (cie_semantic_search, cie_analyze)
//   - Text search (cie_grep, cie_search_text, cie_verify_absence)
//   - Code navigation (cie_find_function, cie_get_function_code, cie_find_type)
//   - Call graph analysis (cie_find_callers, cie_find_callees, cie_trace_path)
//   - Architecture discovery (cie_list_endpoints, cie_list_services, cie_directory_summary)
//   - Database queries (cie_raw_query, cie_schema, cie_index_status)
//
// Configuration is loaded from .cie/project.yaml with environment variable overrides.
// If configuration loading fails, falls back to environment-only configuration.
//
// Parameters:
//   - configPath: Path to .cie/project.yaml (empty string to auto-detect)
func runMCPServer(configPath string) {
	// Log current working directory for debugging
	cwd, _ := os.Getwd()
	fmt.Fprintf(os.Stderr, "MCP Server CWD: %s\n", cwd)
	fmt.Fprintf(os.Stderr, "Config path arg: %q\n", configPath)

	cfg := loadMCPConfig(configPath)
	client, mode, projectID := setupMCPClient(cfg)

	fmt.Fprintf(os.Stderr, "  Embedding configured: %s (%s)\n", cfg.Embedding.BaseURL, cfg.Embedding.Model)

	server := &mcpServer{
		client:         client,
		projectID:      projectID,
		mode:           mode,
		embeddingURL:   cfg.Embedding.BaseURL,
		embeddingModel: cfg.Embedding.Model,
		customRoles:    cfg.Roles.Custom,
	}

	setupGitExecutor(server, configPath, cwd)

	fmt.Fprintf(os.Stderr, "CIE MCP Server v%s starting (%s mode)...\n", mcpVersion, server.mode)
	if server.mode == "remote" {
		fmt.Fprintf(os.Stderr, "  Edge Cache: %s\n", cfg.CIE.EdgeCache)
	}
	fmt.Fprintf(os.Stderr, "  Project: %s\n", server.projectID)

	serveMCPLoop(server)
}

// loadMCPConfig loads the config file or falls back to environment variables.
func loadMCPConfig(configPath string) *Config {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		ue := errors.NewConfigError(
			"Cannot load CIE configuration file",
			"Configuration file is missing or invalid",
			"Using environment variables as fallback. Run 'cie init' to create a proper config.",
			err,
		)
		fmt.Fprintf(os.Stderr, "%s\n", ue.Format(false))

		cfg = DefaultConfig("")
		cfg.applyEnvOverrides()
		fmt.Fprintf(os.Stderr, "Using env fallback: project=%s\n", cfg.ProjectID)
	} else {
		fmt.Fprintf(os.Stderr, "Config loaded: project=%s\n", cfg.ProjectID)
	}
	return cfg
}

// setupMCPClient creates the appropriate Querier based on config (embedded vs remote).
func setupMCPClient(cfg *Config) (tools.Querier, string, string) {
	// Warn if CIE_BASE_URL env var is overriding the config
	if envURL := os.Getenv("CIE_BASE_URL"); envURL != "" && cfg.CIE.EdgeCache == envURL {
		fmt.Fprintf(os.Stderr, "Note: CIE_BASE_URL=%s is set, using remote mode. Unset it for embedded mode.\n", envURL)
	}

	if cfg.CIE.EdgeCache == "" {
		return setupEmbeddedClient(cfg,
			"Cannot open local database",
			"Failed to open CozoDB for embedded MCP mode",
			"Check that ~/.cie/data/ is accessible. Run 'cie index' first if you haven't indexed yet.",
			"embedded",
		)
	}
	return setupRemoteClient(cfg)
}

// setupEmbeddedClient opens a local CozoDB backend and returns an EmbeddedQuerier.
func setupEmbeddedClient(cfg *Config, title, detail, suggestion, mode string) (tools.Querier, string, string) {
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		ProjectID:           cfg.ProjectID,
		Engine:              "rocksdb",
		EmbeddingDimensions: cfg.Embedding.Dimensions,
	})
	if err != nil {
		errors.FatalError(errors.NewDatabaseError(title, detail, suggestion, err), false)
	}
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		_ = backend.Close()
		os.Exit(0)
	}()
	return tools.NewEmbeddedQuerier(backend), mode, cfg.ProjectID
}

// setupRemoteClient configures a remote HTTP client with auto-fallback to embedded mode.
func setupRemoteClient(cfg *Config) (tools.Querier, string, string) {
	httpClient := tools.NewCIEClient(cfg.CIE.EdgeCache, cfg.ProjectID)

	if isReachable(cfg.CIE.EdgeCache) {
		httpClient.SetEmbeddingConfig(cfg.Embedding.BaseURL, cfg.Embedding.Model)
		return httpClient, "remote", cfg.ProjectID
	}

	// Remote unreachable — try local fallback
	if hasLocalData(cfg.ProjectID) {
		fmt.Fprintf(os.Stderr, "Warning: Edge Cache at %s is not reachable. Falling back to embedded mode.\n", cfg.CIE.EdgeCache)
		fmt.Fprintf(os.Stderr, "  Tip: Remove 'edge_cache' from .cie/project.yaml or run 'cie init --force -y' to use embedded mode by default.\n")
		return setupEmbeddedClient(cfg,
			"Cannot open local database",
			"Edge Cache is not reachable and local database failed to open",
			"Run 'cie init --force -y' to switch to embedded mode, then 'cie index' to index.",
			"embedded (fallback)",
		)
	}

	fmt.Fprintf(os.Stderr, "Warning: Edge Cache at %s is not reachable and no local data found.\n", cfg.CIE.EdgeCache)
	fmt.Fprintf(os.Stderr, "  Run 'cie init --force -y && cie index' to set up local mode.\n")
	httpClient.SetEmbeddingConfig(cfg.Embedding.BaseURL, cfg.Embedding.Model)
	return httpClient, "remote (unreachable)", cfg.ProjectID
}

// setupGitExecutor initializes the git executor for git history tools.
func setupGitExecutor(server *mcpServer, configPath, cwd string) {
	path := configPath
	if path == "" {
		path = cwd
	}
	gitExec, gitErr := tools.NewGitExecutor(path)
	if gitErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: Git history tools disabled: %v\n", gitErr)
		return
	}
	server.gitExecutor = gitExec
	fmt.Fprintf(os.Stderr, "  Git repo: %s\n", gitExec.RepoPath())
}

// serveMCPLoop reads JSON-RPC requests from stdin and writes responses to stdout.
func serveMCPLoop(server *mcpServer) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			ue := errors.NewInputError(
				"Invalid JSON in MCP request",
				"The request does not conform to JSON-RPC 2.0 format",
				"Check your MCP client configuration or update Claude Code/Cursor",
			)
			fmt.Fprintf(os.Stderr, "%s\n", ue.Format(false))
			continue
		}

		fmt.Fprintf(os.Stderr, "-> %s\n", req.Method)

		ctx := context.Background()
		resp := server.handleRequest(ctx, req)

		if resp.ID == nil && resp.Result == nil && resp.Error == nil {
			continue
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			ue := errors.NewInternalError(
				"Cannot encode MCP response",
				"Failed to marshal response to JSON",
				"This is a bug. Please report it with the request details.",
				err,
			)
			fmt.Fprintf(os.Stderr, "%s\n", ue.Format(false))
			continue
		}

		_, _ = fmt.Fprintf(os.Stdout, "%s\n", respBytes)
		_ = os.Stdout.Sync()

		fmt.Fprintf(os.Stderr, "<- response sent for %s\n", req.Method)
	}

	if err := scanner.Err(); err != nil {
		ue := errors.NewInternalError(
			"MCP server input error",
			"Failed to read from stdin",
			"Check if stdin is closed or if there's a pipe issue.",
			err,
		)
		errors.FatalError(ue, false)
	}
}

func (s *mcpServer) getTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "cie_index_status",
			Description: "Check the indexing status for a path. Shows how many files and functions are indexed, and warns if the index appears incomplete. Use this FIRST when searches return no results to verify the path is indexed.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Path pattern to check (e.g., 'apps/gateway' or 'internal/'). Leave empty to check entire index.",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "cie_schema",
			Description: "Get the CIE database schema, available tables, fields, operators, and example queries. Call this first to understand what data is available and how to query it.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []string{},
			},
		},
		{
			Name:        "cie_search_text",
			Description: "Search for text patterns in function code, signatures, or names. Returns matching functions with file path, line numbers, and context. IMPORTANT: Use literal=true for exact code patterns like '.GET(', '->', '::' etc. Only use regex mode for complex patterns.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Pattern to search for. For exact code (e.g., '.GET(', '->'), use with literal=true. For regex (e.g., '(?i)handler.*error'), use literal=false.",
					},
					"literal": map[string]any{
						"type":        "boolean",
						"description": "RECOMMENDED: Set to true for exact code patterns (like '.GET(', '::', '->'). Set to false (default) only for regex patterns.",
						"default":     false,
					},
					"search_in": map[string]any{
						"type":        "string",
						"enum":        []string{"code", "signature", "name", "all"},
						"description": "Where to search: 'code' (function body), 'signature', 'name', or 'all'",
						"default":     "all",
					},
					"file_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: filter by file path pattern (e.g., 'batcher.go', '.*_test.go')",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results to return (default: 20)",
						"default":     20,
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "cie_find_function",
			Description: "Find functions by name. Handles Go receiver syntax (e.g., searching 'Batch' finds 'Batcher.Batch'). Returns function details including signature, location, and code.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Function name to find. Can be exact ('NewBatcher') or partial ('Batch' finds 'Batcher.Batch')",
					},
					"exact_match": map[string]any{
						"type":        "boolean",
						"description": "If true, match exact name only. If false (default), also match methods containing the name.",
						"default":     false,
					},
					"include_code": map[string]any{
						"type":        "boolean",
						"description": "If true, include full function code in results",
						"default":     false,
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "cie_find_callers",
			Description: "Find all functions that call a specific function. Useful for understanding how a function is used throughout the codebase.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to find callers for (e.g., 'Batch', 'NewBatcher')",
					},
					"include_indirect": map[string]any{
						"type":        "boolean",
						"description": "If true, include indirect callers (callers of callers). Default: false",
						"default":     false,
					},
				},
				"required": []string{"function_name"},
			},
		},
		{
			Name:        "cie_find_callees",
			Description: "Find all functions called by a specific function. Useful for understanding a function's dependencies.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to find callees for",
					},
				},
				"required": []string{"function_name"},
			},
		},
		{
			Name:        "cie_find_type",
			Description: "Find types, interfaces, classes, or structs by name or pattern. Works across all languages: Go (struct/interface), Python (class), TypeScript (interface/class). Use this to find architectural definitions.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Type name to search for (e.g., 'UserService', 'Handler', 'Config')",
					},
					"kind": map[string]any{
						"type":        "string",
						"enum":        []string{"any", "struct", "interface", "class", "type_alias"},
						"description": "Filter by type kind: 'struct', 'interface', 'class', 'type_alias', or 'any' (default)",
						"default":     "any",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional regex pattern to filter file paths",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results (default: 20)",
						"default":     20,
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "cie_list_files",
			Description: "List files in the indexed codebase. Can filter by language, path pattern, or role.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Regex pattern to filter file paths (e.g., '.*batcher.*', 'internal/cie/.*')",
					},
					"language": map[string]any{
						"type":        "string",
						"description": "Filter by language (e.g., 'go', 'typescript', 'python')",
					},
					"role": map[string]any{
						"type":        "string",
						"enum":        []string{"any", "source", "test", "generated"},
						"description": "Filter by file role: 'source' (exclude tests/generated), 'test', 'generated', or 'any'",
						"default":     "source",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results (default: 50)",
						"default":     50,
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "cie_raw_query",
			Description: "Execute a raw CozoScript query against the CIE database. Use cie_schema first to understand the available tables and operators.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"script": map[string]any{
						"type":        "string",
						"description": "CozoScript query to execute. Example: ?[name, file_path] := *cie_function { name, file_path } :limit 10",
					},
				},
				"required": []string{"script"},
			},
		},
		{
			Name:        "cie_get_function_code",
			Description: "Get the full source code of a specific function by name. Returns the complete function implementation.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to get code for (e.g., 'NewBatcher', 'Pipeline.Run')",
					},
					"full_code": map[string]any{
						"type":        "boolean",
						"description": "If true, return complete code without truncation. Default: false (truncates long functions with hint to view full code)",
						"default":     false,
					},
				},
				"required": []string{"function_name"},
			},
		},
		{
			Name:        "cie_list_functions_in_file",
			Description: "List all functions defined in a specific file. Useful for understanding file structure.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Path to the file (e.g., 'internal/cie/ingestion/batcher.go')",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "cie_get_call_graph",
			Description: "Get the complete call graph for a function - both who calls it and what it calls.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to analyze",
					},
				},
				"required": []string{"function_name"},
			},
		},
		{
			Name:        "cie_find_similar_functions",
			Description: "Find functions with similar names or patterns. Useful for discovering related functionality.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Name pattern to search for (e.g., 'Handler', 'New', 'Parse')",
					},
				},
				"required": []string{"pattern"},
			},
		},
		{
			Name:        "cie_get_file_summary",
			Description: "Get a summary of all entities (functions, types, constants) defined in a file.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{
						"type":        "string",
						"description": "Path to the file to summarize",
					},
				},
				"required": []string{"file_path"},
			},
		},
		{
			Name:        "cie_semantic_search",
			Description: "Search for code by meaning/concept using vector similarity. Use natural language to describe what you're looking for (e.g., 'function that handles user authentication', 'code that parses JSON responses'). Returns the most semantically similar functions.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language description of what you're looking for",
					},
					"role": map[string]any{
						"type":        "string",
						"enum":        []string{"any", "source", "test", "generated", "entry_point", "router", "handler"},
						"description": "Filter by code role: 'source' (exclude tests/generated), 'entry_point' (main functions), 'router' (route definitions), 'handler' (HTTP handlers), 'test', 'generated', or 'any' (no filter)",
						"default":     "source",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional regex to filter by file path (e.g., 'apps/gateway' to only search in gateway)",
					},
					"exclude_paths": map[string]any{
						"type":        "string",
						"description": "Optional regex to exclude file paths. Use when results contain noise from specific directories (e.g., 'metrics|dlq|telemetry' to focus on core business logic)",
					},
					"exclude_anonymous": map[string]any{
						"type":        "boolean",
						"description": "Exclude anonymous/arrow functions like $arrow_X, $anon_X from results (default: true). Set to false to include them.",
						"default":     true,
					},
					"min_similarity": map[string]any{
						"type":        "number",
						"description": "Minimum similarity threshold (0.0-1.0, e.g., 0.5 = 50%). Only return results above this similarity score.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results (default: 10, max: 50)",
						"default":     10,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "cie_analyze",
			Description: "Analyze codebase structure and answer architectural questions. Use natural language to ask about: entry points, routes/endpoints, module organization, dependencies, patterns used, etc. Examples: 'What are the main entry points?', 'How are HTTP routes organized?', 'What's the architecture of the gateway service?'. By default, excludes test files for cleaner results.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "Natural language question about the codebase architecture or structure",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: focus analysis on specific path (e.g., 'apps/gateway')",
					},
					"role": map[string]any{
						"type":        "string",
						"enum":        []string{"source", "test", "any"},
						"description": "Filter results: 'source' (default, excludes tests), 'test' (only tests), 'any' (include all)",
						"default":     "source",
					},
				},
				"required": []string{"question"},
			},
		},
		{
			Name:        "cie_grep",
			Description: "Ultra-fast literal text search (like grep). Searches for EXACT text - no regex. Supports multi-pattern search via 'texts' array for batch searches (reduces API calls). Perfect for searching code patterns like '.GET(', '->', '::new', 'import'.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{
						"type":        "string",
						"description": "Single exact text to search for (e.g., '.GET(', 'func main'). Use 'texts' array for multiple patterns.",
					},
					"texts": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "RECOMMENDED: Array of patterns to search in parallel. Returns grouped results with counts per pattern. Example: ['access_token', 'refresh_token', 'secret']",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Optional: filter by file path substring (e.g., 'routes', 'internal/cie')",
					},
					"exclude_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: regex pattern to EXCLUDE files (e.g., '_test\\.go' to exclude tests, '\\.pb\\.go' to exclude generated). Multiple patterns can be combined with '|'.",
					},
					"case_sensitive": map[string]any{
						"type":        "boolean",
						"description": "If true, search is case-sensitive. Default: false (case-insensitive)",
						"default":     false,
					},
					"context": map[string]any{
						"type":        "integer",
						"description": "Number of lines to show before and after each match (like grep -C). Default: 0 (no context)",
						"default":     0,
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results per pattern (default: 30)",
						"default":     30,
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "cie_verify_absence",
			Description: "Verify that specific patterns do NOT exist in code. Returns PASS/FAIL with detailed violations. Perfect for security audits (no hardcoded secrets, tokens, credentials) and CI/CD checks. Example: verify absence of 'access_token', 'api_key', 'password' in frontend code.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"patterns": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Patterns that should NOT exist in code. Example: ['access_token', 'refresh_token', 'api_key', 'secret']",
					},
					"path": map[string]any{
						"type":        "string",
						"description": "Optional: limit check to specific path (e.g., 'ui/src', 'frontend/')",
					},
					"exclude_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: regex pattern to EXCLUDE files from check (e.g., '_test\\.go|mock')",
					},
					"case_sensitive": map[string]any{
						"type":        "boolean",
						"description": "If true, pattern matching is case-sensitive. Default: false",
						"default":     false,
					},
					"severity": map[string]any{
						"type":        "string",
						"enum":        []string{"critical", "warning", "info"},
						"description": "Severity level for violations. Default: 'warning'",
						"default":     "warning",
					},
				},
				"required": []string{"patterns"},
			},
		},
		{
			Name:        "cie_list_services",
			Description: "List gRPC services and RPC methods from .proto files. Shows service definitions, RPC methods, and their request/response types. Useful for understanding API contracts in gRPC-based projects.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: filter by file path (e.g., 'api/proto')",
					},
					"service_name": map[string]any{
						"type":        "string",
						"description": "Optional: filter by service name",
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "cie_directory_summary",
			Description: "Get a summary of a directory showing files with their main exported functions. Perfect for understanding the architecture of a module or package quickly. Shows file list with the most important functions in each.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Directory path to summarize (e.g., 'apps/gateway/internal/http', 'internal/cie/ingestion')",
					},
					"max_functions_per_file": map[string]any{
						"type":        "integer",
						"description": "Maximum number of functions to show per file (default: 5)",
						"default":     5,
					},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "cie_list_endpoints",
			Description: "List HTTP/REST endpoints defined in the codebase. Detects route definitions from common Go frameworks (Gin, Echo, Chi, Fiber, net/http). Returns a table of [Method] [Path] [Handler] [File]. Perfect for understanding API structure in gateway/server code.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: filter by file path (e.g., 'apps/gateway', 'internal/http')",
					},
					"path_filter": map[string]any{
						"type":        "string",
						"description": "Optional: filter by endpoint path substring (e.g., '/health', 'connections', '/api/v1'). Case-insensitive.",
					},
					"method": map[string]any{
						"type":        "string",
						"enum":        []string{"GET", "POST", "PUT", "DELETE", "PATCH", "ANY", ""},
						"description": "Optional: filter by HTTP method",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results (default: 100)",
						"default":     100,
					},
				},
				"required": []string{},
			},
		},
		{
			Name:        "cie_find_implementations",
			Description: "Find types that implement a given interface. For Go: finds structs with methods matching the interface. For TypeScript: finds classes with 'implements InterfaceName'. Useful for understanding interface usage and finding concrete implementations.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"interface_name": map[string]any{
						"type":        "string",
						"description": "Name of the interface to find implementations for (e.g., 'Reader', 'Handler', 'Repository')",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional regex to filter by file path",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results (default: 20)",
						"default":     20,
					},
				},
				"required": []string{"interface_name"},
			},
		},
		{
			Name:        "cie_trace_path",
			Description: "Trace call paths from source function(s) to a target function. Uses the call graph to find how execution reaches a specific function. Returns the shortest paths with full call chain and file locations. If no source is specified, auto-detects entry points based on language conventions (main for Go/Rust, index/app exports for JS/TS, __main__ for Python). Useful for understanding initialization flows, debugging, security audits, and refactoring impact analysis.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target": map[string]any{
						"type":        "string",
						"description": "Target function name to trace to (e.g., 'RegisterRoutes', 'handleAuth', 'db.connect')",
					},
					"source": map[string]any{
						"type":        "string",
						"description": "Source function name to trace from. If empty, auto-detects entry points (main for Go/Rust, index exports for JS/TS, __main__ for Python). Can be any function name to trace between arbitrary functions.",
					},
					"max_paths": map[string]any{
						"type":        "integer",
						"description": "Maximum number of paths to return (default: 3). Increase for complex codebases with many routes to the target.",
						"default":     3,
					},
					"max_depth": map[string]any{
						"type":        "integer",
						"description": "Maximum call depth to search (default: 10). Increase if target function is deeply nested in the call hierarchy.",
						"default":     10,
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: filter by file path to narrow the search scope (e.g., 'apps/gateway', 'src/server')",
					},
					"waypoints": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional: intermediate function names the path must pass through, in order. Chains BFS segments: source → wp1 → wp2 → ... → target. Useful when functions are far apart or when you know intermediate steps.",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			Name:        "cie_function_history",
			Description: "Get git commit history for a specific function. Tracks changes to the function over time using line-based git history. Useful for understanding when and why a function was modified.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to get history for (e.g., 'HandleAuth', 'NewBatcher')",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of commits to show (default: 10)",
						"default":     10,
					},
					"since": map[string]any{
						"type":        "string",
						"description": "Only show commits after this date (e.g., '2024-01-01', '3 months ago')",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: disambiguate when multiple functions have the same name",
					},
				},
				"required": []string{"function_name"},
			},
		},
		{
			Name:        "cie_find_introduction",
			Description: "Find the commit that first introduced a code pattern. Uses git pickaxe (-S) to find when a pattern was first added to the codebase. Useful for understanding the origin of code, debugging, and security audits.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"code_snippet": map[string]any{
						"type":        "string",
						"description": "The code pattern to find the introduction of (e.g., 'jwt.Generate()', 'access_token :=')",
					},
					"function_name": map[string]any{
						"type":        "string",
						"description": "Optional: limit search to the file containing this function",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: limit search scope to specific paths",
					},
				},
				"required": []string{"code_snippet"},
			},
		},
		{
			Name:        "cie_blame_function",
			Description: "Get aggregated blame analysis for a function showing code ownership. Returns a breakdown of who wrote what percentage of the function, useful for identifying experts, reviewers, and understanding code ownership.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"function_name": map[string]any{
						"type":        "string",
						"description": "Name of the function to analyze (e.g., 'RegisterRoutes', 'Parse')",
					},
					"path_pattern": map[string]any{
						"type":        "string",
						"description": "Optional: disambiguate when multiple functions have the same name",
					},
					"show_lines": map[string]any{
						"type":        "boolean",
						"description": "Include line-by-line breakdown (default: false)",
						"default":     false,
					},
				},
				"required": []string{"function_name"},
			},
		},
	}
}

// toolHandler is the signature for MCP tool handlers.
type toolHandler func(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error)

// toolHandlers maps tool names to their handlers.
var toolHandlers = map[string]toolHandler{
	"cie_schema":                 handleSchema,
	"cie_search_text":            handleSearchText,
	"cie_find_function":          handleFindFunction,
	"cie_find_callers":           handleFindCallers,
	"cie_find_callees":           handleFindCallees,
	"cie_list_files":             handleListFiles,
	"cie_raw_query":              handleRawQuery,
	"cie_get_function_code":      handleGetFunctionCode,
	"cie_list_functions_in_file": handleListFunctionsInFile,
	"cie_get_call_graph":         handleGetCallGraph,
	"cie_find_similar_functions": handleFindSimilarFunctions,
	"cie_get_file_summary":       handleGetFileSummary,
	"cie_semantic_search":        handleSemanticSearch,
	"cie_analyze":                handleAnalyze,
	"cie_find_type":              handleFindType,
	"cie_index_status":           handleIndexStatus,
	"cie_grep":                   handleGrep,
	"cie_verify_absence":         handleVerifyAbsence,
	"cie_list_services":          handleListServices,
	"cie_directory_summary":      handleDirectorySummary,
	"cie_list_endpoints":         handleListEndpoints,
	"cie_find_implementations":   handleFindImplementations,
	"cie_trace_path":             handleTracePath,
	"cie_function_history":       handleFunctionHistory,
	"cie_find_introduction":      handleFindIntroduction,
	"cie_blame_function":         handleBlameFunction,
}

func (s *mcpServer) handleToolCall(ctx context.Context, params mcpToolCallParams) (*mcpToolResult, error) {
	handler, ok := toolHandlers[params.Name]
	if !ok {
		return &mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}, nil
	}

	result, err := handler(ctx, s, params.Arguments)
	if err != nil {
		return s.formatError(params.Name, err), nil
	}

	return &mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: result.Text}},
		IsError: result.IsError,
	}, nil
}

func handleSchema(ctx context.Context, _ *mcpServer, _ map[string]any) (*tools.ToolResult, error) {
	return tools.GetSchema(ctx)
}

func handleSearchText(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	literal, _ := args["literal"].(bool)
	searchIn, _ := args["search_in"].(string)
	filePattern, _ := args["file_pattern"].(string)
	excludePattern, _ := args["exclude_pattern"].(string)
	limit, _ := getIntArg(args, "limit", 20)

	return tools.SearchText(ctx, s.client, tools.SearchTextArgs{
		Pattern:        pattern,
		FilePattern:    filePattern,
		ExcludePattern: excludePattern,
		SearchIn:       searchIn,
		Literal:        literal,
		Limit:          limit,
	})
}

func handleFindFunction(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	name, _ := args["name"].(string)
	exactMatch, _ := args["exact_match"].(bool)
	includeCode, _ := args["include_code"].(bool)
	return tools.FindFunction(ctx, s.client, tools.FindFunctionArgs{
		Name:        name,
		ExactMatch:  exactMatch,
		IncludeCode: includeCode,
	})
}

func handleFindCallers(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	funcName, _ := args["function_name"].(string)
	includeIndirect, _ := args["include_indirect"].(bool)
	return tools.FindCallers(ctx, s.client, tools.FindCallersArgs{
		FunctionName:    funcName,
		IncludeIndirect: includeIndirect,
	})
}

func handleFindCallees(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	funcName, _ := args["function_name"].(string)
	return tools.FindCallees(ctx, s.client, tools.FindCalleesArgs{
		FunctionName: funcName,
	})
}

func handleListFiles(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pathPattern, _ := args["path_pattern"].(string)
	language, _ := args["language"].(string)
	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	return tools.ListFiles(ctx, s.client, tools.ListFilesArgs{
		PathPattern: pathPattern,
		Language:    language,
		Limit:       limit,
	})
}

func handleRawQuery(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	script, _ := args["script"].(string)
	return tools.RawQuery(ctx, s.client, tools.RawQueryArgs{
		Script: script,
	})
}

func handleGetFunctionCode(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	funcName, _ := args["function_name"].(string)
	fullCode, _ := args["full_code"].(bool)
	return tools.GetFunctionCode(ctx, s.client, tools.GetFunctionCodeArgs{
		FunctionName: funcName,
		FullCode:     fullCode,
	})
}

func handleListFunctionsInFile(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	return tools.ListFunctionsInFile(ctx, s.client, tools.ListFunctionsInFileArgs{
		FilePath: filePath,
	})
}

func handleGetCallGraph(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	funcName, _ := args["function_name"].(string)
	return tools.GetCallGraph(ctx, s.client, tools.GetCallGraphArgs{
		FunctionName: funcName,
	})
}

func handleFindSimilarFunctions(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	return tools.FindSimilarFunctions(ctx, s.client, tools.FindSimilarFunctionsArgs{
		Pattern: pattern,
	})
}

func handleGetFileSummary(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	filePath, _ := args["file_path"].(string)
	return tools.GetFileSummary(ctx, s.client, tools.GetFileSummaryArgs{
		FilePath: filePath,
	})
}

func handleSemanticSearch(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	query, _ := args["query"].(string)
	limit, _ := getIntArg(args, "limit", 10)
	role, _ := args["role"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	excludePaths, _ := args["exclude_paths"].(string)
	excludeAnonymous := true
	if v, ok := args["exclude_anonymous"].(bool); ok {
		excludeAnonymous = v
	}
	minSimilarity, _ := getFloatArg(args, "min_similarity", 0)

	return tools.SemanticSearch(ctx, s.client, tools.SemanticSearchArgs{
		Query:            query,
		Limit:            limit,
		Role:             role,
		PathPattern:      pathPattern,
		ExcludePaths:     excludePaths,
		ExcludeAnonymous: excludeAnonymous,
		MinSimilarity:    minSimilarity,
		EmbeddingURL:     s.embeddingURL,
		EmbeddingModel:   s.embeddingModel,
	})
}

func handleAnalyze(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	question, _ := args["question"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	role, _ := args["role"].(string)
	return tools.Analyze(ctx, s.client, tools.AnalyzeArgs{
		Question:    question,
		PathPattern: pathPattern,
		Role:        role,
	})
}

func handleFindType(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	name, _ := args["name"].(string)
	kind, _ := args["kind"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	limit, _ := getIntArg(args, "limit", 20)
	return tools.FindType(ctx, s.client, tools.FindTypeArgs{
		Name:        name,
		Kind:        kind,
		PathPattern: pathPattern,
		Limit:       limit,
	})
}

func handleIndexStatus(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pathPattern, _ := args["path_pattern"].(string)
	return tools.IndexStatus(ctx, s.client, pathPattern, s.projectID, s.mode)
}

func handleGrep(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	text, _ := args["text"].(string)
	path, _ := args["path"].(string)
	excludePattern, _ := args["exclude_pattern"].(string)
	caseSensitive, _ := args["case_sensitive"].(bool)
	contextLines, _ := getIntArg(args, "context", 0)
	limit, _ := getIntArg(args, "limit", 30)

	texts := extractStringArray(args, "texts")

	return tools.Grep(ctx, s.client, tools.GrepArgs{
		Text:           text,
		Texts:          texts,
		Path:           path,
		ExcludePattern: excludePattern,
		CaseSensitive:  caseSensitive,
		ContextLines:   contextLines,
		Limit:          limit,
	})
}

func handleVerifyAbsence(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	path, _ := args["path"].(string)
	excludePattern, _ := args["exclude_pattern"].(string)
	caseSensitive, _ := args["case_sensitive"].(bool)
	severity, _ := args["severity"].(string)

	patterns := extractStringArray(args, "patterns")

	return tools.VerifyAbsence(ctx, s.client, tools.VerifyAbsenceArgs{
		Patterns:       patterns,
		Path:           path,
		ExcludePattern: excludePattern,
		CaseSensitive:  caseSensitive,
		Severity:       severity,
	})
}

func handleListServices(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pathPattern, _ := args["path_pattern"].(string)
	serviceName, _ := args["service_name"].(string)
	return tools.ListServices(ctx, s.client, pathPattern, serviceName)
}

func handleDirectorySummary(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	path, _ := args["path"].(string)
	maxFuncs, _ := getIntArg(args, "max_functions_per_file", 5)
	return tools.DirectorySummary(ctx, s.client, path, maxFuncs)
}

func handleListEndpoints(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	pathPattern, _ := args["path_pattern"].(string)
	pathFilter, _ := args["path_filter"].(string)
	method, _ := args["method"].(string)
	limit, _ := getIntArg(args, "limit", 100)
	return tools.ListEndpoints(ctx, s.client, tools.ListEndpointsArgs{
		PathPattern: pathPattern,
		PathFilter:  pathFilter,
		Method:      method,
		Limit:       limit,
	})
}

func handleFindImplementations(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	interfaceName, _ := args["interface_name"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	limit, _ := getIntArg(args, "limit", 20)
	return tools.FindImplementations(ctx, s.client, tools.FindImplementationsArgs{
		InterfaceName: interfaceName,
		PathPattern:   pathPattern,
		Limit:         limit,
	})
}

func handleTracePath(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	target, _ := args["target"].(string)
	source, _ := args["source"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	maxPaths, _ := getIntArg(args, "max_paths", 3)
	maxDepth, _ := getIntArg(args, "max_depth", 5)
	waypoints := extractStringArray(args, "waypoints")
	return tools.TracePath(ctx, s.client, tools.TracePathArgs{
		Target:      target,
		Source:      source,
		PathPattern: pathPattern,
		MaxPaths:    maxPaths,
		MaxDepth:    maxDepth,
		Waypoints:   waypoints,
	})
}

func handleFunctionHistory(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	if s.gitExecutor == nil {
		return tools.NewError("Git history tools are not available. Git repository not detected."), nil
	}
	funcName, _ := args["function_name"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	since, _ := args["since"].(string)
	limit, _ := getIntArg(args, "limit", 10)
	return tools.FunctionHistory(ctx, s.client, s.gitExecutor, tools.FunctionHistoryArgs{
		FunctionName: funcName,
		Limit:        limit,
		Since:        since,
		PathPattern:  pathPattern,
	})
}

func handleFindIntroduction(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	if s.gitExecutor == nil {
		return tools.NewError("Git history tools are not available. Git repository not detected."), nil
	}
	codeSnippet, _ := args["code_snippet"].(string)
	funcName, _ := args["function_name"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	return tools.FindIntroduction(ctx, s.client, s.gitExecutor, tools.FindIntroductionArgs{
		CodeSnippet:  codeSnippet,
		FunctionName: funcName,
		PathPattern:  pathPattern,
	})
}

func handleBlameFunction(ctx context.Context, s *mcpServer, args map[string]any) (*tools.ToolResult, error) {
	if s.gitExecutor == nil {
		return tools.NewError("Git history tools are not available. Git repository not detected."), nil
	}
	funcName, _ := args["function_name"].(string)
	pathPattern, _ := args["path_pattern"].(string)
	showLines, _ := args["show_lines"].(bool)
	return tools.BlameFunction(ctx, s.client, s.gitExecutor, tools.BlameFunctionArgs{
		FunctionName: funcName,
		PathPattern:  pathPattern,
		ShowLines:    showLines,
	})
}

// extractStringArray extracts a string array from the arguments map.
func extractStringArray(args map[string]any, key string) []string {
	var result []string
	if raw, ok := args[key].([]interface{}); ok {
		for _, item := range raw {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// formatError creates an actionable error message based on the error type and tool
func (s *mcpServer) formatError(toolName string, err error) *mcpToolResult {
	errStr := err.Error()
	var msg string
	var suggestFallback bool

	// Check for common error patterns
	switch {
	case tools.ContainsStr(errStr, "connection refused"):
		if s.mode == "embedded" {
			msg = "**Database Error:** Cannot read local database\n\n"
			msg += "### How to fix:\n"
			msg += "- Run 'cie index' to index the project first\n"
			msg += "- Check that ~/.cie/data/ exists and is accessible\n"
		} else {
			msg = "**Connection Error:** Cannot connect to Edge Cache\n\n"
			msg += "### Possible causes:\n"
			msg += "1. Edge Cache server is not running\n"
			msg += "2. The URL in `.cie/project.yaml` is incorrect\n"
			msg += "3. Network/firewall is blocking the connection\n\n"
			msg += "### How to fix:\n"
			msg += "- Start Edge Cache: `cie edge-cache`\n"
			msg += "- Verify the `edge_cache` URL in `.cie/project.yaml`\n"
		}
		suggestFallback = true

	case tools.ContainsStr(errStr, "503") || tools.ContainsStr(errStr, "Service Unavailable"):
		msg = "**Service Unavailable (503):** Edge Cache returned an error\n\n"
		// Check if this looks like a regex evaluation failure
		if tools.ContainsStr(errStr, "Evaluation") || tools.ContainsStr(errStr, "regex") {
			msg += "### Likely cause: Invalid or complex regex pattern\n\n"
			msg += "The regex pattern may be:\n"
			msg += "1. Invalid syntax (unescaped special characters)\n"
			msg += "2. Too complex for the database to evaluate efficiently\n\n"
			msg += "### Solutions:\n"
			msg += "- Use `cie_search_text` with `literal: true` for exact patterns\n"
			msg += "- Simplify your regex pattern\n"
			msg += "- Use `cie_find_function` for simple name searches\n"
		} else {
			msg += "### Possible causes:\n"
			msg += "1. Edge Cache is still syncing with Primary Hub\n"
			msg += "2. Database query failed internally\n"
			msg += "3. Query pattern too complex\n\n"
			msg += "### How to fix:\n"
			msg += "- Wait a few seconds and retry\n"
			msg += "- Try a simpler query\n"
			msg += "- Check Edge Cache logs for detailed errors\n"
		}
		suggestFallback = true

	case tools.ContainsStr(errStr, "404") || tools.ContainsStr(errStr, "Not Found"):
		msg = "**Not Found (404):** The requested resource doesn't exist\n\n"
		msg += "### Possible causes:\n"
		msg += "1. Project hasn't been indexed yet\n"
		msg += "2. Project ID doesn't match\n\n"
		msg += "### How to fix:\n"
		msg += "- Run `cie index` to index the project\n"
		msg += fmt.Sprintf("- Check that project_id is `%s` in `.cie/project.yaml`\n", s.projectID)
		suggestFallback = true

	case tools.ContainsStr(errStr, "timeout") || tools.ContainsStr(errStr, "deadline exceeded"):
		msg = "**Timeout Error:** Query took too long to execute\n\n"
		msg += "### Possible causes:\n"
		msg += "1. Query is too complex or returns too many results\n"
		msg += "2. Network latency to Edge Cache\n"
		msg += "3. Edge Cache is overloaded\n\n"
		msg += "### How to fix:\n"
		msg += "- Try a more specific query with filters\n"
		msg += "- Reduce the limit parameter\n"
		msg += "- Check network connectivity\n"
		suggestFallback = true

	case tools.ContainsStr(errStr, "query:") || tools.ContainsStr(errStr, "CozoScript") || tools.ContainsStr(errStr, "parse error"):
		msg = "**Query Error:** Database query syntax error\n\n"
		msg += fmt.Sprintf("```\n%s\n```\n\n", errStr)
		msg += "### This is usually a bug in the tool. Please report it.\n"
		msg += "### Workaround:\n"
		msg += "- Try `cie_search_text` with `literal: true` for exact string matching\n"
		msg += "- Use `cie_list_files` to verify what's indexed\n"
		suggestFallback = true

	case tools.ContainsStr(errStr, "no rows") || tools.ContainsStr(errStr, "not found"):
		msg = fmt.Sprintf("**No Results:** %s found no matching data\n\n", toolName)
		msg += "### Possible causes:\n"
		msg += "1. The search pattern doesn't match any indexed content\n"
		msg += "2. The path/file hasn't been indexed\n\n"
		msg += "### How to check:\n"
		msg += "- Use `cie_index_status` to verify what's indexed\n"
		msg += "- Use `cie_list_files` to see available files\n"
		msg += "- Try a broader search pattern\n"
		suggestFallback = true

	default:
		// Generic error with context
		msg = fmt.Sprintf("**Error in %s:**\n```\n%s\n```\n\n", toolName, errStr)
		msg += "### General troubleshooting:\n"
		msg += "1. Check Edge Cache is running: `cie_index_status`\n"
		msg += "2. Verify the project is indexed: `cie_list_files`\n"
		msg += "3. Check `.cie/project.yaml` configuration\n"
		suggestFallback = true
	}

	// Add filesystem fallback suggestions for all blocking errors
	if suggestFallback {
		msg += "\n---\n### Filesystem Fallback\n"
		msg += "If CIE is unavailable, you can use these filesystem tools instead:\n"
		msg += "- **Glob**: Find files by pattern (e.g., `**/*.go`, `src/**/*.ts`)\n"
		msg += "- **Grep**: Search content with regex (e.g., `func.*Handler`)\n"
		msg += "- **Read**: Read specific files by path\n"
		msg += "\n_These tools work directly on the filesystem without requiring CIE._\n"
	}

	return &mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func (s *mcpServer) handleRequest(ctx context.Context, req jsonRPCRequest) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpInitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: mcpCapabilities{
					Tools: map[string]any{"listChanged": true},
				},
				ServerInfo: mcpServerInfo{
					Name:    mcpServerName,
					Version: mcpVersion,
				},
				Instructions: cieInstructions,
			},
		}

	case "notifications/initialized":
		return jsonRPCResponse{}

	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpToolsListResult{
				Tools: s.getTools(),
			},
		}

	case "tools/call":
		var params mcpToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &rpcError{
					Code:    -32602,
					Message: "Invalid params",
					Data:    err.Error(),
				},
			}
		}

		result, err := s.handleToolCall(ctx, params)
		if err != nil {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &rpcError{
					Code:    -32603,
					Message: "Internal error",
					Data:    err.Error(),
				},
			}
		}

		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32601,
				Message: "Method not found",
				Data:    req.Method,
			},
		}
	}
}

// getIntArg retrieves an integer argument from the params map, with a default fallback
func getIntArg(args map[string]interface{}, key string, fallback int) (int, bool) {
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f), true
		}
		if i, ok := v.(int); ok {
			return i, true
		}
	}
	return fallback, false
}

func getFloatArg(args map[string]interface{}, key string, fallback float64) (float64, bool) {
	if v, ok := args[key]; ok {
		if f, ok := v.(float64); ok {
			return f, true
		}
		if i, ok := v.(int); ok {
			return float64(i), true
		}
	}
	return fallback, false
}

// isReachable checks if a URL responds within a short timeout.
func isReachable(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url + "/health")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// hasLocalData checks if there's an existing CozoDB database for the project.
func hasLocalData(projectID string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	dataDir := filepath.Join(homeDir, ".cie", "data", projectID)
	info, err := os.Stat(dataDir)
	return err == nil && info.IsDir()
}
