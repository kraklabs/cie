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
