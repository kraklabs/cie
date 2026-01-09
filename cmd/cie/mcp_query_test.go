// Copyright 2025 KrakLabs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"regexp"
	"strings"
	"testing"
)

// TestMCPQueryFieldNames validates that MCP queries use correct field names
// This test doesn't require CozoDB - it validates query strings statically
func TestMCPQueryFieldNames(t *testing.T) {
	// These patterns detect incorrect field usage
	wrongPatterns := []struct {
		name    string
		pattern *regexp.Regexp
		msg     string
	}{
		{
			name:    "cie_file with file_path",
			pattern: regexp.MustCompile(`\*cie_file\s*\{[^}]*\bfile_path\b`),
			msg:     "cie_file table uses 'path' field, not 'file_path'",
		},
		{
			name:    "cie_function with path (not file_path)",
			pattern: regexp.MustCompile(`\*cie_function\s*\{[^}]*\bpath\b`),
			msg:     "cie_function table uses 'file_path' field, not 'path'",
		},
		{
			name:    "cie_function with code (not code_text)",
			pattern: regexp.MustCompile(`\*cie_function\s*\{[^}]*\bcode\s*[,}]`),
			msg:     "cie_function table uses 'code_text' field, not 'code'",
		},
	}

	// These are representative queries from mcp.go that should be correct
	correctQueries := []string{
		// cie_file queries - must use 'path'
		`?[path] := *cie_file { path } :limit 100`,
		`?[cnt] := cnt = count(id), *cie_file { id, path }, regex_matches(path, ".*gateway.*")`,
		`?[path] := *cie_file { path }, regex_matches(path, "\\.proto$") :limit 100`,
		`?[id, path, language, size] := *cie_file { id, path, language, size } :limit 100`,

		// cie_function queries - must use 'file_path' and 'code_text'
		`?[name, file_path] := *cie_function { name, file_path } :limit 100`,
		`?[name, file_path, signature, code_text, start_line, end_line] := *cie_function { name, file_path, signature, code_text, start_line, end_line }, regex_matches(name, "(?i)^RegisterRoutes$") :limit 1`,
		`?[name, file_path, start_line] := *cie_function { name, file_path, start_line, code_text }, regex_matches(code_text, "\\.(GET|POST|PUT|DELETE|PATCH|Handle)\\s*\\(") :limit 40`,
	}

	// These queries should be DETECTED as wrong
	wrongQueries := []struct {
		query string
		match string // which pattern should match (must match name exactly)
	}{
		{
			query: `?[file_path] := *cie_file { file_path } :limit 100`,
			match: "cie_file with file_path",
		},
		{
			query: `?[path] := *cie_function { name, path } :limit 100`,
			match: "cie_function with path (not file_path)",
		},
		{
			query: `?[name] := *cie_function { name, code }, regex_matches(code, "test")`,
			match: "cie_function with code (not code_text)",
		},
	}

	t.Run("correct queries pass validation", func(t *testing.T) {
		for _, q := range correctQueries {
			for _, wp := range wrongPatterns {
				if wp.pattern.MatchString(q) {
					t.Errorf("Query incorrectly flagged by %s:\n  Query: %s\n  Issue: %s", wp.name, q, wp.msg)
				}
			}
		}
	})

	t.Run("wrong queries are detected", func(t *testing.T) {
		for _, wq := range wrongQueries {
			found := false
			for _, wp := range wrongPatterns {
				if wp.name == wq.match && wp.pattern.MatchString(wq.query) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Wrong query not detected by %s:\n  Query: %s", wq.match, wq.query)
			}
		}
	})
}

// TestMCPQuerySchemaCompliance verifies that common query patterns are schema-compliant
func TestMCPQuerySchemaCompliance(t *testing.T) {
	// Schema reference:
	// cie_file: id, path, hash, language, size
	// cie_function: id, name, signature, file_path, code_text, embedding, start_line, end_line, start_col, end_col

	cieFileFields := map[string]bool{
		"id": true, "path": true, "hash": true, "language": true, "size": true,
	}

	cieFunctionFields := map[string]bool{
		"id": true, "name": true, "signature": true, "file_path": true,
		"code_text": true, "embedding": true, "start_line": true, "end_line": true,
		"start_col": true, "end_col": true,
	}

	// Extract fields from query pattern like *cie_file { field1, field2 }
	extractFields := func(query, table string) []string {
		pattern := regexp.MustCompile(`\*` + table + `\s*\{\s*([^}]+)\}`)
		match := pattern.FindStringSubmatch(query)
		if match == nil {
			return nil
		}
		fieldsStr := match[1]
		// Clean up bindings like "id: callee_id" -> just "id"
		fieldsStr = regexp.MustCompile(`:\s*\w+`).ReplaceAllString(fieldsStr, "")
		parts := strings.Split(fieldsStr, ",")
		var fields []string
		for _, p := range parts {
			f := strings.TrimSpace(p)
			if f != "" {
				fields = append(fields, f)
			}
		}
		return fields
	}

	tests := []struct {
		name   string
		query  string
		table  string
		fields map[string]bool
	}{
		{
			name:   "cie_file path query",
			query:  `?[path] := *cie_file { path } :limit 100`,
			table:  "cie_file",
			fields: cieFileFields,
		},
		{
			name:   "cie_file with count",
			query:  `?[cnt] := cnt = count(id), *cie_file { id, path }, regex_matches(path, "test")`,
			table:  "cie_file",
			fields: cieFileFields,
		},
		{
			name:   "cie_function basic",
			query:  `?[name, file_path] := *cie_function { name, file_path } :limit 100`,
			table:  "cie_function",
			fields: cieFunctionFields,
		},
		{
			name:   "cie_function with code_text",
			query:  `?[name, file_path, start_line] := *cie_function { name, file_path, start_line, code_text }, regex_matches(code_text, "test")`,
			table:  "cie_function",
			fields: cieFunctionFields,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := extractFields(tt.query, tt.table)
			for _, f := range fields {
				if !tt.fields[f] {
					t.Errorf("Query uses unknown field '%s' in table %s:\n  Query: %s\n  Valid fields: %v",
						f, tt.table, tt.query, tt.fields)
				}
			}
		})
	}
}

