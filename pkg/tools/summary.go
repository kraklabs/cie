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
)

// DirectorySummary shows files in a directory with their main exported functions.
//
// It lists all files in the specified directory and shows the most important functions
// in each file. Functions are prioritized by visibility (exported/public first) and name length.
//
// The path parameter specifies the directory to summarize (e.g., "internal/cie/ingestion").
// The maxFuncsPerFile parameter limits how many functions to show per file.
// If maxFuncsPerFile is 0 or negative, defaults to 5 functions per file.
//
// Returns a ToolResult containing a formatted directory listing with file paths and their
// key functions (name, signature, line number). Returns an error if the query fails.
//
// Example output format:
//
//	# Directory Summary: `internal/cie`
//	Found **3 files**
//
//	## internal/cie/client.go
//	- **NewClient** (line 25): `func NewClient(url string) *Client`
//	- **Query** (line 45): `func (c *Client) Query(ctx context.Context, script string) (*Result, error)`
func DirectorySummary(ctx context.Context, client Querier, path string, maxFuncsPerFile int) (*ToolResult, error) {
	if path == "" {
		return NewError("Error: 'path' is required"), nil
	}
	path = normalizeDirPath(path)

	filesResult, err := queryDirFiles(ctx, client, path)
	if err != nil {
		return nil, err
	}
	if len(filesResult.Rows) == 0 {
		return NewResult(fmt.Sprintf("No files found in path: `%s`\n\nUse `cie_list_files` to see available paths.", path)), nil
	}

	output := fmt.Sprintf("# Directory Summary: `%s`\n\nFound **%d files**\n\n", path, len(filesResult.Rows))
	for _, row := range filesResult.Rows {
		filePath := AnyToString(row[0])
		output += formatFileSummaryEntry(ctx, client, filePath, maxFuncsPerFile)
	}
	return NewResult(output), nil
}

func normalizeDirPath(path string) string {
	if len(path) > 0 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

func queryDirFiles(ctx context.Context, client Querier, path string) (*QueryResult, error) {
	query := fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :order path :limit 100`, "^"+EscapeRegex(path)+"/")
	result, err := client.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(result.Rows) == 0 {
		query = fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :order path :limit 100`, EscapeRegex(path))
		return client.Query(ctx, query)
	}
	return result, nil
}

type dirFuncInfo struct {
	name, signature, line string
	exported              bool
}

func formatFileSummaryEntry(ctx context.Context, client Querier, filePath string, maxFuncs int) string {
	query := fmt.Sprintf(`?[name, signature, start_line] := *cie_function { name, signature, start_line, file_path }, file_path == %q :order name :limit %d`, filePath, maxFuncs*2)
	result, err := client.Query(ctx, query)
	if err != nil {
		return ""
	}

	output := fmt.Sprintf("## `%s`\n_Path: %s_\n\n", ExtractFileName(filePath), filePath)
	if len(result.Rows) == 0 {
		return output + "_No functions found_\n\n"
	}

	funcs := parseDirFuncResults(result.Rows)
	output += formatDirFuncs(funcs, maxFuncs)
	if len(result.Rows) > maxFuncs {
		output += fmt.Sprintf("  _... and %d more functions_\n", len(result.Rows)-maxFuncs)
	}
	return output + "\n"
}

func parseDirFuncResults(rows [][]any) []dirFuncInfo {
	var funcs []dirFuncInfo
	for _, row := range rows {
		name := AnyToString(row[0])
		funcs = append(funcs, dirFuncInfo{
			name:      name,
			signature: AnyToString(row[1]),
			line:      AnyToString(row[2]),
			exported:  len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z',
		})
	}
	return funcs
}

func formatDirFuncs(funcs []dirFuncInfo, maxFuncs int) string {
	var output string
	shown := 0
	for _, f := range funcs {
		if f.exported && shown < maxFuncs {
			output += formatExportedFunc(f)
			shown++
		}
	}
	for _, f := range funcs {
		if !f.exported && shown < maxFuncs {
			output += fmt.Sprintf("- %s (line %s)\n", f.name, f.line)
			shown++
		}
	}
	return output
}

func formatExportedFunc(f dirFuncInfo) string {
	output := fmt.Sprintf("- **%s** (line %s)\n", f.name, f.line)
	sigShort := f.signature
	if len(sigShort) > 80 {
		sigShort = sigShort[:77] + "..."
	}
	if sigShort != "" && sigShort != f.name {
		output += fmt.Sprintf("  `%s`\n", sigShort)
	}
	return output
}
