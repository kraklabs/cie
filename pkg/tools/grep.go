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
	"strings"
)

type GrepArgs struct {
	Text           string   // Single pattern (for backward compatibility)
	Texts          []string // Multiple patterns to search in parallel
	Path           string
	ExcludePattern string
	CaseSensitive  bool
	ContextLines   int
	Limit          int
}

// GrepMultiResult holds results grouped by pattern
type GrepMultiResult struct {
	Pattern string
	Count   int
	Matches []GrepMatch
}

// GrepMatch represents a single match
type GrepMatch struct {
	FilePath  string
	Name      string
	StartLine string
	Context   string
}

// Grep performs ultra-fast literal text search with optional context
// Schema v3: code_text is in separate cie_function_code table
// Supports multiple patterns via 'texts' parameter for batch searches
func Grep(ctx context.Context, client Querier, args GrepArgs) (*ToolResult, error) {
	// Handle multi-pattern search
	if len(args.Texts) > 0 {
		return grepMulti(ctx, client, args)
	}

	if args.Text == "" {
		return NewError("Error: 'text' or 'texts' is required"), nil
	}

	// Escape for regex but make it literal
	escapedText := EscapeRegex(args.Text)

	// Build case-insensitive pattern if needed
	pattern := escapedText
	if !args.CaseSensitive {
		pattern = "(?i)" + pattern
	}

	// Build conditions - include code_text if we need context
	needsCode := args.ContextLines > 0
	selectFields := "file_path, name, start_line, end_line"
	if needsCode {
		selectFields = "file_path, name, start_line, end_line, code_text"
	}

	// Build conditions using single-quoted patterns to avoid double-quote escaping issues
	conditions := []string{fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(pattern))}

	// Add path filter if provided (simple substring match)
	if args.Path != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(EscapeRegex(args.Path))))
	}

	// Add exclude pattern if provided (negate the match)
	if args.ExcludePattern != "" {
		conditions = append(conditions, fmt.Sprintf("!regex_matches(file_path, %s)", QuoteCozoPattern(args.ExcludePattern)))
	}

	conditionStr := conditions[0]
	for i := 1; i < len(conditions); i++ {
		conditionStr += ", " + conditions[i]
	}

	// Schema v3: Join with cie_function_code to get code_text
	script := fmt.Sprintf(
		"?[%s] := *cie_function { id, file_path, name, start_line, end_line }, *cie_function_code { function_id: id, code_text }, %s :limit %d",
		selectFields, conditionStr, args.Limit,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		output := fmt.Sprintf("No matches found for: `%s`\n", args.Text)
		if args.Path != "" {
			output += fmt.Sprintf("In path: `%s`\n", args.Path)
		}
		if args.ExcludePattern != "" {
			output += fmt.Sprintf("Excluding: `%s`\n", args.ExcludePattern)
		}

		// If searching in a specific path with no results, check if term exists elsewhere
		if args.Path != "" {
			altPaths := findAlternativePaths(ctx, client, args.Text, args.CaseSensitive)
			if len(altPaths) > 0 {
				output += "\n💡 **Found in other locations:**\n"
				for _, ap := range altPaths {
					output += fmt.Sprintf("- `%s` (%d matches)\n", ap.Path, ap.Count)
				}
				output += "\n"
			}
		}

		// Check for route parameter syntax alternatives
		routeSuggestions := suggestRouteAlternatives(ctx, client, args.Text, args.Path, args.CaseSensitive)
		if routeSuggestions != "" {
			output += routeSuggestions
		}

		output += "\n**Tips:**\n"
		output += "- Check spelling and case (default is case-insensitive)\n"
		output += "- Try a shorter/simpler pattern\n"
		output += "- Use `cie_list_files` to verify the path exists\n"
		return NewResult(output), nil
	}

	output := fmt.Sprintf("Found %d matches for `%s`", len(result.Rows), args.Text)
	if args.Path != "" {
		output += fmt.Sprintf(" in `%s`", args.Path)
	}
	if args.ExcludePattern != "" {
		output += fmt.Sprintf(" (excluding `%s`)", args.ExcludePattern)
	}
	output += ":\n\n"

	for i, row := range result.Rows {
		filePath := AnyToString(row[0])
		name := AnyToString(row[1])
		startLine := AnyToString(row[2])

		output += fmt.Sprintf("%d. **%s** in `%s:%s`\n", i+1, name, filePath, startLine)

		// Show context if requested
		if needsCode && len(row) > 4 {
			codeText := AnyToString(row[4])
			matchContext := extractMatchContext(codeText, args.Text, args.CaseSensitive, args.ContextLines)
			if matchContext != "" {
				output += "```\n" + matchContext + "```\n"
			}
		}
		output += "\n"
	}

	return NewResult(output), nil
}

