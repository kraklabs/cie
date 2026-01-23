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
	if args.Limit <= 0 {
		args.Limit = 100
	}

	// Query functions that contain HTTP method patterns
	conditionStr := buildEndpointQueryConditions(args)
	queryLimit := args.Limit * 3
	if queryLimit > 500 {
		queryLimit = 500
	}

	script := fmt.Sprintf(
		"?[file_path, name, start_line, code_text] := *cie_function { id, file_path, name, start_line }, *cie_function_code { function_id: id, code_text }, %s :limit %d",
		conditionStr, queryLimit,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query endpoints: %w", err)
	}

	// Parse endpoints from matching functions
	var endpoints []endpoint
	for _, row := range result.Rows {
		filePath := AnyToString(row[0])
		funcName := AnyToString(row[1])
		startLine := AnyToString(row[2])
		codeText := AnyToString(row[3])
		endpoints = append(endpoints, parseEndpointsFromCode(codeText, filePath, funcName, startLine, args)...)
	}

	// Deduplicate and check for empty results
	endpoints = deduplicateEndpoints(endpoints)
	if len(endpoints) == 0 {
		return NewResult(formatNoEndpointsFound()), nil
	}

	// Limit results
	totalFound := len(endpoints)
	truncated := totalFound > args.Limit
	if truncated {
		endpoints = endpoints[:args.Limit]
	}

	// Format output
	output := formatEndpointHeader(args, len(endpoints))
	output += formatEndpointTable(endpoints)
	output += "\n" + formatEndpointSummary(endpoints)
	if truncated {
		output += fmt.Sprintf("⚠️ **Warning:** Results truncated. Found %d endpoints but showing only %d (limit). Use `limit=%d` or higher to see all results.\n", totalFound, args.Limit, totalFound)
	}

	return NewResult(output), nil
}

// formatNoEndpointsFound returns the message when no endpoints are found.
func formatNoEndpointsFound() string {
	return "No HTTP endpoints found.\n\n" +
		"**Tips:**\n" +
		"- Check if the codebase uses Go web frameworks (Gin, Echo, Chi, Fiber)\n" +
		"- Try a different `path_pattern` to narrow the search\n" +
		"- Use `cie_grep` with patterns like `.GET(` or `.POST(` for manual search\n"
}

// formatEndpointHeader generates the header for endpoint output.
func formatEndpointHeader(args ListEndpointsArgs, count int) string {
	if args.PathFilter != "" {
		return fmt.Sprintf("## HTTP Endpoints matching `%s` (%d found)\n\n", args.PathFilter, count)
	}
	if args.PathPattern != "" {
		return fmt.Sprintf("## HTTP Endpoints in `%s` (%d found)\n\n", args.PathPattern, count)
	}
	return fmt.Sprintf("## HTTP Endpoints (%d found)\n\n", count)
}

// formatEndpointTable generates the table of endpoints.
func formatEndpointTable(endpoints []endpoint) string {
	var sb strings.Builder
	sb.WriteString("| Method | Path | Handler | File |\n")
	sb.WriteString("|--------|------|---------|------|\n")
	for _, ep := range endpoints {
		fileName := ExtractFileName(ep.FilePath)
		fmt.Fprintf(&sb, "| %s | `%s` | %s | %s:%s |\n", ep.Method, ep.Path, ep.Handler, fileName, ep.Line)
	}
	return sb.String()
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

// endpoint holds parsed endpoint information.
type endpoint struct {
	Method   string
	Path     string
	Handler  string
	FilePath string
	Line     string
}

// buildEndpointQueryConditions builds query conditions for endpoint search.
func buildEndpointQueryConditions(args ListEndpointsArgs) string {
	httpMethodPattern := `([.](GET|POST|PUT|DELETE|PATCH|Get|Post|Put|Delete|Patch|Group|Any)[(]|Handle(Func)?[(])`
	var conditions []string
	conditions = append(conditions, fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(httpMethodPattern)))
	if args.PathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(args.PathPattern)))
	}
	conditions = append(conditions, `!regex_matches(file_path, ___"(_test[.]go|/tests?/|_test/|/test_)"___)`)
	return strings.Join(conditions, ", ")
}

