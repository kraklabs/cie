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

package tools

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// HTTP method patterns for common Go web frameworks
// All patterns require the path to start with "/" to avoid false positives
var httpMethodPatterns = []struct {
	pattern     *regexp.Regexp
	methodIndex int // capture group for method
	pathIndex   int // capture group for path
}{
	// Gin/Echo style: r.GET("/path", handler), e.POST("/path", handler)
	{regexp.MustCompile(`\.(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|Any)\s*\(\s*["'](/[^"']*)["']`), 1, 2},
	// Chi style: r.Get("/path", handler) - lowercase methods
	{regexp.MustCompile(`\.(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*["'](/[^"']*)["']`), 1, 2},
	// Fiber style: app.Get("/path", handler)
	{regexp.MustCompile(`app\.(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*["'](/[^"']*)["']`), 1, 2},
	// http.HandleFunc style: http.HandleFunc("/path", handler)
	{regexp.MustCompile(`HandleFunc\s*\(\s*["'](/[^"']*)["']`), -1, 1}, // no method in pattern
	// mux.Handle style with method in path or separate
	{regexp.MustCompile(`Handle\s*\(\s*["'](/[^"']*)["']`), -1, 1},
	// Group definitions: r.Group("/api")
	{regexp.MustCompile(`\.Group\s*\(\s*["'](/[^"']*)["']`), -1, 1},
}

// ListEndpointsArgs holds arguments for listing HTTP endpoints
type ListEndpointsArgs struct {
	PathPattern string // Filter by file path (e.g., "apps/gateway")
	PathFilter  string // Filter by endpoint path (e.g., "/health", "connections")
	Method      string // Filter by HTTP method (e.g., "GET", "POST")
	Limit       int
}