// extractMatchContext finds matching lines and returns them with context
func extractMatchContext(code, searchText string, caseSensitive bool, contextLines int) string {
	lines := splitLines(code)
	var matchingLineNums []int

	// Find lines containing the search text
	searchLower := searchText
	if !caseSensitive {
		searchLower = ToLower(searchText)
	}

	for i, line := range lines {
		lineToCheck := line
		if !caseSensitive {
			lineToCheck = ToLower(line)
		}
		if ContainsStr(lineToCheck, searchLower) {
			matchingLineNums = append(matchingLineNums, i)
		}
	}

	if len(matchingLineNums) == 0 {
		return ""
	}

	// Build output with context, avoiding duplicates
	var result string
	shown := make(map[int]bool)
	lastShown := -1

	for _, matchLine := range matchingLineNums {
		start := matchLine - contextLines
		if start < 0 {
			start = 0
		}
		end := matchLine + contextLines
		if end >= len(lines) {
			end = len(lines) - 1
		}

		// Add separator if there's a gap
		if lastShown >= 0 && start > lastShown+1 {
			result += "  ...\n"
		}

		for j := start; j <= end; j++ {
			if shown[j] {
				continue
			}
			shown[j] = true
			lastShown = j

			prefix := "  "
			if j == matchLine {
				prefix = "> " // Highlight matching line
			}
			// Show line number relative to function start
			result += fmt.Sprintf("%s%3d: %s\n", prefix, j+1, lines[j])
		}
	}

	// Limit output to avoid huge responses
	if len(result) > 2000 {
		result = result[:2000] + "\n  ... (truncated)\n"
	}

	return result
}

// splitLines splits text into lines
func splitLines(s string) []string {
	var lines []string
	var current string
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// grepMulti searches for multiple patterns and returns grouped results
func grepMulti(ctx context.Context, client Querier, args GrepArgs) (*ToolResult, error) {
	if len(args.Texts) == 0 {
		return NewError("Error: 'texts' array is empty"), nil
	}

	// Build OR pattern: (pattern1|pattern2|pattern3)
	var escapedPatterns []string
	for _, text := range args.Texts {
		escapedPatterns = append(escapedPatterns, EscapeRegex(text))
	}

	combinedPattern := "(" + strings.Join(escapedPatterns, "|") + ")"
	if !args.CaseSensitive {
		combinedPattern = "(?i)" + combinedPattern
	}

	// Build conditions
	conditions := []string{fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(combinedPattern))}

	if args.Path != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(EscapeRegex(args.Path))))
	}

	if args.ExcludePattern != "" {
		conditions = append(conditions, fmt.Sprintf("!regex_matches(file_path, %s)", QuoteCozoPattern(args.ExcludePattern)))
	}

	conditionStr := strings.Join(conditions, ", ")

	// Query with code_text to count per-pattern matches
	script := fmt.Sprintf(
		"?[file_path, name, start_line, code_text] := *cie_function { id, file_path, name, start_line }, *cie_function_code { function_id: id, code_text }, %s :limit %d",
		conditionStr, args.Limit*len(args.Texts), // Get more results since we're grouping
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, err
	}

	// Group results by pattern
	patternCounts := make(map[string]int)
	patternMatches := make(map[string][]GrepMatch)

	for _, row := range result.Rows {
		filePath := AnyToString(row[0])
		name := AnyToString(row[1])
		startLine := AnyToString(row[2])
		codeText := AnyToString(row[3])

		// Check which patterns match this result
		for _, text := range args.Texts {
			checkText := text
			checkCode := codeText
			if !args.CaseSensitive {
				checkText = ToLower(text)
				checkCode = ToLower(codeText)
			}

			if ContainsStr(checkCode, checkText) {
				patternCounts[text]++
				if len(patternMatches[text]) < args.Limit {
					patternMatches[text] = append(patternMatches[text], GrepMatch{
						FilePath:  filePath,
						Name:      name,
						StartLine: startLine,
					})
				}
			}
		}
	}

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("## Multi-pattern search (%d patterns)\n\n", len(args.Texts)))

	if args.Path != "" {
		output.WriteString(fmt.Sprintf("Path filter: `%s`\n\n", args.Path))
	}

	// Summary table
	output.WriteString("| Pattern | Matches |\n")
	output.WriteString("|---------|--------:|\n")

	totalMatches := 0
	for _, text := range args.Texts {
		count := patternCounts[text]
		totalMatches += count
		status := "✓"
		if count == 0 {
			status = "✗"
		}
		output.WriteString(fmt.Sprintf("| `%s` | %s %d |\n", text, status, count))
	}
	output.WriteString(fmt.Sprintf("| **Total** | **%d** |\n\n", totalMatches))

	// Detailed matches per pattern
	for _, text := range args.Texts {
		matches := patternMatches[text]
		if len(matches) == 0 {
			continue
		}

		output.WriteString(fmt.Sprintf("### `%s` (%d matches)\n\n", text, patternCounts[text]))

		for i, match := range matches {
			if i >= 5 { // Limit detail per pattern
				output.WriteString(fmt.Sprintf("  ... and %d more\n", len(matches)-5))
				break
			}
			output.WriteString(fmt.Sprintf("- **%s** in `%s:%s`\n", match.Name, match.FilePath, match.StartLine))
		}
		output.WriteString("\n")
	}

	return NewResult(output.String()), nil
}

