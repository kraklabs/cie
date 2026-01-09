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
	"fmt"
)

// EscapeRegex escapes special regex characters for CozoDB.
// CozoDB regex engine requires [X] notation for most special characters
// because backslash escaping doesn't work reliably.
func EscapeRegex(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		// All special regex characters use [X] notation for CozoDB compatibility
		case '.':
			result = append(result, '[', '.', ']')
		case '(':
			result = append(result, '[', '(', ']')
		case ')':
			result = append(result, '[', ')', ']')
		case '[':
			result = append(result, '[', '[', ']')
		case ']':
			result = append(result, '[', ']', ']')
		case '{':
			result = append(result, '[', '{', ']')
		case '}':
			result = append(result, '[', '}', ']')
		case '+':
			result = append(result, '[', '+', ']')
		case '*':
			result = append(result, '[', '*', ']')
		case '?':
			result = append(result, '[', '?', ']')
		case '^':
			result = append(result, '[', '^', ']')
		case '$':
			result = append(result, '[', '$', ']')
		case '|':
			result = append(result, '[', '|', ']')
		case '\\':
			result = append(result, '[', '\\', ']')
		// Quotes don't need escaping - quoteCozoPattern uses raw strings
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

// QuoteCozoPattern wraps a pattern in CozoDB raw string notation.
// Raw strings use ___"..."___ format where nothing inside needs escaping.
// This allows any characters (including quotes) inside the pattern.
func QuoteCozoPattern(pattern string) string {
	return `___"` + pattern + `"___`
}

// ExtractFileName extracts just the filename from a path
func ExtractFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

// ExtractDir extracts directory from file path
func ExtractDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

// ExtractTopDir extracts the top-level directory from a path
func ExtractTopDir(path string) string {
	parts := make([]string, 0)
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if len(parts) == 0 {
		return "."
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + "/" + parts[1]
}

// FormatRows formats query result rows for display
func FormatRows(rows [][]any) string {
	if len(rows) == 0 {
		return "_No results_\n"
	}
	var sb string
	for i, row := range rows {
		if i >= 20 {
			sb += fmt.Sprintf("_... and %d more_\n", len(rows)-20)
			break
		}
		if len(row) >= 3 {
			sb += fmt.Sprintf("- `%s` in `%s:%v`\n", row[0], row[1], row[2])
		} else if len(row) >= 2 {
			sb += fmt.Sprintf("- `%s` in `%s`\n", row[0], row[1])
		} else if len(row) >= 1 {
			sb += fmt.Sprintf("- `%s`\n", row[0])
		}
	}
	return sb
}

func ContainsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func ContainsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if ContainsStr(s, sub) {
			return true
		}
	}
	return false
}

func Query2Lower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// ToLower converts string to lowercase
func ToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// ExtractKeyTerms extracts searchable terms from a query
func ExtractKeyTerms(query string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"is": true, "are": true, "was": true, "were": true,
		"how": true, "what": true, "where": true, "when": true, "why": true,
		"does": true, "do": true, "did": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "that": true, "this": true,
		"function": true, "code": true, "find": true, "search": true,
	}

	var terms []string
	var current string
	for _, c := range query {
		if c == ' ' || c == '\t' || c == '\n' || c == ',' || c == '.' {
			if current != "" && len(current) > 2 && !stopWords[current] {
				terms = append(terms, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" && len(current) > 2 && !stopWords[current] {
		terms = append(terms, current)
	}

	if len(terms) > 5 {
		return terms[:5]
	}
	return terms
}

// AnyToString converts any value to string
func AnyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%.2f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
