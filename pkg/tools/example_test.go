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

package tools_test

import (
	"fmt"

	"github.com/kraklabs/cie/pkg/tools"
)

// ExampleSemanticSearch demonstrates how to search for code by meaning using
// semantic embeddings. This is the most powerful search feature for finding
// code that matches a natural language description.
func ExampleSemanticSearch() {
	// Configure semantic search with natural language query
	args := tools.SemanticSearchArgs{
		Query: "authentication handler",
		Limit: 5,
		Role:  "source", // Exclude tests and generated code
	}

	fmt.Printf("Searching for: %s\n", args.Query)
	fmt.Printf("Role filter: %s\n", args.Role)
	fmt.Printf("Max results: %d\n", args.Limit)

	// Output:
	// Searching for: authentication handler
	// Role filter: source
	// Max results: 5
}

// ExampleGrep demonstrates ultra-fast literal text search across the codebase.
// Use this for finding exact text patterns like route definitions or function calls.
func ExampleGrep() {
	// Search for HTTP GET route definitions
	args := tools.GrepArgs{
		Text:          ".GET(",
		Path:          "internal/http",
		CaseSensitive: false,
		Limit:         20,
	}

	fmt.Printf("Searching for: %s\n", args.Text)
	fmt.Printf("In path: %s\n", args.Path)
	fmt.Printf("Case sensitive: %v\n", args.CaseSensitive)

	// Output:
	// Searching for: .GET(
	// In path: internal/http
	// Case sensitive: false
}

// ExampleFindFunction demonstrates how to locate a function by name.
// Handles Go receiver syntax automatically (e.g., searching 'Batch' finds 'Batcher.Batch').
func ExampleFindFunction() {
	args := tools.FindFunctionArgs{
		Name:        "BuildRouter",
		ExactMatch:  false,
		IncludeCode: true,
	}

	fmt.Printf("Finding function: %s\n", args.Name)
	fmt.Printf("Exact match: %v\n", args.ExactMatch)
	fmt.Printf("Include code: %v\n", args.IncludeCode)

	// Output:
	// Finding function: BuildRouter
	// Exact match: false
	// Include code: true
}

// ExampleFindCallers demonstrates how to find all functions that call a given function.
// This is useful for understanding code dependencies and impact analysis.
func ExampleFindCallers() {
	args := tools.FindCallersArgs{
		FunctionName:    "handleAuth",
		IncludeIndirect: false,
	}

	fmt.Printf("Finding callers of: %s\n", args.FunctionName)
	fmt.Printf("Include indirect: %v\n", args.IncludeIndirect)

	// Output:
	// Finding callers of: handleAuth
	// Include indirect: false
}

// ExampleTracePath demonstrates how to trace the call path from entry points
// (like main) to a target function. This helps understand how execution flows
// through the codebase.
func ExampleTracePath() {
	args := tools.TracePathArgs{
		Target:   "RegisterRoutes",
		Source:   "main", // Auto-detects entry points if empty
		MaxDepth: 10,     // Maximum call chain depth
		MaxPaths: 3,      // Return top 3 shortest paths
	}

	fmt.Printf("Tracing from: %s\n", args.Source)
	fmt.Printf("To target: %s\n", args.Target)
	fmt.Printf("Max depth: %d\n", args.MaxDepth)

	// Output:
	// Tracing from: main
	// To target: RegisterRoutes
	// Max depth: 10
}

// ExampleAnalyze demonstrates how to ask architectural questions about the codebase
// using LLM-powered analysis. The tool combines semantic search with narrative generation.
func ExampleAnalyze() {
	args := tools.AnalyzeArgs{
		Question:    "What are the main entry points?",
		PathPattern: "cmd/",
		Role:        "source",
	}

	fmt.Printf("Question: %s\n", args.Question)
	fmt.Printf("Path pattern: %s\n", args.PathPattern)
	fmt.Printf("Role filter: %s\n", args.Role)

	// Output:
	// Question: What are the main entry points?
	// Path pattern: cmd/
	// Role filter: source
}

// ExampleListEndpoints demonstrates how to discover HTTP/REST endpoints defined
// in the codebase. Works with common Go frameworks (Gin, Echo, Chi, Fiber).
func ExampleListEndpoints() {
	args := tools.ListEndpointsArgs{
		PathPattern: "internal/http",
		Method:      "GET",
		Limit:       50,
	}

	fmt.Printf("Listing %s endpoints\n", args.Method)
	fmt.Printf("In path: %s\n", args.PathPattern)
	fmt.Printf("Limit: %d\n", args.Limit)

	// Output:
	// Listing GET endpoints
	// In path: internal/http
	// Limit: 50
}

// ExampleVerifyAbsence demonstrates how to verify that sensitive patterns
// do not exist in code. This is useful for security audits to ensure no
// hardcoded secrets, API keys, or passwords are present.
func ExampleVerifyAbsence() {
	// Security audit: check frontend code for sensitive data
	args := tools.VerifyAbsenceArgs{
		Patterns: []string{"api_key", "secret", "password", "access_token"},
		Path:     "ui/src",
	}

	fmt.Printf("Verifying absence of %d patterns\n", len(args.Patterns))
	fmt.Printf("In path: %s\n", args.Path)
	fmt.Printf("Patterns: %v\n", args.Patterns[:2]) // Show first 2 for example

	// Output:
	// Verifying absence of 4 patterns
	// In path: ui/src
	// Patterns: [api_key secret]
}
