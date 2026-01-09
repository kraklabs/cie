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
	"strings"
	"testing"
)

func TestNewResult(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"simple text", "Hello, World!"},
		{"empty text", ""},
		{"multiline", "line1\nline2\nline3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult(tt.text)
			if result == nil {
				t.Fatal("NewResult returned nil")
			}
			if result.Text != tt.text {
				t.Errorf("NewResult().Text = %q, want %q", result.Text, tt.text)
			}
			if result.IsError {
				t.Error("NewResult().IsError should be false")
			}
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"error message", "Something went wrong"},
		{"empty error", ""},
		{"detailed error", "Error: query failed at line 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewError(tt.text)
			if result == nil {
				t.Fatal("NewError returned nil")
			}
			if result.Text != tt.text {
				t.Errorf("NewError().Text = %q, want %q", result.Text, tt.text)
			}
			if !result.IsError {
				t.Error("NewError().IsError should be true")
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "truncation needed",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
		{
			name:   "truncate to zero",
			input:  "hello",
			maxLen: 0,
			want:   "...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatQueryResult(t *testing.T) {
	tests := []struct {
		name   string
		result *QueryResult
		script string
		checks []string // substrings that should be present
	}{
		{
			name: "empty results",
			result: &QueryResult{
				Headers: []string{"name", "file_path"},
				Rows:    [][]any{},
			},
			script: `?[name] := *cie_function { name }`,
			checks: []string{"Found 0 results", "No results found", "Generated CozoScript"},
		},
		{
			name: "single result",
			result: &QueryResult{
				Headers: []string{"name", "file_path"},
				Rows:    [][]any{{"HandleRequest", "handler.go"}},
			},
			script: `?[name, file_path] := *cie_function { name, file_path }`,
			checks: []string{"Found 1 results", "Result 1", "name: HandleRequest", "file_path: handler.go"},
		},
		{
			name: "multiple results",
			result: &QueryResult{
				Headers: []string{"name", "line"},
				Rows: [][]any{
					{"Func1", float64(10)},
					{"Func2", float64(20)},
				},
			},
			script: "test query",
			checks: []string{"Found 2 results", "Result 1", "Result 2", "Func1", "Func2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQueryResult(tt.result, tt.script)
			for _, check := range tt.checks {
				if !strings.Contains(got, check) {
					t.Errorf("FormatQueryResult() should contain %q, got:\n%s", check, got)
				}
			}
		})
	}
}

func TestFormatQueryResult_LongValues(t *testing.T) {
	longValue := strings.Repeat("x", 300)
	result := &QueryResult{
		Headers: []string{"code"},
		Rows:    [][]any{{longValue}},
	}

	got := FormatQueryResult(result, "test")

	// Should be truncated to 200 chars + "..."
	if strings.Contains(got, longValue) {
		t.Error("FormatQueryResult() should truncate long values")
	}
	if !strings.Contains(got, "...") {
		t.Error("FormatQueryResult() should add ellipsis for truncated values")
	}
}

func TestFormatQueryResultSimple(t *testing.T) {
	tests := []struct {
		name   string
		result *QueryResult
		checks []string
	}{
		{
			name: "empty results",
			result: &QueryResult{
				Headers: []string{"name"},
				Rows:    [][]any{},
			},
			checks: []string{"No results found"},
		},
		{
			name: "with results",
			result: &QueryResult{
				Headers: []string{"name", "file"},
				Rows:    [][]any{{"HandleRequest", "handler.go"}},
			},
			checks: []string{"Found 1 results", "name: HandleRequest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQueryResultSimple(tt.result)
			for _, check := range tt.checks {
				if !strings.Contains(got, check) {
					t.Errorf("FormatQueryResultSimple() should contain %q, got:\n%s", check, got)
				}
			}
			// Should NOT contain script section
			if strings.Contains(got, "Generated CozoScript") {
				t.Error("FormatQueryResultSimple() should not contain script")
			}
		})
	}
}

func TestFormatQueryResultSimple_Truncation(t *testing.T) {
	// Create 25 rows
	rows := make([][]any, 25)
	for i := 0; i < 25; i++ {
		rows[i] = []any{"Func"}
	}

	result := &QueryResult{
		Headers: []string{"name"},
		Rows:    rows,
	}

	got := FormatQueryResultSimple(result)

	// Should contain "... and 5 more"
	if !strings.Contains(got, "5 more") {
		t.Errorf("FormatQueryResultSimple() should truncate at 20 rows, got:\n%s", got)
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123456, "123456"},
		{-1, "-1"},
		{-42, "-42"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := intToStr(tt.input)
			if got != tt.want {
				t.Errorf("intToStr(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAnyToStr(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"float64 integer", float64(10), "10"},
		{"float64 decimal", float64(3.5), "3.5"},
		{"float64 with trailing zeros", float64(3.50), "3.5"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"nil", nil, "null"},
		{"unknown type", struct{}{}, "<?>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := anyToStr(tt.input)
			if got != tt.want {
				t.Errorf("anyToStr(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFloatToStr(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{1.0, "1"},
		{3.14, "3.140000"},
		{-3.14, "-3.140000"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := floatToStr(tt.input)
			if got != tt.want {
				t.Errorf("floatToStr(%f) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRolePattern_Struct(t *testing.T) {
	// Verify RolePattern struct fields
	rp := RolePattern{
		FilePattern: ".*_test\\.go$",
		NamePattern: "^Test.*",
		CodePattern: "t\\.Run",
		Description: "Test functions",
	}

	if rp.FilePattern != ".*_test\\.go$" {
		t.Error("FilePattern not set correctly")
	}
	if rp.NamePattern != "^Test.*" {
		t.Error("NamePattern not set correctly")
	}
	if rp.CodePattern != "t\\.Run" {
		t.Error("CodePattern not set correctly")
	}
	if rp.Description != "Test functions" {
		t.Error("Description not set correctly")
	}
}
