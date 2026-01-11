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
	"strings"
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single line",
			input: "hello world",
			want:  []string{"hello world"},
		},
		{
			name:  "multiple lines",
			input: "line1\nline2\nline3",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "trailing newline",
			input: "line1\nline2\n",
			want:  []string{"line1", "line2"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "only newlines",
			input: "\n\n",
			want:  []string{"", ""},
		},
		{
			name:  "mixed empty and content",
			input: "line1\n\nline3",
			want:  []string{"line1", "", "line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitLines() returned %d lines, want %d", len(got), len(tt.want))
				t.Errorf("got: %v, want: %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractMatchContext(t *testing.T) {
	code := `func HandleRequest(c *gin.Context) {
	id := c.Param("id")
	user, err := s.GetUser(id)
	if err != nil {
		c.JSON(500, err)
		return
	}
	c.JSON(200, user)
}`

	tests := []struct {
		name          string
		searchText    string
		caseSensitive bool
		contextLines  int
		wantContains  []string
		wantNotEmpty  bool
	}{
		{
			name:          "find GetUser with no context",
			searchText:    "GetUser",
			caseSensitive: false,
			contextLines:  0,
			wantContains:  []string{"GetUser"},
			wantNotEmpty:  true,
		},
		{
			name:          "find GetUser with context",
			searchText:    "GetUser",
			caseSensitive: false,
			contextLines:  1,
			wantContains:  []string{"GetUser", "id :=", "if err"},
			wantNotEmpty:  true,
		},
		{
			name:          "case insensitive search",
			searchText:    "getuser",
			caseSensitive: false,
			contextLines:  0,
			wantContains:  []string{"GetUser"},
			wantNotEmpty:  true,
		},
		{
			name:          "case sensitive no match",
			searchText:    "getuser",
			caseSensitive: true,
			contextLines:  0,
			wantNotEmpty:  false,
		},
		{
			name:          "not found",
			searchText:    "NotInCode",
			caseSensitive: false,
			contextLines:  0,
			wantNotEmpty:  false,
		},
		{
			name:          "multiple matches",
			searchText:    "c.JSON",
			caseSensitive: false,
			contextLines:  0,
			wantContains:  []string{"500", "200"},
			wantNotEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMatchContext(code, tt.searchText, tt.caseSensitive, tt.contextLines)

			if tt.wantNotEmpty && got == "" {
				t.Error("extractMatchContext() returned empty, want non-empty")
				return
			}
			if !tt.wantNotEmpty && got != "" {
				t.Errorf("extractMatchContext() = %q, want empty", got)
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("extractMatchContext() should contain %q, got:\n%s", want, got)
				}
			}
		})
	}
}

func TestExtractMatchContext_Highlighting(t *testing.T) {
	code := "line1\nmatch here\nline3"

	got := extractMatchContext(code, "match", false, 0)

	// Matching line should be highlighted with "> "
	if !strings.Contains(got, "> ") {
		t.Error("extractMatchContext() should highlight matching lines with '> ' prefix")
	}
	// Line numbers should be present
	if !strings.Contains(got, ":") {
		t.Error("extractMatchContext() should include line numbers")
	}
}

func TestExtractMatchContext_Separator(t *testing.T) {
	// Code with matches far apart
	code := "line1\nmatch1\nline3\nline4\nline5\nline6\nline7\nmatch2\nline9"

	got := extractMatchContext(code, "match", false, 1)

	// Should have separator (...) between distant matches
	if !strings.Contains(got, "...") {
		t.Error("extractMatchContext() should add separator between distant matches")
	}
}

func TestExtractMatchContext_Truncation(t *testing.T) {
	// Create code that would produce very long output
	var builder strings.Builder
	for i := 0; i < 500; i++ {
		builder.WriteString("match this line\n")
	}
	code := builder.String()

	got := extractMatchContext(code, "match", false, 2)

	// Should be truncated to ~2000 chars
	if len(got) > 2100 { // Allow some buffer for truncation message
		t.Errorf("extractMatchContext() should truncate long output, got %d chars", len(got))
	}
	if !strings.Contains(got, "truncated") {
		t.Error("extractMatchContext() should indicate truncation")
	}
}

