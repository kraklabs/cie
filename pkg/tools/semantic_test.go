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
	"testing"
)

func TestMatchesRoleFilter(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		role     string
		want     bool
	}{
		// source role
		{"source: normal file", "internal/handler.go", "source", true},
		{"source: test file", "internal/handler_test.go", "source", false},
		{"source: pb.go generated", "api/message.pb.go", "source", false},
		{"source: _generated file", "internal/types_generated.go", "source", false},
		{"source: tsx file", "src/component.tsx", "source", true},
		{"source: tsx test file", "src/component.test.tsx", "source", false},

		// test role
		{"test: test file", "internal/handler_test.go", "test", true},
		{"test: __tests__ dir", "src/__tests__/component.ts", "test", true},
		{"test: tests dir", "tests/integration/test.py", "test", true},
		{"test: normal file", "internal/handler.go", "test", false},

		// generated role
		{"generated: pb.go", "api/message.pb.go", "generated", true},
		{"generated: _generated", "types_generated.go", "generated", true},
		{"generated: .gen.go", "models.gen.go", "generated", true},
		{"generated: /generated/", "out/generated/types.go", "generated", true},
		{"generated: normal file", "internal/handler.go", "generated", false},

		// any role
		{"any: normal file", "internal/handler.go", "any", true},
		{"any: test file", "internal/handler_test.go", "any", true},
		{"any: generated file", "api/message.pb.go", "any", true},

		// empty role (defaults to source behavior)
		{"empty: normal file", "internal/handler.go", "", true},
		{"empty: test file", "internal/handler_test.go", "", false},

		// specialized roles
		{"router: normal file", "internal/routes.go", "router", true},
		{"router: test file", "internal/routes_test.go", "router", false},
		{"handler: normal file", "internal/handler.go", "handler", true},
		{"handler: test file", "internal/handler_test.go", "handler", false},
		{"entry_point: main file", "cmd/server/main.go", "entry_point", true},
		{"entry_point: test file", "cmd/server/main_test.go", "entry_point", false},

		// unknown role (defaults to source)
		{"unknown: normal file", "internal/handler.go", "unknown_role", true},
		{"unknown: test file", "internal/handler_test.go", "unknown_role", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesRoleFilter(tt.filePath, tt.role)
			if got != tt.want {
				t.Errorf("MatchesRoleFilter(%q, %q) = %v, want %v", tt.filePath, tt.role, got, tt.want)
			}
		})
	}
}

func TestRoleFilters(t *testing.T) {
	tests := []struct {
		role        string
		wantNonNil  bool
		wantMinLen  int
		wantContain string // should contain this substring
	}{
		{"source", true, 2, "negate"},
		{"test", true, 1, "regex_matches"},
		{"generated", true, 1, "pb"},
		{"entry_point", true, 2, "main"},
		{"router", true, 2, "GET"},
		{"handler", true, 2, "Handler"},
		{"any", false, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := RoleFilters(tt.role)
			if tt.wantNonNil && got == nil {
				t.Error("RoleFilters returned nil, expected non-nil")
				return
			}
			if !tt.wantNonNil && got != nil {
				t.Errorf("RoleFilters returned %v, expected nil", got)
				return
			}
			if tt.wantMinLen > 0 && len(got) < tt.wantMinLen {
				t.Errorf("RoleFilters returned %d conditions, want at least %d", len(got), tt.wantMinLen)
			}
			if tt.wantContain != "" {
				found := false
				for _, condition := range got {
					if ContainsStr(condition, tt.wantContain) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("RoleFilters conditions should contain %q, got: %v", tt.wantContain, got)
				}
			}
		})
	}
}

func TestRoleFiltersForHNSW(t *testing.T) {
	tests := []struct {
		role        string
		wantEmpty   bool
		wantContain string
	}{
		{"source", false, "!regex_matches"},
		{"test", false, "regex_matches"},
		{"generated", false, "pb"},
		{"entry_point", false, "main"},
		{"router", false, "RegisterRoutes"},
		{"handler", false, "Handler"},
		{"any", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := RoleFiltersForHNSW(tt.role)
			if tt.wantEmpty && got != "" {
				t.Errorf("RoleFiltersForHNSW(%q) = %q, want empty", tt.role, got)
			}
			if !tt.wantEmpty && got == "" {
				t.Error("RoleFiltersForHNSW returned empty, expected non-empty")
			}
			if tt.wantContain != "" && !ContainsStr(got, tt.wantContain) {
				t.Errorf("RoleFiltersForHNSW(%q) should contain %q, got: %q", tt.role, tt.wantContain, got)
			}
		})
	}
}

