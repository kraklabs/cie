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

// Package sigparse provides Go function signature parsing utilities.
// It is a dependency-free package that can be imported by both
// pkg/ingestion (for ingestion-time dispatch) and pkg/tools (for query-time dispatch).
package sigparse

import "strings"

// ParamInfo holds a parsed parameter's name and base type.
type ParamInfo struct {
	Name string // Parameter name (e.g., "client")
	Type string // Base type name without pointer/slice prefixes (e.g., "Querier")
}

// ParseGoParams parses a Go function signature string and returns
// the parameter names and their base types.
//
// It handles:
//   - Simple params: "name string, age int"
//   - Grouped params: "a, b int" → [{a, int}, {b, int}]
//   - Qualified types: "tools.Querier" → base type "Querier"
//   - Pointer types: "*Querier" → "Querier"
//   - Slice types: "[]Querier" → "Querier"
//   - Variadic types: "...string" → "string"
//   - Func params: "fn func(int) error" → skipped (type is "func")
//   - Method receivers: "func (b *Builder) Build(...)" → receiver excluded
//
// The signature parameter should be a full Go function signature string,
// e.g., "func (s *Server) Run(ctx context.Context, q Querier) error".
func ParseGoParams(signature string) []ParamInfo {
	if signature == "" {
		return nil
	}

	paramStr := ExtractParamString(signature)
	if paramStr == "" {
		return nil
	}

	parts := splitAtTopLevelCommas(paramStr)

	// Process right-to-left for Go grouped-param semantics.
	var params []ParamInfo
	var pendingType string

	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}

		tokens := splitParamTokens(p)
		switch len(tokens) {
		case 0:
			continue
		case 1:
			if pendingType != "" {
				params = append(params, ParamInfo{Name: tokens[0], Type: pendingType})
			}
		default:
			baseType := NormalizeType(tokens[len(tokens)-1])
			name := tokens[0]
			pendingType = baseType
			params = append(params, ParamInfo{Name: name, Type: baseType})
		}
	}

	// Reverse to restore left-to-right order
	for i, j := 0, len(params)-1; i < j; i, j = i+1, j-1 {
		params[i], params[j] = params[j], params[i]
	}

	return params
}

// ExtractParamString extracts the parameter list from a Go function signature.
// Given "func (r *Type) Name(ctx Context, q Querier) error", returns "ctx Context, q Querier".
func ExtractParamString(sig string) string {
	idx := strings.Index(sig, "func")
	if idx == -1 {
		return ""
	}
	pos := idx + 4

	pos = skipWhitespace(sig, pos)

	// If next char is '(', this is a receiver — skip it
	if pos < len(sig) && sig[pos] == '(' {
		end := findMatchingParen(sig, pos)
		if end == -1 {
			return ""
		}
		pos = end + 1
	}

	// Skip whitespace and function name
	pos = skipWhitespace(sig, pos)
	for pos < len(sig) && sig[pos] != '(' {
		pos++
	}

	if pos >= len(sig) {
		return ""
	}

	end := findMatchingParen(sig, pos)
	if end == -1 {
		return ""
	}

	return sig[pos+1 : end]
}

// NormalizeType extracts the base type name from a Go type expression.
//
//	"*Querier" → "Querier"
//	"[]Querier" → "Querier"
//	"tools.Querier" → "Querier"
//	"*tools.Querier" → "Querier"
//	"...string" → "string"
//	"func(int) error" → "func"
func NormalizeType(t string) string {
	t = strings.TrimLeft(t, "*")

	if strings.HasPrefix(t, "[]") {
		t = t[2:]
		t = strings.TrimLeft(t, "*")
	}

	t = strings.TrimPrefix(t, "...")

	if strings.HasPrefix(t, "func") {
		return "func"
	}

	if dot := strings.LastIndex(t, "."); dot >= 0 {
		t = t[dot+1:]
	}

	return t
}

func findMatchingParen(s string, pos int) int {
	depth := 0
	for i := pos; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitAtTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func splitParamTokens(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	s = strings.TrimPrefix(s, "...")

	var tokens []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}

		start := i
		if s[i] == '*' || s[i] == '[' {
			tokens = append(tokens, s[start:])
			break
		}

		if strings.HasPrefix(s[i:], "func") {
			tokens = append(tokens, s[start:])
			break
		}

		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '(' && s[i] != ')' {
			i++
		}
		token := s[start:i]
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

func skipWhitespace(s string, pos int) int {
	for pos < len(s) && (s[pos] == ' ' || s[pos] == '\t' || s[pos] == '\n') {
		pos++
	}
	return pos
}
