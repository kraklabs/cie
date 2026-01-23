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

// SearchTextArgs holds arguments for text search.
type SearchTextArgs struct {
	Pattern        string
	SearchIn       string // "code", "signature", "name", "all"
	FilePattern    string
	ExcludePattern string // Pattern to exclude (uses negate())
	Literal        bool   // If true, treat pattern as literal string (escape regex chars)
	Limit          int
}

// SearchText searches for text patterns in function code, signatures, or names.
// Schema v3: code_text is in separate cie_function_code table
func SearchText(ctx context.Context, client Querier, args SearchTextArgs) (*ToolResult, error) {
	if args.Pattern == "" {
		return NewError("Error: 'pattern' is required"), nil
	}

	if args.SearchIn == "" {
		args.SearchIn = "all"
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	// Validate regex if not in literal mode
	// Validate regex if not in literal mode
	if !args.Literal {
		if _, err := regexp.Compile(args.Pattern); err != nil {
			return NewError(fmt.Sprintf(
				"**Invalid Regex Pattern:**\n```\n%v\n```\n\n"+
					"The pattern `%s` is not valid regex: %v\n\n"+
					"### Solutions:\n"+
					"1. **Use literal mode** (recommended for exact matches):\n"+
					"   ```json\n"+
					"   {\"pattern\": \"%s\", \"literal\": true}\n"+
					"   ```\n\n"+
					"2. **Escape special characters manually:**\n"+
					"   Special regex chars: `.` `(` `)` `[` `]` `{` `}` `*` `+` `?` `^` `$` `|` `\\`\n"+
					"   Example: `.GET(` → `\\.GET\\(`\n\n"+
					"### Tip:\n"+
					"Use `literal: true` when searching for exact code patterns like `.GET(`, `->`, `::`, etc.",
				err, args.Pattern, err, args.Pattern)), nil
		}
	}

	// Escape pattern if literal mode is requested
	pattern := args.Pattern
	if args.Literal {
		pattern = EscapeRegex(pattern)
	}

	// Determine if we need to join with cie_function_code (only for code/all search)
	needsCodeJoin := args.SearchIn == "code" || args.SearchIn == "all"

	// Build query based on search target
	var conditions []string
	switch args.SearchIn {
	case "code":
		conditions = append(conditions, fmt.Sprintf("regex_matches(code_text, %q)", pattern))
	case "signature":
		conditions = append(conditions, fmt.Sprintf("regex_matches(signature, %q)", pattern))
	case "name":
		conditions = append(conditions, fmt.Sprintf("regex_matches(name, %q)", pattern))
	default: // "all"
		conditions = append(conditions, fmt.Sprintf("(regex_matches(name, %q) or regex_matches(signature, %q) or regex_matches(code_text, %q))", pattern, pattern, pattern))
	}

	if args.FilePattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %q)", args.FilePattern))
	}

	// Handle exclude pattern using negate() - CozoDB doesn't support negative lookahead
	if args.ExcludePattern != "" {
		conditions = append(conditions, fmt.Sprintf("negate(regex_matches(file_path, %q))", args.ExcludePattern))
	}

	// Schema v3: Join with cie_function_code only when searching in code
	var script string
	if needsCodeJoin {
		script = fmt.Sprintf(
			"?[file_path, name, signature, start_line, end_line] := *cie_function { id, file_path, name, signature, start_line, end_line }, *cie_function_code { function_id: id, code_text }, %s :limit %d",
			strings.Join(conditions, ", "),
			args.Limit,
		)
	} else {
		script = fmt.Sprintf(
			"?[file_path, name, signature, start_line, end_line] := *cie_function { file_path, name, signature, start_line, end_line }, %s :limit %d",
			strings.Join(conditions, ", "),
			args.Limit,
		)
	}

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nGenerated query:\n%s", err, script)), nil
	}

	return NewResult(FormatQueryResult(result, script)), nil
}

// FindFunctionArgs holds arguments for finding functions.
type FindFunctionArgs struct {
	Name        string
	ExactMatch  bool
	IncludeCode bool
}

