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
	"strings"
)

// GetFunctionCodeArgs holds arguments for getting function code.
type GetFunctionCodeArgs struct {
	FunctionName string
	FullCode     bool // If true, return complete code without truncation
}

// GetFunctionCode retrieves the full source code of a function.
// Schema v3: code_text is in separate cie_function_code table
func GetFunctionCode(ctx context.Context, client Querier, args GetFunctionCodeArgs) (*ToolResult, error) {
	funcName := strings.TrimSpace(args.FunctionName)
	if funcName == "" {
		return NewError("Error: function_name cannot be empty"), nil
	}

	// Try exact match first - join with cie_function_code for code_text
	script := fmt.Sprintf(`?[name, file_path, signature, code_text, start_line, end_line] := *cie_function { id, name, file_path, signature, start_line, end_line }, *cie_function_code { function_id: id, code_text }, regex_matches(name, "(?i)^%s$") :limit 1`, EscapeRegex(funcName))

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v", err)), nil
	}

	if len(result.Rows) == 0 {
		// Try partial match
		script = fmt.Sprintf(`?[name, file_path, signature, code_text, start_line, end_line] := *cie_function { id, name, file_path, signature, start_line, end_line }, *cie_function_code { function_id: id, code_text }, regex_matches(name, "(?i)%s") :limit 1`, EscapeRegex(funcName))
		result, err = client.Query(ctx, script)
		if err != nil {
			return NewError(fmt.Sprintf("Query error: %v", err)), nil
		}
	}

	if len(result.Rows) == 0 {
		return NewResult(fmt.Sprintf("Function '%s' not found.", funcName)), nil
	}

	row := result.Rows[0]
	name := anyToStr(row[0])
	filePath := anyToStr(row[1])
	signature := anyToStr(row[2])
	codeText := anyToStr(row[3])
	startLine := row[4]
	endLine := row[5]

	// Determine language for syntax highlighting
	lang := detectLanguage(filePath)

	// Truncate very long code unless full_code is requested
	truncated := false
	const maxCodeLen = 3000
	if !args.FullCode && len(codeText) > maxCodeLen {
		codeText = codeText[:maxCodeLen]
		truncated = true
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Function**: %s\n", name))
	sb.WriteString(fmt.Sprintf("**File**: %s:%v-%v\n", filePath, startLine, endLine))
	sb.WriteString(fmt.Sprintf("**Signature**: %s\n\n", signature))
	sb.WriteString(fmt.Sprintf("```%s\n%s\n```", lang, codeText))

	if truncated {
		sb.WriteString("\n\n⚠️ **Code truncated**. To view full code:\n")
		sb.WriteString(fmt.Sprintf("- Use `Read` tool: `%s` (lines %v-%v)\n", filePath, startLine, endLine))
		sb.WriteString("- Or call this tool with `full_code: true`")
	}

	return NewResult(sb.String()), nil
}

// ListFunctionsInFileArgs holds arguments for listing functions in a file.
type ListFunctionsInFileArgs struct {
	FilePath string
}

