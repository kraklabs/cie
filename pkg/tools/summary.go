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

// DirectorySummary shows files in a directory with their main functions
func DirectorySummary(ctx context.Context, client Querier, path string, maxFuncsPerFile int) (*ToolResult, error) {
	if path == "" {
		return NewError("Error: 'path' is required"), nil
	}

	// Normalize path - remove trailing slash
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// 1. Get all files in the directory
	// Use caret anchor for prefix match
	filesQuery := fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :order path :limit 100`, "^"+EscapeRegex(path)+"/")
	filesResult, err := client.Query(ctx, filesQuery)
	if err != nil {
		return nil, err
	}

	if len(filesResult.Rows) == 0 {
		// Try without the ^ anchor in case it's a partial match
		filesQuery = fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :order path :limit 100`, EscapeRegex(path))
		filesResult, err = client.Query(ctx, filesQuery)
		if err != nil {
			return nil, err
		}
	}

	if len(filesResult.Rows) == 0 {
		return NewResult(fmt.Sprintf("No files found in path: `%s`\n\nUse `cie_list_files` to see available paths.", path)), nil
	}

	// 2. Get functions for each file (prioritize exported/public functions)
	output := fmt.Sprintf("# Directory Summary: `%s`\n\n", path)
	output += fmt.Sprintf("Found **%d files**\n\n", len(filesResult.Rows))

	// Group files by subdirectory for better organization
	filesByDir := make(map[string][]string)
	for _, row := range filesResult.Rows {
		filePath := AnyToString(row[0])
		dir := ExtractDir(filePath)
		filesByDir[dir] = append(filesByDir[dir], filePath)
	}

	// Process each file
	for _, row := range filesResult.Rows {
		filePath := AnyToString(row[0])

		// Get functions for this file, prioritizing:
		// 1. Exported functions (capitalized names in Go)
		// 2. Public functions (no underscore prefix)
		// 3. Shorter names (likely more important)
		funcsQuery := fmt.Sprintf(`?[name, signature, start_line] := *cie_function { name, signature, start_line, file_path }, file_path == %q :order name :limit %d`, filePath, maxFuncsPerFile*2)
		funcsResult, err := client.Query(ctx, funcsQuery)
		if err != nil {
			continue // Skip this file on error
		}

		// Format file entry
		fileName := ExtractFileName(filePath)
		output += fmt.Sprintf("## `%s`\n", fileName)
		output += fmt.Sprintf("_Path: %s_\n\n", filePath)

		if len(funcsResult.Rows) == 0 {
			output += "_No functions found_\n\n"
			continue
		}

		// Sort functions: exported first, then by name
		type funcInfo struct {
			name      string
			signature string
			line      string
			exported  bool
		}
		var funcs []funcInfo
		for _, frow := range funcsResult.Rows {
			name := AnyToString(frow[0])
			sig := AnyToString(frow[1])
			line := AnyToString(frow[2])
			exported := len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
			funcs = append(funcs, funcInfo{name, sig, line, exported})
		}

		// Show exported functions first
		shown := 0
		for _, f := range funcs {
			if f.exported && shown < maxFuncsPerFile {
				sigShort := f.signature
				if len(sigShort) > 80 {
					sigShort = sigShort[:77] + "..."
				}
				output += fmt.Sprintf("- **%s** (line %s)\n", f.name, f.line)
				if sigShort != "" && sigShort != f.name {
					output += fmt.Sprintf("  `%s`\n", sigShort)
				}
				shown++
			}
		}

		// Fill remaining slots with non-exported
		for _, f := range funcs {
			if !f.exported && shown < maxFuncsPerFile {
				output += fmt.Sprintf("- %s (line %s)\n", f.name, f.line)
				shown++
			}
		}

		if len(funcsResult.Rows) > maxFuncsPerFile {
			output += fmt.Sprintf("  _... and %d more functions_\n", len(funcsResult.Rows)-maxFuncsPerFile)
		}
		output += "\n"
	}

	return NewResult(output), nil
}