// ListEndpoints lists HTTP endpoints found in the codebase
// Uses a combined regex pattern to find all HTTP route definitions.
func ListEndpoints(ctx context.Context, client Querier, args ListEndpointsArgs) (*ToolResult, error) {
	type endpoint struct {
		Method   string
		Path     string
		Handler  string
		FilePath string
		Line     string
	}
	var endpoints []endpoint

	// Pattern that matches HTTP method calls across common frameworks:
	// - Gin/Echo: .GET(, .POST(, etc.
	// - Chi: .Get(, .Post(, etc.
	// - net/http: HandleFunc(, Handle(
	// Uses raw string notation ___"..."___ for CozoDB compatibility
	httpMethodPattern := `([.](GET|POST|PUT|DELETE|PATCH|Get|Post|Put|Delete|Patch|Group|Any)[(]|Handle(Func)?[(])`

	// Build conditions for path filter and test exclusion
	// Aggressively filter test files using [.] instead of \. for CozoDB raw strings
	var conditions []string
	conditions = append(conditions, fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(httpMethodPattern)))
	if args.PathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(args.PathPattern)))
	}
	conditions = append(conditions, `!regex_matches(file_path, ___"(_test[.]go|/tests?/|_test/|/test_)"___)`)

	conditionStr := conditions[0]
	for i := 1; i < len(conditions); i++ {
		conditionStr += ", " + conditions[i]
	}

	// Query with generous limit since we'll parse and filter
	queryLimit := args.Limit * 3
	if queryLimit > 500 {
		queryLimit = 500
	}

	// Schema v3: Join with cie_function_code to get code_text
	script := fmt.Sprintf(
		"?[file_path, name, start_line, code_text] := *cie_function { id, file_path, name, start_line }, *cie_function_code { function_id: id, code_text }, %s :limit %d",
		conditionStr, queryLimit,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, err
	}

	// Parse endpoints from matching functions
	for _, row := range result.Rows {
		filePath := AnyToString(row[0])
		funcName := AnyToString(row[1])
		startLine := AnyToString(row[2])
		codeText := AnyToString(row[3])

		// Try each HTTP pattern to extract endpoints
		for _, p := range httpMethodPatterns {
			matches := p.pattern.FindAllStringSubmatch(codeText, -1)
			for _, match := range matches {
				var httpMethod, httpPath string

				if p.methodIndex > 0 && p.methodIndex < len(match) {
					httpMethod = strings.ToUpper(match[p.methodIndex])
				} else {
					httpMethod = "ANY"
				}

				if p.pathIndex > 0 && p.pathIndex < len(match) {
					httpPath = match[p.pathIndex]
				}

				if httpPath == "" {
					continue
				}

				// Apply method filter if specified
				if args.Method != "" && httpMethod != strings.ToUpper(args.Method) && httpMethod != "ANY" {
					continue
				}

				// Apply endpoint path filter if specified
				if args.PathFilter != "" && !strings.Contains(strings.ToLower(httpPath), strings.ToLower(args.PathFilter)) {
					continue
				}

				endpoints = append(endpoints, endpoint{
					Method:   httpMethod,
					Path:     httpPath,
					Handler:  funcName,
					FilePath: filePath,
					Line:     startLine,
				})
			}
		}
	}

	// Deduplicate endpoints
	seen := make(map[string]bool)
	var uniqueEndpoints []endpoint
	for _, ep := range endpoints {
		key := ep.Method + "|" + ep.Path + "|" + ep.FilePath
		if !seen[key] {
			seen[key] = true
			uniqueEndpoints = append(uniqueEndpoints, ep)
		}
	}
	endpoints = uniqueEndpoints

	if len(endpoints) == 0 {
		output := "No HTTP endpoints found.\n\n"
		output += "**Tips:**\n"
		output += "- Check if the codebase uses Go web frameworks (Gin, Echo, Chi, Fiber)\n"
		output += "- Try a different `path_pattern` to narrow the search\n"
		output += "- Use `cie_grep` with patterns like `.GET(` or `.POST(` for manual search\n"
		return NewResult(output), nil
	}

	// Limit results and track if truncated
	truncated := len(endpoints) > args.Limit
	totalFound := len(endpoints)
	if truncated {
		endpoints = endpoints[:args.Limit]
	}

	// Format output as a table
	output := fmt.Sprintf("## HTTP Endpoints (%d found)\n\n", len(endpoints))
	if args.PathPattern != "" {
		output = fmt.Sprintf("## HTTP Endpoints in `%s` (%d found)\n\n", args.PathPattern, len(endpoints))
	}
	if args.PathFilter != "" {
		output = fmt.Sprintf("## HTTP Endpoints matching `%s` (%d found)\n\n", args.PathFilter, len(endpoints))
	}

	output += "| Method | Path | Handler | File |\n"
	output += "|--------|------|---------|------|\n"

	for _, ep := range endpoints {
		fileName := ExtractFileName(ep.FilePath)
		output += fmt.Sprintf("| %s | `%s` | %s | %s:%s |\n",
			ep.Method, ep.Path, ep.Handler, fileName, ep.Line)
	}

	// Summary section
	output += "\n### Summary\n\n"

	// Group by HTTP method
	methodCounts := make(map[string]int)
	for _, ep := range endpoints {
		methodCounts[ep.Method]++
	}
	output += "**By Method:**\n"
	methodOrder := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "ANY"}
	for _, m := range methodOrder {
		if count, ok := methodCounts[m]; ok {
			output += fmt.Sprintf("- %s: %d\n", m, count)
		}
	}
	output += "\n"

	// Group by path prefix for API structure
	prefixCounts := make(map[string]int)
	for _, ep := range endpoints {
		prefix := extractPathPrefix(ep.Path)
		prefixCounts[prefix]++
	}

	if len(prefixCounts) > 1 {
		output += "**By API Path:**\n"
		// Sort prefixes for consistent output
		var prefixes []string
		for prefix := range prefixCounts {
			prefixes = append(prefixes, prefix)
		}
		sort.Strings(prefixes)
		for _, prefix := range prefixes {
			output += fmt.Sprintf("- `%s` (%d endpoints)\n", prefix, prefixCounts[prefix])
		}
		output += "\n"
	}

	// Group by file/module
	fileCounts := make(map[string]int)
	for _, ep := range endpoints {
		fileName := ExtractFileName(ep.FilePath)
		fileCounts[fileName]++
	}
	if len(fileCounts) > 1 {
		output += "**By File:**\n"
		var files []string
		for f := range fileCounts {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			output += fmt.Sprintf("- %s: %d\n", f, fileCounts[f])
		}
		output += "\n"
	}

	// Add truncation warning if we hit the limit
	if truncated {
		output += fmt.Sprintf("⚠️ **Warning:** Results truncated. Found %d endpoints but showing only %d (limit). Use `limit=%d` or higher to see all results.\n", totalFound, args.Limit, totalFound)
	}

	return NewResult(output), nil
}

// extractPathPrefix extracts the first 2 path segments for grouping (e.g., /v1/users -> /v1/users)
func extractPathPrefix(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 {
		return path
	}
	// Take up to 2 segments
	if len(parts) == 1 {
		return "/" + parts[0]
	}
	return "/" + parts[0] + "/" + parts[1]
}