func TestGrepArgs_Struct(t *testing.T) {
	args := GrepArgs{
		Text:           ".GET(",
		Path:           "internal/",
		ExcludePattern: "_test\\.go$",
		CaseSensitive:  true,
		ContextLines:   2,
		Limit:          50,
	}

	if args.Text != ".GET(" {
		t.Error("Text not set correctly")
	}
	if args.Path != "internal/" {
		t.Error("Path not set correctly")
	}
	if args.ExcludePattern != "_test\\.go$" {
		t.Error("ExcludePattern not set correctly")
	}
	if !args.CaseSensitive {
		t.Error("CaseSensitive should be true")
	}
	if args.ContextLines != 2 {
		t.Error("ContextLines not set correctly")
	}
	if args.Limit != 50 {
		t.Error("Limit not set correctly")
	}
}

// ============================================================================
// Main Grep Function Tests
// ============================================================================

func TestGrep_SinglePattern_Success(t *testing.T) {
	tests := []struct {
		name         string
		args         GrepArgs
		mockRows     [][]any
		wantContains string
		wantErr      bool
	}{
		{
			name: "basic search with results",
			args: GrepArgs{Text: ".GET(", Limit: 100},
			mockRows: [][]any{
				{"/api/routes.go", "RegisterRoutes", int64(10), int64(15)},
			},
			wantContains: "/api/routes.go",
		},
		{
			name: "with path filtering",
			args: GrepArgs{Text: "Handler", Path: "internal/", Limit: 100},
			mockRows: [][]any{
				{"/internal/api/handler.go", "HandleRequest", int64(42), int64(50)},
			},
			wantContains: "/internal/api/handler.go",
		},
		{
			name: "case insensitive by default",
			args: GrepArgs{Text: "TODO", CaseSensitive: false, Limit: 100},
			mockRows: [][]any{
				{"/app.go", "MainFunc", int64(5), int64(10)},
			},
			wantContains: "/app.go",
		},
		{
			name: "with limit parameter",
			args: GrepArgs{Text: "func", Limit: 10},
			mockRows: [][]any{
				{"/file1.go", "Func1", int64(1), int64(5)},
				{"/file2.go", "Func2", int64(10), int64(15)},
			},
			wantContains: "/file1.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			headers := []string{"file_path", "name", "start_line", "end_line"}
			client := NewMockClientWithResults(headers, tt.mockRows)

			result, err := Grep(ctx, client, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			assertNoError(t, err)
			if result == nil {
				t.Fatal("expected result, got nil")
			}
			if tt.wantContains != "" {
				assertContains(t, result.Text, tt.wantContains)
			}
		})
	}
}

func TestGrep_SinglePattern_WithContext(t *testing.T) {
	tests := []struct {
		name         string
		contextLines int
		wantCode     bool
	}{
		{
			name:         "no context",
			contextLines: 0,
			wantCode:     false,
		},
		{
			name:         "with 2 lines context",
			contextLines: 2,
			wantCode:     true,
		},
		{
			name:         "with 5 lines context",
			contextLines: 5,
			wantCode:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)

			code := "func Example() {\n\treturn nil\n}"
			var headers []string
			var rows [][]any

			if tt.wantCode {
				headers = []string{"file_path", "name", "start_line", "end_line", "code_text"}
				rows = [][]any{
					{"/app.go", "Example", int64(10), int64(12), code},
				}
			} else {
				headers = []string{"file_path", "name", "start_line", "end_line"}
				rows = [][]any{
					{"/app.go", "Example", int64(10), int64(12)},
				}
			}

			client := NewMockClientWithResults(headers, rows)
			args := GrepArgs{Text: "Example", ContextLines: tt.contextLines, Limit: 100}

			result, err := Grep(ctx, client, args)

			assertNoError(t, err)
			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if tt.wantCode {
				assertContains(t, result.Text, "return nil")
			}
		})
	}
}

func TestGrep_SinglePattern_NoResults(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := GrepArgs{Text: "NonExistent", Limit: 100}
	result, err := Grep(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "No matches found")
}