// ListFunctionsInFile lists all functions defined in a specific file.
func ListFunctionsInFile(ctx context.Context, client Querier, args ListFunctionsInFileArgs) (*ToolResult, error) {
	filePath := strings.TrimSpace(args.FilePath)
	if filePath == "" {
		return NewError("Error: file_path cannot be empty"), nil
	}

	// Try exact suffix match first (most reliable)
	script := fmt.Sprintf(`?[name, signature, start_line, file_path] := *cie_function { name, signature, file_path, start_line }, ends_with(file_path, %q) :order start_line :limit 50`, filePath)

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v", err)), nil
	}

	// If no results, try regex match (more flexible)
	if len(result.Rows) == 0 {
		script = fmt.Sprintf(`?[name, signature, start_line, file_path] := *cie_function { name, signature, file_path, start_line }, regex_matches(file_path, "(?i)%s") :order start_line :limit 50`, EscapeRegex(filePath))
		result, err = client.Query(ctx, script)
		if err != nil {
			return NewError(fmt.Sprintf("Query error: %v", err)), nil
		}
	}

	if len(result.Rows) == 0 {
		// Check if the file exists in the index at all
		fileCheck := fmt.Sprintf(`?[path] := *cie_file { path }, ends_with(path, %q) :limit 1`, filePath)
		fileResult, _ := client.Query(ctx, fileCheck)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**No functions found in '%s'**\n\n", filePath))

		if fileResult != nil && len(fileResult.Rows) > 0 {
			sb.WriteString("ℹ️ The file IS indexed, but contains no extractable functions.\n")
			sb.WriteString("This can happen if:\n")
			sb.WriteString("- The file only contains type definitions, constants, or imports\n")
			sb.WriteString("- The parser couldn't extract functions (unsupported syntax)\n")
		} else {
			// Check if similar files exist
			similarCheck := fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, "(?i)%s") :limit 5`, EscapeRegex(extractFileName(filePath)))
			similarResult, _ := client.Query(ctx, similarCheck)

			sb.WriteString("⚠️ The file is NOT in the index.\n\n")
			sb.WriteString("Possible causes:\n")
			sb.WriteString("1. Path doesn't match exactly - check for typos\n")
			sb.WriteString("2. File was excluded by indexing rules in `.cie/project.yaml`\n")
			sb.WriteString("3. File was added after last indexing - run `cie index`\n")

			if similarResult != nil && len(similarResult.Rows) > 0 {
				sb.WriteString("\n**Similar indexed files:**\n")
				for _, row := range similarResult.Rows {
					sb.WriteString(fmt.Sprintf("- `%s`\n", anyToStr(row[0])))
				}
			}
		}
		return NewResult(sb.String()), nil
	}

	// Get the actual file path from results for accurate reporting
	actualPath := filePath
	if len(result.Rows) > 0 && len(result.Rows[0]) > 3 {
		actualPath = anyToStr(result.Rows[0][3])
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Functions in %s** (%d found):\n\n", actualPath, len(result.Rows)))
	for _, row := range result.Rows {
		name := anyToStr(row[0])
		signature := anyToStr(row[1])
		line := row[2]
		sb.WriteString(fmt.Sprintf("• Line %v: **%s**\n", line, name))
		if len(signature) < 80 {
			sb.WriteString(fmt.Sprintf("  `%s`\n", signature))
		}
		sb.WriteString("\n")
	}

	return NewResult(sb.String()), nil
}

// extractFileName extracts the file name from a path for fuzzy matching.
func extractFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// GetCallGraphArgs holds arguments for getting a call graph.
type GetCallGraphArgs struct {
	FunctionName string
}

// GetCallGraph gets both callers and callees of a function.
func GetCallGraph(ctx context.Context, client Querier, args GetCallGraphArgs) (*ToolResult, error) {
	funcName := strings.TrimSpace(args.FunctionName)
	if funcName == "" {
		return NewError("Error: function_name cannot be empty"), nil
	}

	// Get callers
	callersResult, err := FindCallers(ctx, client, FindCallersArgs{FunctionName: funcName})
	if err != nil {
		return nil, fmt.Errorf("find callers for %s: %w", funcName, err)
	}

	// Get callees
	calleesResult, err := FindCallees(ctx, client, FindCalleesArgs{FunctionName: funcName})
	if err != nil {
		return nil, fmt.Errorf("find callees for %s: %w", funcName, err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Call Graph for '%s'\n\n", funcName))
	sb.WriteString("## Callers (functions that call this):\n")
	sb.WriteString(callersResult.Text)
	sb.WriteString("\n\n## Callees (functions called by this):\n")
	sb.WriteString(calleesResult.Text)

	return NewResult(sb.String()), nil
}

// FindSimilarFunctionsArgs holds arguments for finding similar functions.
type FindSimilarFunctionsArgs struct {
	Pattern string
}

// FindSimilarFunctions finds functions with similar names.
func FindSimilarFunctions(ctx context.Context, client Querier, args FindSimilarFunctionsArgs) (*ToolResult, error) {
	pattern := strings.TrimSpace(args.Pattern)
	if pattern == "" {
		return NewError("Error: pattern cannot be empty"), nil
	}

	script := fmt.Sprintf(`?[name, file_path, signature] := *cie_function { name, file_path, signature }, regex_matches(name, "(?i)%s") :limit 20`, EscapeRegex(pattern))

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v", err)), nil
	}

	if len(result.Rows) == 0 {
		return NewResult(fmt.Sprintf("No functions matching '%s' found.", pattern)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Functions similar to '%s':\n\n", pattern))
	for _, row := range result.Rows {
		name := anyToStr(row[0])
		filePath := anyToStr(row[1])
		signature := anyToStr(row[2])
		sb.WriteString(fmt.Sprintf("• **%s**\n  File: %s\n", name, filePath))
		if len(signature) < 100 {
			sb.WriteString(fmt.Sprintf("  Signature: %s\n", signature))
		}
		sb.WriteString("\n")
	}

	return NewResult(sb.String()), nil
}

// GetFileSummaryArgs holds arguments for getting a file summary.
type GetFileSummaryArgs struct {
	FilePath string
}

// GetFileSummary gets a summary of all entities in a file (functions, types, methods).
func GetFileSummary(ctx context.Context, client Querier, args GetFileSummaryArgs) (*ToolResult, error) {
	filePath := strings.TrimSpace(args.FilePath)
	if filePath == "" {
		return NewError("Error: file_path cannot be empty"), nil
	}

	typeResult, funcResult, err := queryFileSummaryEntities(ctx, client, filePath)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v", err)), nil
	}

	if len(typeResult.Rows) == 0 && len(funcResult.Rows) == 0 {
		return NewResult(fmt.Sprintf("No entities found in '%s'.", filePath)), nil
	}

	return NewResult(formatFileSummary(filePath, typeResult.Rows, funcResult.Rows)), nil
}

func queryFileSummaryEntities(ctx context.Context, client Querier, filePath string) (*QueryResult, *QueryResult, error) {
	escapedPath := EscapeRegex(filePath)
	typeScript := fmt.Sprintf(`?[name, kind, start_line] := *cie_type { name, kind, file_path, start_line }, regex_matches(file_path, "(?i)%s") :order start_line :limit 100`, escapedPath)
	typeResult, _ := client.Query(ctx, typeScript)
	if typeResult == nil {
		typeResult = &QueryResult{}
	}

	funcScript := fmt.Sprintf(`?[name, signature, start_line] := *cie_function { name, signature, file_path, start_line }, regex_matches(file_path, "(?i)%s") :order start_line :limit 100`, escapedPath)
	funcResult, err := client.Query(ctx, funcScript)
	return typeResult, funcResult, err
}

func formatFileSummary(filePath string, typeRows, funcRows [][]any) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Summary of %s\n\n", filePath))

	formatFileSummaryTypes(&sb, typeRows)
	formatFileSummaryFunctions(&sb, funcRows)

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("**Total**: %d types, %d functions/methods\n", len(typeRows), len(funcRows)))
	return sb.String()
}

func formatFileSummaryTypes(sb *strings.Builder, rows [][]any) {
	if len(rows) == 0 {
		return
	}
	_, _ = fmt.Fprintf(sb, "## Types (%d)\n\n", len(rows))
	for _, row := range rows {
		_, _ = fmt.Fprintf(sb, "• **Line %v**: `%s` (%s)\n", row[2], anyToStr(row[0]), anyToStr(row[1]))
	}
	sb.WriteString("\n")
}

func formatFileSummaryFunctions(sb *strings.Builder, rows [][]any) {
	if len(rows) == 0 {
		return
	}
	var methods, functions [][]any
	for _, row := range rows {
		if strings.Contains(anyToStr(row[0]), ".") {
			methods = append(methods, row)
		} else {
			functions = append(functions, row)
		}
	}
	formatFuncSection(sb, "Functions", functions)
	formatFuncSection(sb, "Methods", methods)
}

func formatFuncSection(sb *strings.Builder, title string, rows [][]any) {
	if len(rows) == 0 {
		return
	}
	_, _ = fmt.Fprintf(sb, "## %s (%d)\n\n", title, len(rows))
	for _, row := range rows {
		name, signature := anyToStr(row[0]), anyToStr(row[1])
		_, _ = fmt.Fprintf(sb, "• **Line %v**: `%s`\n", row[2], name)
		if len(signature) > 0 && len(signature) < 100 {
			_, _ = fmt.Fprintf(sb, "  `%s`\n", signature)
		}
	}
	sb.WriteString("\n")
}