func TestPostFilterByPath(t *testing.T) {
	// Create test rows: [name, file_path, signature, start_line, distance]
	rows := [][]any{
		{"HandleRequest", "internal/handler.go", "func HandleRequest()", 10, 0.1},
		{"TestHandler", "internal/handler_test.go", "func TestHandler()", 1, 0.2},
		{"MockService", "internal/mocks/service.go", "type MockService", 5, 0.3},
		{"RegisterRoutes", "internal/routes.go", "func RegisterRoutes()", 1, 0.15},
		{"$anon_1", "internal/utils.go", "func()", 20, 0.25},
		{"$arrow_2", "src/component.ts", "() => {}", 30, 0.22},
	}

	tests := []struct {
		name             string
		pathPattern      string
		role             string
		query            string
		excludePaths     string
		excludeAnonymous bool
		wantLen          int
		wantContains     []string
		wantExcludes     []string
	}{
		{
			name:             "filter by path pattern",
			pathPattern:      "routes",
			role:             "source",
			excludeAnonymous: true,
			wantLen:          1,
			wantContains:     []string{"RegisterRoutes"},
		},
		{
			name:             "exclude tests with source role",
			pathPattern:      "",
			role:             "source",
			excludeAnonymous: true,
			wantExcludes:     []string{"TestHandler"},
		},
		{
			name:             "exclude mocks by default",
			pathPattern:      "",
			role:             "source",
			query:            "find handlers",
			excludeAnonymous: true,
			wantExcludes:     []string{"MockService"},
		},
		{
			name:             "include mocks when query mentions mock",
			pathPattern:      "",
			role:             "source",
			query:            "find mock implementations",
			excludeAnonymous: true,
			wantContains:     []string{"MockService"},
		},
		{
			name:             "exclude anonymous functions",
			pathPattern:      "",
			role:             "source",
			excludeAnonymous: true,
			wantExcludes:     []string{"$anon_1", "$arrow_2"},
		},
		{
			name:             "include anonymous when disabled",
			pathPattern:      "",
			role:             "any",
			excludeAnonymous: false,
			wantContains:     []string{"$anon_1", "$arrow_2"},
		},
		{
			name:             "exclude custom path pattern",
			pathPattern:      "",
			role:             "source",
			excludePaths:     "utils",
			excludeAnonymous: false, // disable to not filter $anon_1 by name
			wantExcludes:     []string{"$anon_1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := postFilterByPath(rows, tt.pathPattern, tt.role, tt.query, tt.excludePaths, tt.excludeAnonymous)

			if tt.wantLen > 0 && len(got) != tt.wantLen {
				t.Errorf("postFilterByPath() returned %d rows, want %d", len(got), tt.wantLen)
			}

			// Check for expected names
			for _, want := range tt.wantContains {
				found := false
				for _, row := range got {
					if name, ok := row[0].(string); ok && name == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("postFilterByPath() should contain %q", want)
				}
			}

			// Check for excluded names
			for _, exclude := range tt.wantExcludes {
				for _, row := range got {
					if name, ok := row[0].(string); ok && name == exclude {
						t.Errorf("postFilterByPath() should exclude %q", exclude)
					}
				}
			}
		})
	}
}

func TestBuildHNSWParams(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		role        string
		pathPattern string
		wantHighK   bool // expect high queryK for filtering
	}{
		{"any role no path", 10, "any", "", false},
		{"source role", 10, "source", "", true},
		{"test role", 10, "test", "", true},
		{"any with path pattern", 10, "any", "internal/", true},
		{"source with path pattern", 10, "source", "internal/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryK, ef := buildHNSWParams(tt.limit, tt.role, tt.pathPattern)

			if tt.wantHighK {
				// When filtering is needed, expect high queryK (>=1000)
				if queryK < 1000 {
					t.Errorf("buildHNSWParams() queryK = %d, expected >=1000 for filtering", queryK)
				}
				if ef < queryK {
					t.Errorf("buildHNSWParams() ef = %d, should be >= queryK (%d)", ef, queryK)
				}
			} else {
				// No filtering: queryK should match limit
				if queryK != tt.limit {
					t.Errorf("buildHNSWParams() queryK = %d, expected %d (limit)", queryK, tt.limit)
				}
			}
		})
	}
}

func TestFormatEmbeddingForCozoDB(t *testing.T) {
	tests := []struct {
		name      string
		embedding []float64
		want      string
	}{
		{
			name:      "simple embedding",
			embedding: []float64{0.1, 0.2, 0.3},
			want:      "vec([0.100000,0.200000,0.300000])",
		},
		{
			name:      "single value",
			embedding: []float64{0.5},
			want:      "vec([0.500000])",
		},
		{
			name:      "negative values",
			embedding: []float64{-0.1, 0.0, 0.1},
			want:      "vec([-0.100000,0.000000,0.100000])",
		},
		{
			name:      "empty embedding",
			embedding: []float64{},
			want:      "vec([])",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEmbeddingForCozoDB(tt.embedding)
			if got != tt.want {
				t.Errorf("formatEmbeddingForCozoDB() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSemanticSearchArgs_Defaults(t *testing.T) {
	args := SemanticSearchArgs{
		Query: "find authentication handlers",
	}

	// Verify default values
	if args.Limit != 0 {
		t.Errorf("Default Limit should be 0 (filled by function), got %d", args.Limit)
	}
	if args.Role != "" {
		t.Errorf("Default Role should be empty (defaults to 'source'), got %q", args.Role)
	}
	if args.PathPattern != "" {
		t.Errorf("Default PathPattern should be empty, got %q", args.PathPattern)
	}
	if args.ExcludeAnonymous {
		t.Errorf("Default ExcludeAnonymous should be false (defaults to true in function)")
	}
	if args.MinSimilarity != 0 {
		t.Errorf("Default MinSimilarity should be 0, got %f", args.MinSimilarity)
	}
}

func TestAnonymousFunctionPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Should match
		{"$anon_123", "$anon_123", true},
		{"$arrow_456", "$arrow_456", true},
		{"$lambda_789", "$lambda_789", true},
		{"anonymous", "anonymous", true},
		{"<anonymous>", "<anonymous>", true},

		// Should not match
		{"normal function", "HandleRequest", false},
		{"contains anon", "handleAnonData", false},
		{"$anon without number", "$anon_", false},
		{"prefix $anon", "prefix$anon_1", false},
		{"suffix $anon", "$anon_1suffix", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := anonymousFunctionPattern.MatchString(tt.input)
			if got != tt.want {
				t.Errorf("anonymousFunctionPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