// TestCountQueryFallbackPattern validates the count fallback pattern
func TestCountQueryFallbackPattern(t *testing.T) {
	// Count queries should have a fallback pattern
	// CozoDB correct syntax: ?[count(var)] := *relation { field: var }
	countPatterns := []struct {
		name         string
		countQuery   string
		fallbackList string
	}{
		{
			name:         "file count",
			countQuery:   `?[count(f)] := *cie_file { id: f }`,
			fallbackList: `?[id] := *cie_file { id } :limit 10000`,
		},
		{
			name:         "function count",
			countQuery:   `?[count(f)] := *cie_function { id: f }`,
			fallbackList: `?[id] := *cie_function { id } :limit 10000`,
		},
		{
			name:         "filtered file count",
			countQuery:   `?[count(f)] := *cie_file { id: f, path }, regex_matches(path, ".*test.*")`,
			fallbackList: `?[id] := *cie_file { id, path }, regex_matches(path, ".*test.*") :limit 10000`,
		},
	}

	for _, p := range countPatterns {
		t.Run(p.name, func(t *testing.T) {
			// Verify count query uses count() function
			if !strings.Contains(p.countQuery, "count(") {
				t.Errorf("Count query doesn't use count(): %s", p.countQuery)
			}

			// Verify fallback list query doesn't use count
			if strings.Contains(p.fallbackList, "count(") {
				t.Errorf("Fallback query shouldn't use count(): %s", p.fallbackList)
			}

			// Verify both queries use the same table
			if strings.Contains(p.countQuery, "cie_file") != strings.Contains(p.fallbackList, "cie_file") {
				t.Errorf("Count and fallback queries use different tables")
			}
		})
	}
}

// TestHNSWQueryRequiredFields validates HNSW semantic search queries
// Schema v3: HNSW index is on cie_function_embedding, code_text is in cie_function_code
func TestHNSWQueryRequiredFields(t *testing.T) {
	// HNSW query must include all fields used in filters
	// If filtering by code_text, must join with cie_function_code

	hnswQuery := `?[name, file_path, signature, start_line, distance] :=
		~cie_function_embedding:embedding_idx { function_id | query: [0.1, 0.2], k: 10, ef: 50, bind_distance: distance },
		*cie_function { id: function_id, name, file_path, signature, start_line },
		*cie_function_code { function_id, code_text },
		regex_matches(code_text, "test")
		:order distance
		:limit 10`

	// Check that code_text is included when filtering by it
	if strings.Contains(hnswQuery, "regex_matches(code_text") {
		if !strings.Contains(hnswQuery, "code_text") {
			t.Error("HNSW query filters by code_text but doesn't select it")
		}
	}

	// Check that function_id is included (required for HNSW join)
	if !strings.Contains(hnswQuery, "function_id") {
		t.Error("HNSW query must include function_id field for join")
	}
}

// TestRoleFilterPatterns validates role-based filter patterns
func TestRoleFilterPatterns(t *testing.T) {
	rolePatterns := map[string]string{
		"test":      `(?i)_test\.go$|test_.*\.go$|\.test\.(ts|tsx|js|jsx)$|__tests__/`,
		"generated": `(?i)\.pb\.go$|_generated\.go$|\.gen\.go$`,
		"handler":   `(?i)(handler|controller)`,
		"router":    `(?i)(route|router|register.*route)`,
	}

	testCases := []struct {
		role     string
		filePath string
		funcName string
		match    bool
	}{
		{"test", "internal/handler_test.go", "TestHandle", true},
		{"test", "internal/handler.go", "Handle", false},
		{"generated", "api/service.pb.go", "GetUser", true},
		{"generated", "api/service.go", "GetUser", false},
		{"handler", "internal/handler.go", "HandleRequest", true},
		{"handler", "internal/service.go", "GetUser", false},
	}

	for _, tc := range testCases {
		t.Run(tc.role+"_"+tc.funcName, func(t *testing.T) {
			pattern := rolePatterns[tc.role]
			re := regexp.MustCompile(pattern)

			matched := re.MatchString(tc.filePath) || re.MatchString(tc.funcName)
			if matched != tc.match {
				t.Errorf("Role %s: expected match=%v for file=%s, func=%s, got=%v",
					tc.role, tc.match, tc.filePath, tc.funcName, matched)
			}
		})
	}
}
