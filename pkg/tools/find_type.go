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

// FindTypeArgs holds arguments for the find_type tool.
type FindTypeArgs struct {
	Name        string // Type name to search for
	Kind        string // Filter by kind: "any", "struct", "interface", "class", "type_alias"
	PathPattern string // Optional file path filter
	Limit       int    // Max results (default 20)
}

// FindType searches for types/interfaces/classes/structs by name.
// Works across all languages (Go structs/interfaces, Python classes, TypeScript interfaces/classes).
func FindType(ctx context.Context, client Querier, args FindTypeArgs) (*ToolResult, error) {
	if args.Name == "" {
		return NewError("Error: 'name' is required"), nil
	}

	if args.Limit <= 0 {
		args.Limit = 20
	}

	// Build query conditions
	var conditions []string

	// Name matching (case-insensitive regex)
	// Use EscapeRegex for CozoDB compatibility ([.] instead of \.)
	namePattern := fmt.Sprintf("(?i)%s", EscapeRegex(args.Name))
	conditions = append(conditions, fmt.Sprintf("regex_matches(name, %q)", namePattern))

	// Kind filter - use string equality, not regex
	if args.Kind != "" && args.Kind != "any" {
		conditions = append(conditions, fmt.Sprintf("kind == %q", args.Kind))
	}

	// Path filter - user provides regex pattern
	if args.PathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %q)", args.PathPattern))
	}

	// Build query - single line format like other tools
	query := fmt.Sprintf(
		"?[name, kind, file_path, start_line, end_line] := *cie_type { name, kind, file_path, start_line, end_line }, %s :limit %d",
		strings.Join(conditions, ", "),
		args.Limit,
	)

	result, err := client.Query(ctx, query)
	if err != nil {
		// Check if table doesn't exist (needs re-indexing)
		errStr := err.Error()
		if strings.Contains(errStr, "cie_type") && strings.Contains(errStr, "not found") {
			return NewError("Table 'cie_type' not found. Re-index is required to use this tool.\n\n" +
				"Run: `cie index --path /path/to/repo` to rebuild the index with type support."), nil
		}
		return NewError(fmt.Sprintf("Query failed: %v\n\nQuery: %s", err, query)), nil
	}

	if len(result.Rows) == 0 {
		return NewResult(fmt.Sprintf("No types found matching '%s'\n\n"+
			"### Tips:\n"+
			"- Use **cie_semantic_search** for concept-based search\n"+
			"- Check if the index includes types (requires re-indexing after CIE update)\n"+
			"- Try a partial name (e.g., 'Service' instead of 'UserService')", args.Name)), nil
	}

	// Format output
	output := fmt.Sprintf("### Types matching '%s'\n\n", args.Name)
	if args.Kind != "" && args.Kind != "any" {
		output = fmt.Sprintf("### %s types matching '%s'\n\n", args.Kind, args.Name)
	}

	for i, row := range result.Rows {
		name := AnyToString(row[0])
		kind := AnyToString(row[1])
		filePath := AnyToString(row[2])
		startLine := AnyToString(row[3])

		output += fmt.Sprintf("%d. **%s** (%s)\n", i+1, name, kind)
		output += fmt.Sprintf("   File: %s:%s\n\n", filePath, startLine)
	}

	return NewResult(output), nil
}

// TypeInfo represents information about a type.
type TypeInfo struct {
	Name      string
	Kind      string
	FilePath  string
	StartLine int
	EndLine   int
	CodeText  string
}

// GetTypeCode retrieves the code of a specific type.
// Schema v3: code_text is in cie_type_code table
func GetTypeCode(ctx context.Context, client Querier, name, filePath string) (*ToolResult, error) {
	if name == "" {
		return NewError("Error: 'name' is required"), nil
	}

	// Build query - use exact match with ==
	// Schema v3: Join with cie_type_code for code_text
	var query string
	if filePath != "" {
		query = fmt.Sprintf(
			"?[name, kind, file_path, code_text, start_line, end_line] := "+
				"*cie_type { id, name, kind, file_path, start_line, end_line }, "+
				"*cie_type_code { type_id: id, code_text }, "+
				"name == %q, file_path == %q :limit 1",
			name, filePath)
	} else {
		query = fmt.Sprintf(
			"?[name, kind, file_path, code_text, start_line, end_line] := "+
				"*cie_type { id, name, kind, file_path, start_line, end_line }, "+
				"*cie_type_code { type_id: id, code_text }, "+
				"name == %q :limit 1",
			name)
	}

	result, err := client.Query(ctx, query)
	if err != nil {
		return NewError(fmt.Sprintf("Query failed: %v", err)), nil
	}

	if len(result.Rows) == 0 {
		return NewResult(fmt.Sprintf("Type '%s' not found", name)), nil
	}

	row := result.Rows[0]
	typeName := AnyToString(row[0])
	kind := AnyToString(row[1])
	path := AnyToString(row[2])
	codeText := AnyToString(row[3])
	startLine := AnyToString(row[4])
	endLine := AnyToString(row[5])

	// Determine language for syntax highlighting
	lang := detectLanguage(path)

	output := fmt.Sprintf("### %s (%s)\n\n", typeName, kind)
	output += fmt.Sprintf("**File:** %s:%s-%s\n\n", path, startLine, endLine)
	output += fmt.Sprintf("```%s\n%s\n```\n", lang, codeText)

	return NewResult(output), nil
}

// detectLanguage detects the programming language from file extension.
// Uses strings.HasSuffix for efficiency (no regex compilation).
func detectLanguage(filePath string) string {
	filePath = strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return "go"
	case strings.HasSuffix(filePath, ".py"):
		return "python"
	case strings.HasSuffix(filePath, ".ts"), strings.HasSuffix(filePath, ".tsx"):
		return "typescript"
	case strings.HasSuffix(filePath, ".js"), strings.HasSuffix(filePath, ".jsx"):
		return "javascript"
	case strings.HasSuffix(filePath, ".rs"):
		return "rust"
	case strings.HasSuffix(filePath, ".java"):
		return "java"
	default:
		return "unknown"
	}
}