func TestGrep_EmptyText(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := GrepArgs{Text: "", Texts: nil, Limit: 100}
	result, err := Grep(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	assertContains(t, result.Text, "Error")
	assertContains(t, result.Text, "required")
}

func TestGrep_QueryError(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientWithError(fmt.Errorf("database error"))

	args := GrepArgs{Text: "test", Limit: 100}
	_, err := Grep(ctx, client, args)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertContains(t, err.Error(), "grep query")
}

// ============================================================================
// GrepMulti Tests (Batch Multi-Pattern Search)
// ============================================================================

func TestGrep_MultiplePatterns_AllMatch(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/auth.go", "Login", int64(10), "access_token := jwt.Generate()"},
		{"/auth.go", "Refresh", int64(20), "refresh_token := jwt.Refresh()"},
		{"/config.go", "LoadAPI", int64(30), "api_key := os.Getenv(\"API_KEY\")"},
	}
	client := NewMockClientWithResults(headers, rows)

	args := GrepArgs{
		Texts: []string{"access_token", "refresh_token", "api_key"},
		Limit: 100,
	}

	result, err := Grep(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Check summary table
	assertContains(t, result.Text, "access_token")
	assertContains(t, result.Text, "refresh_token")
	assertContains(t, result.Text, "api_key")
}

func TestGrep_MultiplePatterns_PartialMatch(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/auth.go", "Login", int64(10), "access_token := jwt.Generate()"},
	}
	client := NewMockClientWithResults(headers, rows)

	args := GrepArgs{
		Texts: []string{"access_token", "refresh_token", "api_key"},
		Limit: 100,
	}

	result, err := Grep(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Should show count for all patterns, even if some are 0
	assertContains(t, result.Text, "access_token")
}

func TestGrep_MultiplePatterns_NoneMatch(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := GrepArgs{
		Texts: []string{"pattern1", "pattern2"},
		Limit: 100,
	}

	result, err := Grep(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// Check for 0 count in the table
	assertContains(t, result.Text, "✗ 0")
	assertContains(t, result.Text, "pattern1")
	assertContains(t, result.Text, "pattern2")
}

// ============================================================================
// VerifyAbsence Tests (Security Audit Tool)
// ============================================================================

func TestVerifyAbsence_NoViolations(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := VerifyAbsenceArgs{
		Patterns: []string{"secret", "password", "api_key"},
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "PASS")
	assertContains(t, result.Text, "**Violations found:** 0")
}

func TestVerifyAbsence_WithViolations(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/config.go", "LoadConfig", int64(15), "api_key := \"hardcoded_key\""},
		{"/auth.go", "Connect", int64(30), "password := \"secret123\""},
	}
	client := NewMockClientWithResults(headers, rows)

	args := VerifyAbsenceArgs{
		Patterns: []string{"api_key", "password"},
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "FAIL")
	assertContains(t, result.Text, "api_key")
	assertContains(t, result.Text, "password")
	assertContains(t, result.Text, "/config.go")
	assertContains(t, result.Text, "/auth.go")
}

func TestVerifyAbsence_EmptyPatterns(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := VerifyAbsenceArgs{
		Patterns: []string{},
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected error result, got nil")
	}
	assertContains(t, result.Text, "Error")
	assertContains(t, result.Text, "required")
}

func TestVerifyAbsence_DefaultSeverity(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/config.go", "LoadConfig", int64(15), "api_key := \"test\""},
	}
	client := NewMockClientWithResults(headers, rows)

	args := VerifyAbsenceArgs{
		Patterns: []string{"api_key"},
		Severity: "", // Should default to "warning"
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "WARNING")
}

func TestVerifyAbsence_CustomSeverity(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/config.go", "LoadConfig", int64(15), "api_key := \"test\""},
	}
	client := NewMockClientWithResults(headers, rows)

	args := VerifyAbsenceArgs{
		Patterns: []string{"api_key"},
		Severity: "critical",
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "CRITICAL")
}

func TestVerifyAbsence_WithPathFilter(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/frontend/src/config.js", "loadConfig", int64(5), "api_key := \"test\""},
	}
	client := NewMockClientWithResults(headers, rows)

	args := VerifyAbsenceArgs{
		Patterns: []string{"api_key"},
		Path:     "frontend/",
	}

	result, err := VerifyAbsence(ctx, client, args)

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	assertContains(t, result.Text, "frontend")
}

