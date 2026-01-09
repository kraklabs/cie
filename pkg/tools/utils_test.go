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

package tools

import (
	"testing"
)

func TestEscapeRegex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "dot",
			input: "file.go",
			want:  "file[.]go",
		},
		{
			name:  "asterisk",
			input: "a*b",
			want:  "a[*]b",
		},
		{
			name:  "plus",
			input: "a+b",
			want:  "a[+]b",
		},
		{
			name:  "question mark",
			input: "a?b",
			want:  "a[?]b",
		},
		{
			name:  "brackets",
			input: "[a]",
			want:  "[[]a[]]",
		},
		{
			name:  "parentheses",
			input: "(a)",
			want:  "[(]a[)]",
		},
		{
			name:  "curly braces",
			input: "{a}",
			want:  "[{]a[}]",
		},
		{
			name:  "caret",
			input: "^a$",
			want:  "[^]a[$]",
		},
		{
			name:  "pipe",
			input: "a|b",
			want:  "a[|]b",
		},
		{
			name:  "backslash",
			input: `a\b`,
			want:  `a[\]b`,
		},
		{
			name:  "complex pattern",
			input: "func (*Server).GET()",
			want:  "func [(][*]Server[)][.]GET[(][)]",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeRegex(tt.input)
			if got != tt.want {
				t.Errorf("EscapeRegex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuoteCozoPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{
			name:    "simple pattern",
			pattern: "hello",
			want:    `___"hello"___`,
		},
		{
			name:    "pattern with quotes",
			pattern: `say "hello"`,
			want:    `___"say "hello""___`,
		},
		{
			name:    "pattern with special chars",
			pattern: `func.*\(.*\)`,
			want:    `___"func.*\(.*\)"___`,
		},
		{
			name:    "empty pattern",
			pattern: "",
			want:    `___""___`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuoteCozoPattern(tt.pattern)
			if got != tt.want {
				t.Errorf("QuoteCozoPattern(%q) = %q, want %q", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestExtractFileName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/service/handler.go", "handler.go"},
		{"handler.go", "handler.go"},
		{"/absolute/path/to/file.txt", "file.txt"},
		{"a/b/c/d/e/f.go", "f.go"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExtractFileName(tt.path)
			if got != tt.want {
				t.Errorf("ExtractFileName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/service/handler.go", "internal/service"},
		{"handler.go", "."},
		{"/absolute/path/to/file.txt", "/absolute/path/to"},
		{"a/b/c/d/e/f.go", "a/b/c/d/e"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExtractDir(tt.path)
			if got != tt.want {
				t.Errorf("ExtractDir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractTopDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"internal/service/handler.go", "internal/service"},
		{"apps/gateway/routes/api.go", "apps/gateway"},
		{"cmd/server/main.go", "cmd/server"},
		{"handler.go", "."},
		{"single/file.go", "single"}, // only first segment before /
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExtractTopDir(tt.path)
			if got != tt.want {
				t.Errorf("ExtractTopDir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFormatRows(t *testing.T) {
	tests := []struct {
		name string
		rows [][]any
		want string
	}{
		{
			name: "empty rows",
			rows: [][]any{},
			want: "_No results_\n",
		},
		{
			name: "single column",
			rows: [][]any{{"handler.go"}},
			want: "- `handler.go`\n",
		},
		{
			name: "two columns",
			rows: [][]any{{"HandleRequest", "handler.go"}},
			want: "- `HandleRequest` in `handler.go`\n",
		},
		{
			name: "three columns",
			rows: [][]any{{"HandleRequest", "handler.go", 10}},
			want: "- `HandleRequest` in `handler.go:10`\n",
		},
		{
			name: "multiple rows",
			rows: [][]any{
				{"Func1", "file1.go", 1},
				{"Func2", "file2.go", 2},
			},
			want: "- `Func1` in `file1.go:1`\n- `Func2` in `file2.go:2`\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatRows(tt.rows)
			if got != tt.want {
				t.Errorf("FormatRows() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRows_Truncation(t *testing.T) {
	// Create 25 rows
	rows := make([][]any, 25)
	for i := 0; i < 25; i++ {
		rows[i] = []any{"Func", "file.go", i}
	}

	got := FormatRows(rows)

	// Should contain "... and 5 more"
	if !ContainsStr(got, "... and 5 more") {
		t.Errorf("FormatRows() should truncate at 20 rows, got: %s", got)
	}
}

func TestContainsStr(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"hello", "hello", true},
		{"hello", "helloworld", false},
		{"", "", true},
		{"hello", "", true},
		{"", "hello", false},
	}

	for _, tt := range tests {
		name := tt.s + "_" + tt.substr
		t.Run(name, func(t *testing.T) {
			got := ContainsStr(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("ContainsStr(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s       string
		substrs []string
		want    bool
	}{
		{"hello world", []string{"foo", "world"}, true},
		{"hello world", []string{"foo", "bar"}, false},
		{"hello world", []string{}, false},
		{"hello", []string{"hello"}, true},
		{"", []string{"foo"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := ContainsAny(tt.s, tt.substrs)
			if got != tt.want {
				t.Errorf("ContainsAny(%q, %v) = %v, want %v", tt.s, tt.substrs, got, tt.want)
			}
		})
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"hello", "hello"},
		{"123ABC", "123abc"},
		{"", ""},
		{"MixedCASE123", "mixedcase123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToLower(tt.input)
			if got != tt.want {
				t.Errorf("ToLower(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuery2Lower(t *testing.T) {
	// Query2Lower should behave identically to ToLower
	tests := []struct {
		input string
		want  string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"123ABC", "123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Query2Lower(tt.input)
			if got != tt.want {
				t.Errorf("Query2Lower(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractKeyTerms(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "simple query",
			query: "authentication handler",
			want:  []string{"authentication", "handler"},
		},
		{
			name:  "filters stop words",
			query: "how does the user login work",
			want:  []string{"user", "login", "work"},
		},
		{
			name:  "filters short words",
			query: "a is to be",
			want:  nil,
		},
		{
			name:  "handles punctuation",
			query: "find, search. query",
			want:  []string{"query"}, // "find" and "search" are stop words, "query" is not
		},
		{
			name:  "max 5 terms",
			query: "term1 term2 term3 term4 term5 term6 term7",
			want:  []string{"term1", "term2", "term3", "term4", "term5"},
		},
		{
			name:  "empty query",
			query: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractKeyTerms(tt.query)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractKeyTerms(%q) = %v, want %v", tt.query, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractKeyTerms(%q)[%d] = %q, want %q", tt.query, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAnyToString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"int64", int64(123), "123"},
		{"float64 integer", float64(10), "10"},
		{"float64 decimal", float64(3.14159), "3.14"},
		{"nil", nil, ""},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnyToString(tt.input)
			if got != tt.want {
				t.Errorf("AnyToString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