// FindFunction finds functions by name.
// Schema v3: code_text is in separate cie_function_code table
func FindFunction(ctx context.Context, client Querier, args FindFunctionArgs) (*ToolResult, error) {
	if args.Name == "" {
		return NewError("Error: 'name' is required"), nil
	}

	var condition string
	if args.ExactMatch {
		condition = fmt.Sprintf("name = %q", args.Name)
	} else {
		// Match exact name OR methods ending with .Name
		condition = fmt.Sprintf("(name = %q or ends_with(name, %q))", args.Name, "."+args.Name)
	}

	// Schema v3: Join with cie_function_code only when include_code is true
	var script string
	if args.IncludeCode {
		script = fmt.Sprintf("?[file_path, name, signature, start_line, end_line, code_text] := *cie_function { id, file_path, name, signature, start_line, end_line }, *cie_function_code { function_id: id, code_text }, %s", condition)
	} else {
		script = fmt.Sprintf("?[file_path, name, signature, start_line, end_line] := *cie_function { file_path, name, signature, start_line, end_line }, %s", condition)
	}

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nGenerated query:\n%s", err, script)), nil
	}

	return NewResult(FormatQueryResult(result, script)), nil
}

// FindCallersArgs holds arguments for finding callers.
type FindCallersArgs struct {
	FunctionName    string
	IncludeIndirect bool
}

// FindCallers finds all functions that call a specific function.
func FindCallers(ctx context.Context, client Querier, args FindCallersArgs) (*ToolResult, error) {
	if args.FunctionName == "" {
		return NewError("Error: 'function_name' is required"), nil
	}

	condition := fmt.Sprintf("(callee_name = %q or ends_with(callee_name, %q))", args.FunctionName, "."+args.FunctionName)

	script := fmt.Sprintf(`?[caller_file, caller_name, caller_line, callee_name] := 
  *cie_calls { caller_id, callee_id },
  *cie_function { id: callee_id, name: callee_name },
  *cie_function { id: caller_id, file_path: caller_file, name: caller_name, start_line: caller_line },
  %s`, condition)

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nGenerated query:\n%s", err, script)), nil
	}

	return NewResult(FormatQueryResult(result, script)), nil
}

// FindCalleesArgs holds arguments for finding callees.
type FindCalleesArgs struct {
	FunctionName string
}

// FindCallees finds all functions called by a specific function.
func FindCallees(ctx context.Context, client Querier, args FindCalleesArgs) (*ToolResult, error) {
	if args.FunctionName == "" {
		return NewError("Error: 'function_name' is required"), nil
	}

	condition := fmt.Sprintf("(caller_name = %q or ends_with(caller_name, %q))", args.FunctionName, "."+args.FunctionName)

	script := fmt.Sprintf(`?[caller_name, callee_file, callee_name, callee_line] := 
  *cie_calls { caller_id, callee_id },
  *cie_function { id: caller_id, name: caller_name },
  *cie_function { id: callee_id, file_path: callee_file, name: callee_name, start_line: callee_line },
  %s`, condition)

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nGenerated query:\n%s", err, script)), nil
	}

	return NewResult(FormatQueryResult(result, script)), nil
}

// ListFilesArgs holds arguments for listing files.
type ListFilesArgs struct {
	PathPattern string
	Language    string
	Limit       int
}

// ListFiles lists files in the indexed codebase.
func ListFiles(ctx context.Context, client Querier, args ListFilesArgs) (*ToolResult, error) {
	if args.Limit <= 0 {
		args.Limit = 50
	}

	var conditions []string
	if args.PathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(path, %q)", args.PathPattern))
	}
	if args.Language != "" {
		conditions = append(conditions, fmt.Sprintf("language = %q", args.Language))
	}

	script := "?[path, language, size] := *cie_file { path, language, size }"
	if len(conditions) > 0 {
		script += ", " + strings.Join(conditions, ", ")
	}
	script += fmt.Sprintf(" :limit %d", args.Limit)

	result, err := client.Query(ctx, script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nGenerated query:\n%s", err, script)), nil
	}

	return NewResult(FormatQueryResult(result, script)), nil
}

// RawQueryArgs holds arguments for raw queries.
type RawQueryArgs struct {
	Script string
}

// RawQuery executes a raw CozoScript query.
func RawQuery(ctx context.Context, client Querier, args RawQueryArgs) (*ToolResult, error) {
	if args.Script == "" {
		return NewError("Error: 'script' is required"), nil
	}

	result, err := client.Query(ctx, args.Script)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v\n\nQuery:\n%s", err, args.Script)), nil
	}

	return NewResult(FormatQueryResult(result, args.Script)), nil
}