// altPath represents an alternative path where a search term was found
type altPath struct {
	Path  string
	Count int
}

// findAlternativePaths searches for a term in the entire codebase and returns
// top-level directories where it was found, grouped by common path prefixes.
func findAlternativePaths(ctx context.Context, client Querier, text string, caseSensitive bool) []altPath {
	// Escape for regex but make it literal
	escapedText := EscapeRegex(text)
	pattern := escapedText
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}

	// Search globally (no path filter), limit to 100 for performance
	script := fmt.Sprintf(
		"?[file_path] := *cie_function { id, file_path }, *cie_function_code { function_id: id, code_text }, regex_matches(code_text, %s) :limit 100",
		QuoteCozoPattern(pattern),
	)

	result, err := client.Query(ctx, script)
	if err != nil || len(result.Rows) == 0 {
		return nil
	}

	// Group by top-level directory (first 2 path components)
	pathCounts := make(map[string]int)
	for _, row := range result.Rows {
		filePath := AnyToString(row[0])
		topDir := extractTopDir(filePath)
		if topDir != "" {
			pathCounts[topDir]++
		}
	}

	// Convert to slice and sort by count
	var paths []altPath
	for p, c := range pathCounts {
		paths = append(paths, altPath{Path: p, Count: c})
	}

	// Sort by count descending
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j].Count > paths[i].Count {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}

	// Limit to top 5
	if len(paths) > 5 {
		paths = paths[:5]
	}

	return paths
}

// routeParamPatterns defines common route parameter syntaxes used by different frameworks
var routeParamPatterns = []struct {
	regex   string // regex to match the pattern
	syntax  string // human-readable syntax name
	example string // example of what it looks like
}{
	{`\{[a-zA-Z_][a-zA-Z0-9_]*\}`, "OpenAPI/Swagger", "{id}"},
	{`:[a-zA-Z_][a-zA-Z0-9_]*`, "Gin/Express", ":id"},
	{`<[a-zA-Z_][a-zA-Z0-9_]*>`, "Flask/Angular", "<id>"},
	{`\[[a-zA-Z_][a-zA-Z0-9_]*\]`, "Next.js", "[id]"},
}

