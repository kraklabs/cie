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

import "strings"

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	Text    string
	IsError bool
}

// NewResult creates a successful tool result.
func NewResult(text string) *ToolResult {
	return &ToolResult{Text: text}
}

// NewError creates an error tool result.
func NewError(text string) *ToolResult {
	return &ToolResult{Text: text, IsError: true}
}

// FunctionInfo represents a function found in the codebase.
type FunctionInfo struct {
	ID        string
	Name      string
	Signature string
	FilePath  string
	CodeText  string
	StartLine int
	EndLine   int
}

// FileInfo represents a file in the codebase.
type FileInfo struct {
	ID       string
	Path     string
	Language string
	Size     int
}

// CallerInfo represents a function call relationship.
type CallerInfo struct {
	CallerName string
	CallerFile string
	CallerLine int
	CalleeName string
}

// Truncate truncates a string to the specified length.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatQueryResult formats a QueryResult as a readable string.
func FormatQueryResult(result *QueryResult, script string) string {
	var sb strings.Builder
	sb.WriteString("Found ")
	sb.WriteString(intToStr(len(result.Rows)))
	sb.WriteString(" results\n\n")

	if len(result.Rows) == 0 {
		sb.WriteString("No results found.\n")
	} else {
		for i, row := range result.Rows {
			sb.WriteString("--- Result ")
			sb.WriteString(intToStr(i + 1))
			sb.WriteString(" ---\n")
			for j, val := range row {
				if j < len(result.Headers) {
					valStr := anyToStr(val)
					if len(valStr) > 200 {
						valStr = valStr[:200] + "..."
					}
					sb.WriteString("  ")
					sb.WriteString(result.Headers[j])
					sb.WriteString(": ")
					sb.WriteString(valStr)
					sb.WriteString("\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\nGenerated CozoScript:\n")
	sb.WriteString(script)

	return sb.String()
}

// FormatQueryResultSimple formats without the script.
func FormatQueryResultSimple(result *QueryResult) string {
	var sb strings.Builder

	if len(result.Rows) == 0 {
		return "No results found."
	}

	sb.WriteString("Found ")
	sb.WriteString(intToStr(len(result.Rows)))
	sb.WriteString(" results:\n\n")

	for i, row := range result.Rows {
		if i >= 20 {
			sb.WriteString("\n... and ")
			sb.WriteString(intToStr(len(result.Rows) - 20))
			sb.WriteString(" more results")
			break
		}
		for j, val := range row {
			if j < len(result.Headers) {
				valStr := anyToStr(val)
				if len(valStr) > 100 {
					valStr = valStr[:100] + "..."
				}
				sb.WriteString("  ")
				sb.WriteString(result.Headers[j])
				sb.WriteString(": ")
				sb.WriteString(valStr)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + intToStr(-i)
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

func anyToStr(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return intToStr(int(val))
		}
		return strings.TrimRight(strings.TrimRight(floatToStr(val), "0"), ".")
	case int:
		return intToStr(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return "<?>"
	}
}

func floatToStr(f float64) string {
	// Simple float to string for display
	if f == 0 {
		return "0"
	}
	negative := f < 0
	if negative {
		f = -f
	}
	intPart := int(f)
	fracPart := int((f - float64(intPart)) * 1000000)
	result := intToStr(intPart)
	if fracPart > 0 {
		result += "." + intToStr(fracPart)
	}
	if negative {
		result = "-" + result
	}
	return result
}

// RolePattern defines how to identify a role in code.
type RolePattern struct {
	// FilePattern is a regex to match file paths (e.g., ".*/routes/.*\\.go")
	FilePattern string
	// NamePattern is a regex to match function names (e.g., ".*Handler$")
	NamePattern string
	// CodePattern is a regex to match code content (e.g., "\\.GET\\(")
	CodePattern string
	// Description explains what this role represents
	Description string
}
