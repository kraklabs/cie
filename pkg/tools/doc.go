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

// Package tools provides MCP tools for code intelligence queries against CIE.
//
// CIE (Code Intelligence Engine) indexes codebases and provides semantic
// understanding through the Model Context Protocol (MCP). This package
// implements the tool functions that can be called via MCP to query indexed
// code, navigate call graphs, analyze architecture, and discover patterns.
//
// # Quick Start
//
// Create a client connected to the CIE Edge Cache and execute tools:
//
//	client := &tools.CIEClient{
//		BaseURL:        "http://localhost:3420",
//		ProjectID:      "myproject",
//		HTTPClient:     &http.Client{Timeout: 30 * time.Second},
//		EmbeddingURL:   "http://localhost:11434",
//		EmbeddingModel: "nomic-embed-text",
//	}
//
//	// Semantic search for code by meaning
//	result, err := tools.SemanticSearch(ctx, client, tools.SemanticSearchArgs{
//		Query: "authentication handler",
//		Limit: 10,
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(result.Text)
//
// # Available Tool Categories
//
// The package provides tools organized into five categories:
//
// Search Tools:
//   - SemanticSearch: Find code by meaning using embeddings
//   - Grep: Ultra-fast literal text search with context
//   - SearchText: Regex-based pattern search in function code/signatures
//
// Navigation Tools:
//   - FindFunction: Find functions by name
//   - FindCallers: Find functions that call a given function
//   - FindCallees: Find functions called by a given function
//   - GetFunctionCode: Get the full source code of a function
//   - GetTypeCode: Get the source code of a type/class/interface
//   - FindType: Find type definitions by name
//   - FindImplementations: Find types implementing an interface
//   - ListFunctionsInFile: List all functions defined in a file
//   - FindSimilarFunctions: Find functions with similar names
//
// Analysis Tools:
//   - Analyze: Answer architectural questions using semantic search
//   - TracePath: Trace call paths from entry points to target function
//   - GetCallGraph: Get complete call graph for a function
//   - DirectorySummary: Summarize files in a directory with key functions
//   - GetFileSummary: Summarize all entities defined in a file
//
// Discovery Tools:
//   - ListEndpoints: List HTTP/REST endpoints from route definitions
//   - ListServices: List gRPC services and RPC methods from .proto files
//   - ListFiles: List indexed files with filtering options
//
// Utility Tools:
//   - GetSchema: Get CIE database schema information
//   - IndexStatus: Check indexing status and health
//   - VerifyAbsence: Verify patterns don't exist (security audits)
//   - RawQuery: Execute raw CozoScript queries
//
// # Error Handling
//
// All tool functions return a ToolResult struct that includes both the
// result data and any error information. Check both the error return value
// and the ToolResult.IsError field:
//
//	result, err := tools.SemanticSearch(ctx, client, args)
//	if err != nil {
//		// Handle execution error (network, timeout, etc.)
//		log.Printf("tool execution failed: %v", err)
//		return err
//	}
//	if result.IsError {
//		// Handle tool-level error (e.g., invalid query, missing data)
//		log.Printf("tool returned error: %s", result.Text)
//		return fmt.Errorf("semantic search failed: %s", result.Text)
//	}
//	// Success - result.Text contains the tool output
//	fmt.Println(result.Text)
//
// # Configuration
//
// Tools are configured through the CIEClient struct. All fields are optional
// except BaseURL and ProjectID:
//
//	client := &tools.CIEClient{
//		BaseURL:        "http://localhost:3420",     // Required: CIE Edge Cache URL
//		ProjectID:      "myproject",                 // Required: Project identifier
//		HTTPClient:     &http.Client{Timeout: 30 * time.Second}, // Optional: custom HTTP client
//		EmbeddingURL:   "http://localhost:11434",    // Optional: Ollama URL for embeddings
//		EmbeddingModel: "nomic-embed-text",          // Optional: embedding model name
//	}
//
// For testing, use TestCIEClient which implements the Querier interface
// with an embedded CozoDB instance instead of HTTP calls.
//
// # Architecture
//
// This package uses CozoDB with CozoScript queries to access indexed code.
// The database schema (v3) separates metadata from code content:
//   - cie_function: function metadata (name, signature, file, lines)
//   - cie_function_code: function source code (indexed separately)
//   - cie_function_embedding: function embeddings for semantic search
//   - cie_type_code: type/class/interface source code
//
// All tools accept a Querier interface, allowing them to work with either
// an HTTP client (CIEClient) or an embedded database (TestCIEClient).
//
// # Supported Languages
//
// The tools support code indexed from these languages:
//   - Go (functions, methods, types, interfaces)
//   - Python (functions, classes, methods)
//   - TypeScript/JavaScript (functions, classes, arrow functions)
//   - Protobuf (services, RPC methods, messages)
//
// # Role-Based Filtering
//
// Many tools support role-based filtering to distinguish between different
// types of code files:
//   - "source": Regular source code (excludes tests and generated files)
//   - "test": Test files only
//   - "generated": Generated code files
//   - "entry_point": Main functions and entry points
//   - "router": Route definition functions
//   - "handler": HTTP request handler functions
//   - "any": No filtering (includes all files)
//
// Example using role filtering:
//
//	// Search only in source code (exclude tests)
//	result, err := tools.SemanticSearch(ctx, client, tools.SemanticSearchArgs{
//		Query: "authentication",
//		Role:  "source",
//		Limit: 10,
//	})
package tools