// suggestRouteAlternatives checks if the search term contains route parameters
// and suggests alternative syntaxes that might exist in the codebase.
func suggestRouteAlternatives(ctx context.Context, client Querier, text, path string, caseSensitive bool) string {
	// Check if the text looks like a route (contains /)
	if !strings.Contains(text, "/") {
		return ""
	}

	// Try to find route parameters in the text
	var foundSyntax string
	var paramNames []string

	for _, p := range routeParamPatterns {
		re := regexp.MustCompile(p.regex)
		matches := re.FindAllString(text, -1)
		if len(matches) > 0 {
			foundSyntax = p.syntax
			paramNames = matches
			break
		}
	}

	if foundSyntax == "" {
		return ""
	}

	// Generate alternative patterns
	var alternatives []struct {
		text   string
		syntax string
	}

	for _, param := range paramNames {
		// Extract the parameter name (strip the delimiters)
		paramName := extractParamName(param)

		for _, p := range routeParamPatterns {
			if p.syntax == foundSyntax {
				continue // Skip the original syntax
			}
			altText := text
			var altParam string
			switch p.syntax {
			case "OpenAPI/Swagger":
				altParam = "{" + paramName + "}"
			case "Gin/Express":
				altParam = ":" + paramName
			case "Flask/Angular":
				altParam = "<" + paramName + ">"
			case "Next.js":
				altParam = "[" + paramName + "]"
			}
			altText = strings.Replace(altText, param, altParam, 1)
			alternatives = append(alternatives, struct {
				text   string
				syntax string
			}{altText, p.syntax})
		}
	}

	// Try each alternative and see if it has matches
	var suggestions []string
	for _, alt := range alternatives {
		count := countMatches(ctx, client, alt.text, path, caseSensitive)
		if count > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- `%s` (%s syntax): **%d matches**", alt.text, alt.syntax, count))
		}
	}

	if len(suggestions) == 0 {
		return ""
	}

	output := "\n💡 **Route parameter syntax alternatives found:**\n"
	output += fmt.Sprintf("Your search uses %s syntax (`%s`).\n", foundSyntax, paramNames[0])
	output += "Try these alternative syntaxes:\n"
	for _, s := range suggestions {
		output += s + "\n"
	}
	return output
}

// extractParamName extracts the parameter name from various syntaxes
func extractParamName(param string) string {
	// Remove common delimiters: {}, :, <>, []
	param = strings.TrimPrefix(param, "{")
	param = strings.TrimSuffix(param, "}")
	param = strings.TrimPrefix(param, ":")
	param = strings.TrimPrefix(param, "<")
	param = strings.TrimSuffix(param, ">")
	param = strings.TrimPrefix(param, "[")
	param = strings.TrimSuffix(param, "]")
	return param
}

// countMatches quickly counts how many functions match a pattern
func countMatches(ctx context.Context, client Querier, text, path string, caseSensitive bool) int {
	escapedText := EscapeRegex(text)
	pattern := escapedText
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}

	conditions := []string{fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(pattern))}
	if path != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(EscapeRegex(path))))
	}

	script := fmt.Sprintf(
		"?[count(id)] := *cie_function { id, file_path }, *cie_function_code { function_id: id, code_text }, %s",
		strings.Join(conditions, ", "),
	)

	result, err := client.Query(ctx, script)
	if err != nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return 0
	}

	if v, ok := result.Rows[0][0].(float64); ok {
		return int(v)
	}
	return 0
}

// extractTopDir extracts the first 2 path components (e.g., "apps/gateway" from "apps/gateway/internal/http/foo.go")
func extractTopDir(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return ""
}

// VerifyAbsenceArgs holds arguments for absence verification
type VerifyAbsenceArgs struct {
	Patterns       []string // Patterns that should NOT exist
	Path           string   // Path filter
	ExcludePattern string   // Files to exclude from check
	CaseSensitive  bool
	Severity       string // "critical", "warning", "info"
}

// VerifyAbsenceResult represents the verification result
type VerifyAbsenceResult struct {
	Passed     bool                `json:"passed"`
	Violations []AbsenceViolation  `json:"violations,omitempty"`
	Summary    AbsenceCheckSummary `json:"summary"`
}

type AbsenceViolation struct {
	Pattern  string `json:"pattern"`
	FilePath string `json:"file_path"`
	Function string `json:"function"`
	Line     string `json:"line"`
	Severity string `json:"severity"`
}

type AbsenceCheckSummary struct {
	PatternsChecked int `json:"patterns_checked"`
	ViolationsFound int `json:"violations_found"`
	FilesScanned    int `json:"files_scanned"`
}