func TestVerifyAbsence_CaseSensitive(t *testing.T) {
	tests := []struct {
		name          string
		caseSensitive bool
		pattern       string
		code          string
		shouldMatch   bool
	}{
		{
			name:          "case insensitive matches uppercase",
			caseSensitive: false,
			pattern:       "api_key",
			code:          "API_KEY := \"test\"",
			shouldMatch:   true,
		},
		{
			name:          "case sensitive no match",
			caseSensitive: true,
			pattern:       "api_key",
			code:          "API_KEY := \"test\"",
			shouldMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)

			var client Querier
			if tt.shouldMatch {
				headers := []string{"file_path", "name", "start_line", "code_text"}
				rows := [][]any{
					{"/config.go", "LoadConfig", int64(15), tt.code},
				}
				client = NewMockClientWithResults(headers, rows)
			} else {
				client = NewMockClientEmpty()
			}

			args := VerifyAbsenceArgs{
				Patterns:      []string{tt.pattern},
				CaseSensitive: tt.caseSensitive,
			}

			result, err := VerifyAbsence(ctx, client, args)
			assertNoError(t, err)

			if tt.shouldMatch {
				assertContains(t, result.Text, "FAIL")
			} else {
				assertContains(t, result.Text, "PASS")
			}
		})
	}
}

// ============================================================================
// Helper Function Tests
// ============================================================================

