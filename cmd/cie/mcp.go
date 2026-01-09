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
	"os"

	"github.com/kraklabs/cie/pkg/llm"
	"github.com/kraklabs/cie/pkg/tools"
)

const (
	mcpVersion    = "1.4.0" // Added multi-pattern grep, verify_absence tool, improved endpoints summary
	mcpServerName = "cie"

	// semanticSearchPathFilterK is the number of HNSW candidates to retrieve
	// when path_pattern filtering is needed. Must be large because matching
	// paths may be scattered across the vector space.
	semanticSearchPathFilterK = 2000

	// semanticSearchMinEf is the minimum ef parameter for HNSW search.
	// ef controls how many neighbors are explored during search.
	semanticSearchMinEf = 50
)

// traceFuncInfo holds function metadata for call path tracing
type traceFuncInfo struct {
	name     string
	filePath string
	line     string
}

// JSON-RPC 2.0 structures
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP structures
type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpCapabilities struct {
	Tools map[string]any `json:"tools,omitempty"`
}

type mcpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    mcpCapabilities `json:"capabilities"`
	ServerInfo      mcpServerInfo   `json:"serverInfo"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpToolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCP Server
type mcpServer struct {
	client         *tools.CIEClient
	embeddingURL   string
	embeddingModel string
	customRoles    map[string]RolePattern // Custom role patterns from config
}

func runMCPServer(configPath string) {
	var cfg *Config
	var err error

	// Log current working directory for debugging
	cwd, _ := os.Getwd()
	fmt.Fprintf(os.Stderr, "MCP Server CWD: %s\n", cwd)
	fmt.Fprintf(os.Stderr, "Config path arg: %q\n", configPath)

	// Try to load config
	cfg, err = LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config load failed: %v\n", err)
		// Fall back to environment variables
		cfg = DefaultConfig("")
		cfg.applyEnvOverrides() // Apply LLM and other env overrides
		fmt.Fprintf(os.Stderr, "Using env fallback: project=%s, llm.enabled=%v, llm.url=%s\n",
			cfg.ProjectID, cfg.LLM.Enabled, cfg.LLM.BaseURL)
	} else {
		fmt.Fprintf(os.Stderr, "Config loaded: project=%s, llm.enabled=%v, llm.url=%s\n",
			cfg.ProjectID, cfg.LLM.Enabled, cfg.LLM.BaseURL)
	}

	client := tools.NewCIEClient(cfg.CIE.EdgeCache, cfg.ProjectID)

	// Configure embedding for semantic search in analyze
	client.SetEmbeddingConfig(cfg.Embedding.BaseURL, cfg.Embedding.Model)
	fmt.Fprintf(os.Stderr, "  Embedding configured: %s (%s)\n", cfg.Embedding.BaseURL, cfg.Embedding.Model)

	// Configure LLM provider if enabled
	if cfg.LLM.Enabled && cfg.LLM.BaseURL != "" {
		provider, err := llm.NewProvider(llm.ProviderConfig{
			Type:         "openai", // All providers are OpenAI-compatible
			BaseURL:      cfg.LLM.BaseURL,
			DefaultModel: cfg.LLM.Model,
			APIKey:       cfg.LLM.APIKey,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to configure LLM provider: %v\n", err)
		} else {
			client.SetLLMProvider(provider, cfg.LLM.MaxTokens)
			fmt.Fprintf(os.Stderr, "  LLM configured: %s (%s)\n", cfg.LLM.BaseURL, cfg.LLM.Model)
		}
	} else {
		fmt.Fprintf(os.Stderr, "  LLM not enabled (enabled=%v, url=%q)\n", cfg.LLM.Enabled, cfg.LLM.BaseURL)
	}

	server := &mcpServer{
		client:         client,
		embeddingURL:   cfg.Embedding.BaseURL,
		embeddingModel: cfg.Embedding.Model,
		customRoles:    cfg.Roles.Custom,
	}

	fmt.Fprintf(os.Stderr, "CIE MCP Server v%s starting...\n", mcpVersion)
	fmt.Fprintf(os.Stderr, "  Edge Cache: %s\n", server.client.BaseURL)
	fmt.Fprintf(os.Stderr, "  Project: %s\n", server.client.ProjectID)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "JSON parse error: %v\n", err)
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
			fmt.Fprintf(os.Stderr, "Marshal error: %v\n", err)
			continue
		}

		fmt.Fprintf(os.Stdout, "%s\n", respBytes)
		os.Stdout.Sync()

		fmt.Fprintf(os.Stderr, "<- response sent for %s\n", req.Method)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Scanner error: %v\n", err)
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
				},
				"required": []string{"target"},
			},
		},
	}
}

func (s *mcpServer) handleToolCall(ctx context.Context, params mcpToolCallParams) (*mcpToolResult, error) {
	var result *tools.ToolResult
	var err error

	switch params.Name {
	case "cie_schema":
		result, err = tools.GetSchema(ctx)

	case "cie_search_text":
		pattern, _ := params.Arguments["pattern"].(string)
		literal, _ := params.Arguments["literal"].(bool)
		searchIn, _ := params.Arguments["search_in"].(string)
		filePattern, _ := params.Arguments["file_pattern"].(string)
		excludePattern, _ := params.Arguments["exclude_pattern"].(string)
		limit, _ := getIntArg(params.Arguments, "limit", 20)

		result, err = tools.SearchText(ctx, s.client, tools.SearchTextArgs{
			Pattern:        pattern,
			FilePattern:    filePattern,
			ExcludePattern: excludePattern,
			SearchIn:       searchIn,
			Literal:        literal,
			Limit:          limit,
		})

	case "cie_find_function":
		name, _ := params.Arguments["name"].(string)
		exactMatch, _ := params.Arguments["exact_match"].(bool)
		includeCode, _ := params.Arguments["include_code"].(bool)
		result, err = tools.FindFunction(ctx, s.client, tools.FindFunctionArgs{
			Name:        name,
			ExactMatch:  exactMatch,
			IncludeCode: includeCode,
		})

	case "cie_find_callers":
		funcName, _ := params.Arguments["function_name"].(string)
		includeIndirect, _ := params.Arguments["include_indirect"].(bool)
		result, err = tools.FindCallers(ctx, s.client, tools.FindCallersArgs{
			FunctionName:    funcName,
			IncludeIndirect: includeIndirect,
		})

	case "cie_find_callees":
		funcName, _ := params.Arguments["function_name"].(string)
		result, err = tools.FindCallees(ctx, s.client, tools.FindCalleesArgs{
			FunctionName: funcName,
		})

	case "cie_list_files":
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		language, _ := params.Arguments["language"].(string)
		limit := 50
		if l, ok := params.Arguments["limit"].(float64); ok {
			limit = int(l)
		}
		result, err = tools.ListFiles(ctx, s.client, tools.ListFilesArgs{
			PathPattern: pathPattern,
			Language:    language,
			Limit:       limit,
		})

	case "cie_raw_query":
		script, _ := params.Arguments["script"].(string)
		result, err = tools.RawQuery(ctx, s.client, tools.RawQueryArgs{
			Script: script,
		})

	case "cie_get_function_code":
		funcName, _ := params.Arguments["function_name"].(string)
		fullCode, _ := params.Arguments["full_code"].(bool)
		result, err = tools.GetFunctionCode(ctx, s.client, tools.GetFunctionCodeArgs{
			FunctionName: funcName,
			FullCode:     fullCode,
		})

	case "cie_list_functions_in_file":
		filePath, _ := params.Arguments["file_path"].(string)
		result, err = tools.ListFunctionsInFile(ctx, s.client, tools.ListFunctionsInFileArgs{
			FilePath: filePath,
		})

	case "cie_get_call_graph":
		funcName, _ := params.Arguments["function_name"].(string)
		result, err = tools.GetCallGraph(ctx, s.client, tools.GetCallGraphArgs{
			FunctionName: funcName,
		})

	case "cie_find_similar_functions":
		pattern, _ := params.Arguments["pattern"].(string)
		result, err = tools.FindSimilarFunctions(ctx, s.client, tools.FindSimilarFunctionsArgs{
			Pattern: pattern,
		})

	case "cie_get_file_summary":
		filePath, _ := params.Arguments["file_path"].(string)
		result, err = tools.GetFileSummary(ctx, s.client, tools.GetFileSummaryArgs{
			FilePath: filePath,
		})

	case "cie_semantic_search":
		query, _ := params.Arguments["query"].(string)
		limit, _ := getIntArg(params.Arguments, "limit", 10)
		role, _ := params.Arguments["role"].(string)
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		excludePaths, _ := params.Arguments["exclude_paths"].(string)
		excludeAnonymous := true // default
		if v, ok := params.Arguments["exclude_anonymous"].(bool); ok {
			excludeAnonymous = v
		}
		minSimilarity, _ := getFloatArg(params.Arguments, "min_similarity", 0)

		result, err = tools.SemanticSearch(ctx, s.client, tools.SemanticSearchArgs{
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

	case "cie_analyze":
		question, _ := params.Arguments["question"].(string)
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		role, _ := params.Arguments["role"].(string)
		result, err = tools.Analyze(ctx, s.client, tools.AnalyzeArgs{
			Question:    question,
			PathPattern: pathPattern,
			Role:        role,
		})

	case "cie_find_type":
		name, _ := params.Arguments["name"].(string)
		kind, _ := params.Arguments["kind"].(string)
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		limit, _ := getIntArg(params.Arguments, "limit", 20)
		result, err = tools.FindType(ctx, s.client, tools.FindTypeArgs{
			Name:        name,
			Kind:        kind,
			PathPattern: pathPattern,
			Limit:       limit,
		})

	case "cie_index_status":
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		result, err = tools.IndexStatus(ctx, s.client, pathPattern)

	case "cie_grep":
		text, _ := params.Arguments["text"].(string)
		path, _ := params.Arguments["path"].(string)
		excludePattern, _ := params.Arguments["exclude_pattern"].(string)
		caseSensitive, _ := params.Arguments["case_sensitive"].(bool)
		contextLines, _ := getIntArg(params.Arguments, "context", 0)
		limit, _ := getIntArg(params.Arguments, "limit", 30)

		// Handle texts array for multi-pattern search
		var texts []string
		if textsRaw, ok := params.Arguments["texts"].([]interface{}); ok {
			for _, t := range textsRaw {
				if s, ok := t.(string); ok {
					texts = append(texts, s)
				}
			}
		}

		result, err = tools.Grep(ctx, s.client, tools.GrepArgs{
			Text:           text,
			Texts:          texts,
			Path:           path,
			ExcludePattern: excludePattern,
			CaseSensitive:  caseSensitive,
			ContextLines:   contextLines,
			Limit:          limit,
		})

	case "cie_verify_absence":
		path, _ := params.Arguments["path"].(string)
		excludePattern, _ := params.Arguments["exclude_pattern"].(string)
		caseSensitive, _ := params.Arguments["case_sensitive"].(bool)
		severity, _ := params.Arguments["severity"].(string)

		// Handle patterns array
		var patterns []string
		if patternsRaw, ok := params.Arguments["patterns"].([]interface{}); ok {
			for _, p := range patternsRaw {
				if s, ok := p.(string); ok {
					patterns = append(patterns, s)
				}
			}
		}

		result, err = tools.VerifyAbsence(ctx, s.client, tools.VerifyAbsenceArgs{
			Patterns:       patterns,
			Path:           path,
			ExcludePattern: excludePattern,
			CaseSensitive:  caseSensitive,
			Severity:       severity,
		})

	case "cie_list_services":
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		serviceName, _ := params.Arguments["service_name"].(string)
		result, err = tools.ListServices(ctx, s.client, pathPattern, serviceName)

	case "cie_directory_summary":
		path, _ := params.Arguments["path"].(string)
		maxFuncs, _ := getIntArg(params.Arguments, "max_functions_per_file", 5)
		result, err = tools.DirectorySummary(ctx, s.client, path, maxFuncs)

	case "cie_list_endpoints":
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		pathFilter, _ := params.Arguments["path_filter"].(string)
		method, _ := params.Arguments["method"].(string)
		limit, _ := getIntArg(params.Arguments, "limit", 100)
		result, err = tools.ListEndpoints(ctx, s.client, tools.ListEndpointsArgs{
			PathPattern: pathPattern,
			PathFilter:  pathFilter,
			Method:      method,
			Limit:       limit,
		})

	case "cie_find_implementations":
		interfaceName, _ := params.Arguments["interface_name"].(string)
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		limit, _ := getIntArg(params.Arguments, "limit", 20)

		result, err = tools.FindImplementations(ctx, s.client, tools.FindImplementationsArgs{
			InterfaceName: interfaceName,
			PathPattern:   pathPattern,
			Limit:         limit,
		})

	case "cie_trace_path":
		target, _ := params.Arguments["target"].(string)
		source, _ := params.Arguments["source"].(string)
		pathPattern, _ := params.Arguments["path_pattern"].(string)
		maxPaths, _ := getIntArg(params.Arguments, "max_paths", 3)
		maxDepth, _ := getIntArg(params.Arguments, "max_depth", 5)

		result, err = tools.TracePath(ctx, s.client, tools.TracePathArgs{
			Target:      target,
			Source:      source,
			PathPattern: pathPattern,
			MaxPaths:    maxPaths,
			MaxDepth:    maxDepth,
		})

	default:
		return &mcpToolResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}, nil
	}

	if err != nil {
		return s.formatError(params.Name, err), nil
	}

	return &mcpToolResult{
		Content: []mcpContent{{Type: "text", Text: result.Text}},
		IsError: result.IsError,
	}, nil
}

// formatError creates an actionable error message based on the error type and tool
func (s *mcpServer) formatError(toolName string, err error) *mcpToolResult {
	errStr := err.Error()
	var msg string
	var suggestFallback bool

	// Check for common error patterns
	switch {
	case tools.ContainsStr(errStr, "connection refused"):
		msg = fmt.Sprintf("**Connection Error:** Cannot connect to Edge Cache at `%s`\n\n", s.client.BaseURL)
		msg += "### Possible causes:\n"
		msg += "1. Edge Cache server is not running\n"
		msg += "2. The URL in `.cie/project.yaml` is incorrect\n"
		msg += "3. Network/firewall is blocking the connection\n\n"
		msg += "### How to fix:\n"
		msg += fmt.Sprintf("- Check if Edge Cache is running: `curl %s/health`\n", s.client.BaseURL)
		msg += "- Start Edge Cache: `cie edge-cache`\n"
		msg += "- Verify the `edge_cache` URL in `.cie/project.yaml`\n"
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
		msg += fmt.Sprintf("- Check that project_id is `%s` in `.cie/project.yaml`\n", s.client.ProjectID)
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