// parseEndpointsFromCode extracts endpoints from function code using HTTP patterns.
func parseEndpointsFromCode(codeText, filePath, funcName, startLine string, args ListEndpointsArgs) []endpoint {
	var endpoints []endpoint
	for _, p := range httpMethodPatterns {
		matches := p.pattern.FindAllStringSubmatch(codeText, -1)
		for _, match := range matches {
			ep := extractEndpointFromMatch(match, p.methodIndex, p.pathIndex, filePath, funcName, startLine)
			if ep == nil {
				continue
			}
			if !endpointMatchesFilters(ep, args) {
				continue
			}
			endpoints = append(endpoints, *ep)
		}
	}
	return endpoints
}

// extractEndpointFromMatch extracts endpoint info from a regex match.
func extractEndpointFromMatch(match []string, methodIndex, pathIndex int, filePath, funcName, startLine string) *endpoint {
	var httpMethod, httpPath string
	if methodIndex > 0 && methodIndex < len(match) {
		httpMethod = strings.ToUpper(match[methodIndex])
	} else {
		httpMethod = "ANY"
	}
	if pathIndex > 0 && pathIndex < len(match) {
		httpPath = match[pathIndex]
	}
	if httpPath == "" {
		return nil
	}
	return &endpoint{
		Method:   httpMethod,
		Path:     httpPath,
		Handler:  funcName,
		FilePath: filePath,
		Line:     startLine,
	}
}

// endpointMatchesFilters checks if an endpoint matches the given filters.
func endpointMatchesFilters(ep *endpoint, args ListEndpointsArgs) bool {
	if args.Method != "" && ep.Method != strings.ToUpper(args.Method) && ep.Method != "ANY" {
		return false
	}
	if args.PathFilter != "" && !strings.Contains(strings.ToLower(ep.Path), strings.ToLower(args.PathFilter)) {
		return false
	}
	return true
}

// deduplicateEndpoints removes duplicate endpoints.
func deduplicateEndpoints(endpoints []endpoint) []endpoint {
	seen := make(map[string]bool)
	var unique []endpoint
	for _, ep := range endpoints {
		key := ep.Method + "|" + ep.Path + "|" + ep.FilePath
		if !seen[key] {
			seen[key] = true
			unique = append(unique, ep)
		}
	}
	return unique
}

// formatEndpointSummary generates the summary section of endpoint output.
func formatEndpointSummary(endpoints []endpoint) string {
	var sb strings.Builder
	sb.WriteString("### Summary\n\n")

	// Group by HTTP method
	methodCounts := make(map[string]int)
	for _, ep := range endpoints {
		methodCounts[ep.Method]++
	}
	sb.WriteString("**By Method:**\n")
	methodOrder := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "ANY"}
	for _, m := range methodOrder {
		if count, ok := methodCounts[m]; ok {
			fmt.Fprintf(&sb, "- %s: %d\n", m, count)
		}
	}
	sb.WriteString("\n")

	// Group by path prefix
	prefixCounts := make(map[string]int)
	for _, ep := range endpoints {
		prefix := extractPathPrefix(ep.Path)
		prefixCounts[prefix]++
	}
	if len(prefixCounts) > 1 {
		sb.WriteString("**By API Path:**\n")
		var prefixes []string
		for prefix := range prefixCounts {
			prefixes = append(prefixes, prefix)
		}
		sort.Strings(prefixes)
		for _, prefix := range prefixes {
			fmt.Fprintf(&sb, "- `%s` (%d endpoints)\n", prefix, prefixCounts[prefix])
		}
		sb.WriteString("\n")
	}

	// Group by file
	fileCounts := make(map[string]int)
	for _, ep := range endpoints {
		fileName := ExtractFileName(ep.FilePath)
		fileCounts[fileName]++
	}
	if len(fileCounts) > 1 {
		sb.WriteString("**By File:**\n")
		var files []string
		for f := range fileCounts {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(&sb, "- %s: %d\n", f, fileCounts[f])
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
