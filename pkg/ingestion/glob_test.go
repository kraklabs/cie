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
package ingestion

import (
	"testing"
)

func TestMatchesGlob_BasicPatterns(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Exact match
		{"exact match", "foo.go", "foo.go", true},
		{"exact no match", "foo.go", "bar.go", false},

		// * wildcard (single segment)
		{"star prefix", "foo.go", "*.go", true},
		{"star suffix", "test_foo", "test_*", true},
		{"star middle", "test_foo_bar", "test_*_bar", true},
		{"star no match ext", "foo.txt", "*.go", false},

		// ** wildcard (any depth)
		{"doublestar prefix any depth", "a/b/c/foo.go", "**/*.go", true},
		{"doublestar prefix root", "foo.go", "**/*.go", true},
		{"doublestar suffix", "node_modules/pkg/index.js", "node_modules/**", true},
		{"doublestar suffix nested", "node_modules/a/b/c/d.js", "node_modules/**", true},
		{"doublestar full path", "vendor/github.com/pkg/errors/errors.go", "vendor/**", true},

		// ? wildcard (single char)
		{"question single", "foo.go", "fo?.go", true},
		{"question no match", "fooo.go", "fo?.go", false},

		// Character classes
		{"char class match", "foo.go", "foo.[gt]o", true},
		{"char class no match", "foo.go", "foo.[ab]o", false},
		{"char range match", "file1.go", "file[0-9].go", true},
		{"char range no match", "filea.go", "file[0-9].go", false},
		{"negated class match", "foo.go", "foo.[!ab]o", true},
		{"negated class no match", "foo.ao", "foo.[!ab]o", false},

		// Common patterns
		{".git dir exact", ".git", ".git/**", true},
		{".git subdir", ".git/objects/pack", ".git/**", true},
		{"node_modules deep", "node_modules/lodash/package.json", "node_modules/**", true},
		{"vendor deep", "vendor/github.com/pkg/errors/errors.go", "vendor/**", true},
		{"dist match", "dist/bundle.js", "dist/**", true},
		{"build match", "build/output/main", "build/**", true},

		// Pattern without ** can match anywhere
		{"implicit prefix", "src/test.go", "test.go", true},
		{"implicit prefix nested", "a/b/c/test.go", "test.go", true},

		// Directory patterns
		{"dir pattern", "tests/unit/test.go", "tests/**", true},
		{"dir pattern exact", "tests", "tests/**", true},

		// bin/** pattern - critical fix for nested directories
		{"bin nested dir", "apps/catalog/bin", "bin/**", true},
		{"bin nested dir deep", "apps/gateway/bin", "bin/**", true},
		{"bin exact", "bin", "bin/**", true},
		{"bin nested file", "apps/catalog/bin/catalog", "bin/**", true},
		{"bindings no match", "apps/bindings/foo", "bin/**", false},

		// Complex patterns
		{"complex nested", "src/components/Button/Button.test.tsx", "**/*.test.tsx", true},
		{"complex no match", "src/components/Button/Button.tsx", "**/*.test.tsx", false},

		// Edge cases
		{"empty path", "", "**", true},
		{"empty pattern", "foo.go", "", false},
		{"path with dots", "foo.bar.baz.go", "*.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesGlob(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesGlob(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesGlob_CommonExcludePatterns(t *testing.T) {
	// Test patterns commonly used in ExcludeGlobs
	patterns := []string{
		".git/**",
		"node_modules/**",
		"dist/**",
		"vendor/**",
		"build/**",
		"*.o",
		"*.so",
		"*.dylib",
	}

	// Paths that should be excluded
	excludedPaths := []string{
		".git/objects/pack/file",
		".git/HEAD",
		"node_modules/lodash/index.js",
		"node_modules/@types/node/index.d.ts",
		"dist/bundle.js",
		"dist/assets/style.css",
		"vendor/github.com/pkg/errors/errors.go",
		"build/main",
		"build/output/binary",
		"main.o",
		"src/lib.o",
		"libfoo.so",
		"lib/libbar.dylib",
	}

	// Paths that should NOT be excluded
	includedPaths := []string{
		"src/main.go",
		"pkg/handler/handler.go",
		"cmd/app/main.go",
		"internal/config/config.go",
		"README.md",
		"go.mod",
		"go.sum",
		".gitignore",     // Not in .git directory
		"git/file.go",    // Not .git
		"modules/foo.go", // Not node_modules
	}

	rl := &RepoLoader{}

	for _, path := range excludedPaths {
		if !rl.shouldExclude(path, patterns) {
			t.Errorf("shouldExclude(%q) = false, want true", path)
		}
	}

	for _, path := range includedPaths {
		if rl.shouldExclude(path, patterns) {
			t.Errorf("shouldExclude(%q) = true, want false", path)
		}
	}
}

func TestMatchCharClass(t *testing.T) {
	tests := []struct {
		name  string
		c     byte
		class string
		want  bool
	}{
		{"simple match", 'a', "abc", true},
		{"simple no match", 'd', "abc", false},
		{"range match", 'e', "a-z", true},
		{"range no match", 'E', "a-z", false},
		{"digit range", '5', "0-9", true},
		{"negated match", 'd', "!abc", true},
		{"negated no match", 'a', "!abc", false},
		{"caret negation", 'd', "^abc", true},
		{"mixed", 'f', "a-z0-9", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchCharClass(tt.c, tt.class)
			if got != tt.want {
				t.Errorf("matchCharClass(%c, %q) = %v, want %v", tt.c, tt.class, got, tt.want)
			}
		})
	}
}

func TestMatchGlobPattern_Complex(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Multiple wildcards
		{"multi star", "src/test/foo_test.go", "src/*/*.go", true},
		{"multi star deep", "a/b/c/d.go", "a/*/c/*.go", true},

		// ** in middle
		{"doublestar middle", "src/pkg/util/helper.go", "src/**/helper.go", true},
		{"doublestar middle deep", "a/b/c/d/e/f.go", "a/**/f.go", true},

		// Mixed wildcards
		{"mixed wildcards", "test_data/fixture_1.json", "test_*/*_?.json", true},

		// Trailing patterns
		// Note: pkg/ with trailing slash can match pkg/* because * matches empty string
		// This is acceptable behavior for exclude patterns
		{"file in dir", "pkg/file.go", "pkg/*", true},
		{"nested file", "pkg/sub/file.go", "pkg/*/*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchGlobPattern(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("matchGlobPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}
