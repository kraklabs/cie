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

// ListEndpointsArgs holds arguments for listing HTTP/REST endpoints.
type ListEndpointsArgs struct {
	// PathPattern filters results by file path using regex.
	// Example: "apps/gateway" matches files in the gateway app directory.
	PathPattern string

	// PathFilter filters results by endpoint path using substring match.
	// Example: "/health" matches "/api/v1/health", "connections" matches "/api/connections".
	PathFilter string

	// Method filters results by HTTP method (case-insensitive).
	// Examples: "GET", "POST", "PUT", "DELETE", "PATCH".
	// Leave empty to match all methods.
	Method string

	// Limit is the maximum number of endpoints to return.
	// Defaults to 100 if zero or negative.
	Limit int
}

// ListEndpoints lists HTTP/REST endpoints defined in the codebase.
//
// It detects route definitions from multiple popular Go web frameworks:
//   - Gin/Echo: r.GET("/path", handler), e.POST("/path", handler)
//   - Chi: r.Get("/path", handler), r.Post("/path", handler)
//   - Fiber: app.Get("/path", handler)
//   - net/http: http.HandleFunc("/path", handler)
//   - Generic: mux.Handle("/path", handler), r.Group("/api")
//
// The function searches for HTTP method patterns in function code and extracts
// the endpoint path, method, and handler information.
//
// Results can be filtered by file path (PathPattern), endpoint path (PathFilter),
// or HTTP method (Method). Test files are automatically excluded from results.
//
// Returns a ToolResult containing a formatted table of endpoints with columns:
// [Method] [Path] [Handler] [File:Line]
//
// Returns an error if the query execution fails.
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