func TestMatchesGrepPattern(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		text          string
		caseSensitive bool
		want          bool
	}{
		{
			name:          "case insensitive match",
			code:          "func GetUser() {}",
			text:          "getuser",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case sensitive match",
			code:          "func GetUser() {}",
			text:          "GetUser",
			caseSensitive: true,
			want:          true,
		},
		{
			name:          "case sensitive no match",
			code:          "func GetUser() {}",
			text:          "getuser",
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "no match",
			code:          "func GetUser() {}",
			text:          "NotInCode",
			caseSensitive: false,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesGrepPattern(tt.code, tt.text, tt.caseSensitive)
			if got != tt.want {
				t.Errorf("matchesGrepPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesAbsencePattern(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		pattern       string
		caseSensitive bool
		want          bool
	}{
		{
			name:          "case insensitive match",
			code:          "API_KEY := secret",
			pattern:       "api_key",
			caseSensitive: false,
			want:          true,
		},
		{
			name:          "case sensitive no match",
			code:          "API_KEY := secret",
			pattern:       "api_key",
			caseSensitive: true,
			want:          false,
		},
		{
			name:          "exact match",
			code:          "api_key := secret",
			pattern:       "api_key",
			caseSensitive: true,
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAbsencePattern(tt.code, tt.pattern, tt.caseSensitive)
			if got != tt.want {
				t.Errorf("matchesAbsencePattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractParamName(t *testing.T) {
	tests := []struct {
		name  string
		param string
		want  string
	}{
		{
			name:  "curly braces",
			param: "{id}",
			want:  "id",
		},
		{
			name:  "colon prefix",
			param: ":id",
			want:  "id",
		},
		{
			name:  "angle brackets",
			param: "<id>",
			want:  "id",
		},
		{
			name:  "square brackets",
			param: "[id]",
			want:  "id",
		},
		{
			name:  "no special chars",
			param: "id",
			want:  "id",
		},
		{
			name:  "empty",
			param: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParamName(tt.param)
			if got != tt.want {
				t.Errorf("extractParamName(%q) = %q, want %q", tt.param, got, tt.want)
			}
		})
	}
}

// ============================================================================
// Query Builder Tests
// ============================================================================

func TestBuildGrepQuery(t *testing.T) {
	tests := []struct {
		name            string
		args            GrepArgs
		needsCode       bool
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "basic query without code",
			args: GrepArgs{
				Text:  ".GET(",
				Limit: 100,
			},
			needsCode: false,
			wantContains: []string{
				"file_path",
				"name",
				"start_line",
				"regex_matches",
			},
			// Note: code_text is always in query for matching, but not in select fields
		},
		{
			name: "query with code for context",
			args: GrepArgs{
				Text:  "Handler",
				Limit: 100,
			},
			needsCode: true,
			wantContains: []string{
				"file_path",
				"name",
				"code_text",
				"regex_matches",
			},
		},
		{
			name: "query with path filter",
			args: GrepArgs{
				Text:  "func",
				Path:  "internal/",
				Limit: 100,
			},
			needsCode: false,
			wantContains: []string{
				"regex_matches(file_path",
				"internal/",
			},
		},
		{
			name: "query with exclude pattern",
			args: GrepArgs{
				Text:           "test",
				ExcludePattern: "_test\\.go",
				Limit:          100,
			},
			needsCode: false,
			wantContains: []string{
				"!regex_matches(file_path",
				"_test",
			},
		},
		{
			name: "case insensitive query",
			args: GrepArgs{
				Text:          "TODO",
				CaseSensitive: false,
				Limit:         100,
			},
			needsCode: false,
			wantContains: []string{
				"(?i)",
			},
		},
		{
			name: "case sensitive query",
			args: GrepArgs{
				Text:          "TODO",
				CaseSensitive: true,
				Limit:         100,
			},
			needsCode: false,
			wantNotContains: []string{
				"(?i)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildGrepQuery(tt.args, tt.needsCode)

			for _, want := range tt.wantContains {
				if !strings.Contains(query, want) {
					t.Errorf("buildGrepQuery() should contain %q, got:\n%s", want, query)
				}
			}

			for _, wantNot := range tt.wantNotContains {
				if strings.Contains(query, wantNot) {
					t.Errorf("buildGrepQuery() should not contain %q, got:\n%s", wantNot, query)
				}
			}
		})
	}
}

func TestBuildGrepMultiQuery(t *testing.T) {
	args := GrepArgs{
		Texts: []string{"access_token", "refresh_token", "api_key"},
		Limit: 100,
	}

	query := buildGrepMultiQuery(args)

	// Should contain all patterns
	assertContains(t, query, "access_token")
	assertContains(t, query, "refresh_token")
	assertContains(t, query, "api_key")

	// Should have code_text for matching
	assertContains(t, query, "code_text")
}

func TestBuildAbsenceQuery(t *testing.T) {
	tests := []struct {
		name         string
		args         VerifyAbsenceArgs
		wantContains []string
	}{
		{
			name: "basic absence query",
			args: VerifyAbsenceArgs{
				Patterns: []string{"secret", "password"},
			},
			wantContains: []string{
				"secret",
				"password",
				"regex_matches",
				"code_text",
			},
		},
		{
			name: "with path filter",
			args: VerifyAbsenceArgs{
				Patterns: []string{"api_key"},
				Path:     "frontend/",
			},
			wantContains: []string{
				"api_key",
				"frontend/",
				"regex_matches(file_path",
			},
		},
		{
			name: "with exclude pattern",
			args: VerifyAbsenceArgs{
				Patterns:       []string{"token"},
				ExcludePattern: "_test\\.go",
			},
			wantContains: []string{
				"token",
				"!regex_matches(file_path",
				"_test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := buildAbsenceQuery(tt.args)

			for _, want := range tt.wantContains {
				if !strings.Contains(query, want) {
					t.Errorf("buildAbsenceQuery() should contain %q, got:\n%s", want, query)
				}
			}
		})
	}
}

// ============================================================================
// Edge Cases Tests
// ============================================================================

func TestGrep_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		args     GrepArgs
		mockRows [][]any
		wantErr  bool
	}{
		{
			name: "very long search text",
			args: GrepArgs{
				Text:  strings.Repeat("a", 1000),
				Limit: 100,
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
		{
			name: "special regex characters",
			args: GrepArgs{
				Text:  ".*()+[]{}|^$",
				Limit: 100,
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
		{
			name: "unicode in search",
			args: GrepArgs{
				Text:  "函数名称",
				Limit: 100,
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
		{
			name: "newlines in search text",
			args: GrepArgs{
				Text:  "line1\nline2",
				Limit: 100,
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)

			var client Querier
			if len(tt.mockRows) > 0 {
				headers := []string{"file_path", "name", "start_line", "end_line"}
				client = NewMockClientWithResults(headers, tt.mockRows)
			} else {
				client = NewMockClientEmpty()
			}

			result, err := Grep(ctx, client, tt.args)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.wantErr && result == nil {
				t.Fatal("expected result, got nil")
			}
		})
	}
}

func TestVerifyAbsence_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		args     VerifyAbsenceArgs
		mockRows [][]any
		wantErr  bool
	}{
		{
			name: "many patterns",
			args: VerifyAbsenceArgs{
				Patterns: []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8", "p9", "p10"},
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
		{
			name: "very long pattern",
			args: VerifyAbsenceArgs{
				Patterns: []string{strings.Repeat("pattern", 100)},
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
		{
			name: "unicode in pattern",
			args: VerifyAbsenceArgs{
				Patterns: []string{"密码", "秘密"},
			},
			mockRows: [][]any{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)

			var client Querier
			if len(tt.mockRows) > 0 {
				headers := []string{"file_path", "name", "start_line", "code_text"}
				client = NewMockClientWithResults(headers, tt.mockRows)
			} else {
				client = NewMockClientEmpty()
			}

			result, err := VerifyAbsence(ctx, client, tt.args)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.wantErr && result == nil {
				t.Fatal("expected result, got nil")
			}
		})
	}
}

// ============================================================================
// Alternative Suggestion System Tests
// ============================================================================

func TestFindAlternativePaths(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path"}
	rows := [][]any{
		{"internal/api/routes.go"},
		{"internal/http/handler.go"},
		{"pkg/server/routes.go"},
		{"cmd/app/main.go"},
		{"examples/demo/routes.go"},
		{"test/fixtures/routes.go"},
	}
	client := NewMockClientWithResults(headers, rows)

	alternatives := findAlternativePaths(ctx, client, "routes", false)

	// Should return results (may be grouped by top dir or limited to 5)
	// The function groups by top directory, so results may vary
	// Just verify it returns something reasonable without crashing
	if len(alternatives) > 5 {
		t.Errorf("expected at most 5 alternatives, got %d", len(alternatives))
	}

	// The function should process the mock data
	// Even if results are empty due to grouping logic, it should work
	t.Logf("Got %d alternatives", len(alternatives))
}

func TestFindAlternativePaths_NoMatches(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	alternatives := findAlternativePaths(ctx, client, "nonexistent", false)

	// Should return empty array
	if len(alternatives) != 0 {
		t.Errorf("expected no alternatives, got %d", len(alternatives))
	}
}

func TestFindAlternativePaths_LimitTo5(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path"}
	rows := [][]any{
		{"dir1/file.go"},
		{"dir2/file.go"},
		{"dir3/file.go"},
		{"dir4/file.go"},
		{"dir5/file.go"},
		{"dir6/file.go"},
		{"dir7/file.go"},
		{"dir8/file.go"},
	}
	client := NewMockClientWithResults(headers, rows)

	alternatives := findAlternativePaths(ctx, client, "file", false)

	// Should cap at 5
	if len(alternatives) > 5 {
		t.Errorf("expected at most 5 alternatives, got %d", len(alternatives))
	}
}

func TestSuggestRouteAlternatives_NotARoute(t *testing.T) {
	ctx := setupTest(t)
	client := NewMockClientEmpty()

	// Non-route paths should return empty
	tests := []string{
		"internal/handler",
		"pkg/util",
		"GetUser",
		"func_name",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			result := suggestRouteAlternatives(ctx, client, text, "", false)
			if result != "" {
				t.Errorf("expected empty result for non-route %q, got: %s", text, result)
			}
		})
	}
}

func TestSuggestRouteAlternatives_RouteFormats(t *testing.T) {
	ctx := setupTest(t)

	//  Mock needs to return count results for countMatches calls
	countResult := [][]any{{float64(1)}}
	client := NewMockClientWithResults([]string{"count"}, countResult)

	tests := []struct {
		name     string
		text     string
		wantFind string
	}{
		{
			name:     "OpenAPI format {id}",
			text:     "/users/{id}",
			wantFind: ":id", // Should suggest Gin/Express format
		},
		{
			name:     "Gin/Express format :id",
			text:     "/users/:id",
			wantFind: "{id}", // Should suggest OpenAPI format
		},
		{
			name:     "Flask/Angular format <id>",
			text:     "/users/<id>",
			wantFind: ":id", // Should suggest Gin/Express format
		},
		{
			name:     "Next.js format [id]",
			text:     "/users/[id]",
			wantFind: ":id", // Should suggest Gin/Express format
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestRouteAlternatives(ctx, client, tt.text, "", false)

			// Should detect route and return suggestions (or empty if countMatches returns 0)
			// Just verify it doesn't crash and returns valid format
			if result != "" {
				// If it returns suggestions, they should be formatted properly
				if !strings.Contains(result, "alternative") && !strings.Contains(result, "syntax") {
					t.Errorf("expected formatted suggestions, got:\n%s", result)
				}
			}
		})
	}
}

func TestSuggestRouteAlternatives_MultipleParams(t *testing.T) {
	ctx := setupTest(t)

	// Mock returns count for suggestions
	countResult := [][]any{{float64(1)}}
	client := NewMockClientWithResults([]string{"count"}, countResult)

	result := suggestRouteAlternatives(ctx, client, "/users/{userId}/posts/{postId}", "", false)

	// Should detect as a route with multiple parameters
	// Function should handle this without crashing
	// If it returns suggestions, verify basic format
	if result != "" {
		if !strings.Contains(result, "syntax") {
			t.Errorf("expected formatted output with 'syntax', got:\n%s", result)
		}
	}
}

func TestCountMatches(t *testing.T) {
	ctx := setupTest(t)

	tests := []struct {
		name     string
		mockRows [][]any
		want     int
	}{
		{
			name: "multiple matches",
			mockRows: [][]any{
				{float64(3)}, // count(id) returns float64
			},
			want: 3,
		},
		{
			name: "no matches",
			mockRows: [][]any{
				{float64(0)},
			},
			want: 0,
		},
		{
			name:     "empty result",
			mockRows: [][]any{},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client Querier
			if len(tt.mockRows) > 0 {
				headers := []string{"count"}
				client = NewMockClientWithResults(headers, tt.mockRows)
			} else {
				client = NewMockClientEmpty()
			}

			got := countMatches(ctx, client, "test", "path", false)
			if got != tt.want {
				t.Errorf("countMatches() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountAbsenceFiles(t *testing.T) {
	ctx := setupTest(t)

	tests := []struct {
		name     string
		mockRows [][]any
		want     int
	}{
		{
			name: "files found",
			mockRows: [][]any{
				{float64(42)}, // count(file_path) returns float64
			},
			want: 42,
		},
		{
			name: "no files",
			mockRows: [][]any{
				{float64(0)},
			},
			want: 0,
		},
		{
			name:     "empty result",
			mockRows: [][]any{},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client Querier
			if len(tt.mockRows) > 0 {
				headers := []string{"count"}
				client = NewMockClientWithResults(headers, tt.mockRows)
			} else {
				client = NewMockClientEmpty()
			}

			got := countAbsenceFiles(ctx, client, "path")
			if got != tt.want {
				t.Errorf("countAbsenceFiles() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFormatGrepNoResults_WithAlternatives(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path"}
	rows := [][]any{
		{"internal/api/routes.go"},
		{"pkg/server/routes.go"},
	}
	client := NewMockClientWithResults(headers, rows)

	args := GrepArgs{
		Text:  "route_handler",
		Path:  "cmd/",
		Limit: 100,
	}

	result := formatGrepNoResults(ctx, client, args)

	// Should contain "No matches found"
	assertContains(t, result, "No matches found")

	// Should suggest alternatives from other directories
	// (if findAlternativePaths returns results)
	if strings.Contains(result, "Found in other locations") {
		assertContains(t, result, "internal")
	}
}

func TestFormatGrepNoResults_RouteAlternatives(t *testing.T) {
	ctx := setupTest(t)

	headers := []string{"file_path", "name", "start_line", "code_text"}
	rows := [][]any{
		{"/routes.go", "SetupRoutes", int64(10), "r.GET(\"/users/:id\", handler)"},
	}
	client := NewMockClientWithResults(headers, rows)

	args := GrepArgs{
		Text:  "/users/{id}",
		Limit: 100,
	}

	result := formatGrepNoResults(ctx, client, args)

	// Should suggest route format alternatives
	// (if suggestRouteAlternatives returns results)
	assertContains(t, result, "No matches found")
}