// VerifyAbsence checks that patterns do NOT exist in the codebase
// Useful for security audits (no secrets, no hardcoded tokens, etc.)
func VerifyAbsence(ctx context.Context, client Querier, args VerifyAbsenceArgs) (*ToolResult, error) {
	if len(args.Patterns) == 0 {
		return NewError("Error: 'patterns' array is required"), nil
	}

	if args.Severity == "" {
		args.Severity = "warning"
	}

	// Build OR pattern for all patterns
	var escapedPatterns []string
	for _, pattern := range args.Patterns {
		escapedPatterns = append(escapedPatterns, EscapeRegex(pattern))
	}

	combinedPattern := "(" + strings.Join(escapedPatterns, "|") + ")"
	if !args.CaseSensitive {
		combinedPattern = "(?i)" + combinedPattern
	}

	// Build conditions
	conditions := []string{fmt.Sprintf("regex_matches(code_text, %s)", QuoteCozoPattern(combinedPattern))}

	if args.Path != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %s)", QuoteCozoPattern(EscapeRegex(args.Path))))
	}

	if args.ExcludePattern != "" {
		conditions = append(conditions, fmt.Sprintf("!regex_matches(file_path, %s)", QuoteCozoPattern(args.ExcludePattern)))
	}

	conditionStr := strings.Join(conditions, ", ")

	// Query for violations
	script := fmt.Sprintf(
		"?[file_path, name, start_line, code_text] := *cie_function { id, file_path, name, start_line }, *cie_function_code { function_id: id, code_text }, %s :limit 100",
		conditionStr,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, err
	}

	// Also count total files for context
	fileCountScript := "?[count(file_path)] := *cie_file { file_path }"
	if args.Path != "" {
		fileCountScript = fmt.Sprintf(
			"?[count(file_path)] := *cie_file { file_path }, regex_matches(file_path, %s)",
			QuoteCozoPattern(EscapeRegex(args.Path)),
		)
	}
	fileCountResult, err := client.Query(ctx, fileCountScript)
	filesScanned := 0
	if err == nil && fileCountResult != nil && len(fileCountResult.Rows) > 0 && len(fileCountResult.Rows[0]) > 0 {
		if v, ok := fileCountResult.Rows[0][0].(float64); ok {
			filesScanned = int(v)
		}
	}

	// Collect violations grouped by pattern
	var violations []AbsenceViolation

	for _, row := range result.Rows {
		filePath := AnyToString(row[0])
		name := AnyToString(row[1])
		startLine := AnyToString(row[2])
		codeText := AnyToString(row[3])

		// Find which pattern matched
		for _, pattern := range args.Patterns {
			checkPattern := pattern
			checkCode := codeText
			if !args.CaseSensitive {
				checkPattern = ToLower(pattern)
				checkCode = ToLower(codeText)
			}

			if ContainsStr(checkCode, checkPattern) {
				violations = append(violations, AbsenceViolation{
					Pattern:  pattern,
					FilePath: filePath,
					Function: name,
					Line:     startLine,
					Severity: args.Severity,
				})
				break // Only count once per function even if multiple patterns match
			}
		}
	}

	passed := len(violations) == 0

	// Build output
	var output strings.Builder

	if passed {
		output.WriteString("## ✅ Verification PASSED\n\n")
		output.WriteString(fmt.Sprintf("No matches found for %d pattern(s).\n\n", len(args.Patterns)))
	} else {
		output.WriteString(fmt.Sprintf("## ❌ Verification FAILED (%s)\n\n", strings.ToUpper(args.Severity)))
		output.WriteString(fmt.Sprintf("Found %d violation(s) across %d pattern(s).\n\n", len(violations), len(args.Patterns)))
	}

	// Summary
	output.WriteString("### Summary\n\n")
	output.WriteString(fmt.Sprintf("- **Patterns checked:** %d\n", len(args.Patterns)))
	output.WriteString(fmt.Sprintf("- **Violations found:** %d\n", len(violations)))
	output.WriteString(fmt.Sprintf("- **Files scanned:** %d\n", filesScanned))
	if args.Path != "" {
		output.WriteString(fmt.Sprintf("- **Path filter:** `%s`\n", args.Path))
	}
	output.WriteString("\n")

	// Patterns checked
	output.WriteString("### Patterns\n\n")
	for _, pattern := range args.Patterns {
		found := false
		for _, v := range violations {
			if v.Pattern == pattern {
				found = true
				break
			}
		}
		if found {
			output.WriteString(fmt.Sprintf("- ❌ `%s`\n", pattern))
		} else {
			output.WriteString(fmt.Sprintf("- ✅ `%s`\n", pattern))
		}
	}
	output.WriteString("\n")

	// Violations detail
	if len(violations) > 0 {
		output.WriteString("### Violations\n\n")
		for i, v := range violations {
			if i >= 10 {
				output.WriteString(fmt.Sprintf("\n... and %d more violations\n", len(violations)-10))
				break
			}
			output.WriteString(fmt.Sprintf("**%d. `%s`** found in:\n", i+1, v.Pattern))
			output.WriteString(fmt.Sprintf("   - Function: `%s`\n", v.Function))
			output.WriteString(fmt.Sprintf("   - File: `%s:%s`\n\n", v.FilePath, v.Line))
		}
	}

	return NewResult(output.String()), nil
}
