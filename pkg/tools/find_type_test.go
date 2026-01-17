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

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filePath string
		want     string
	}{
		// Go
		{"handler.go", "go"},
		{"internal/service/handler.go", "go"},
		{"HANDLER.GO", "go"}, // case insensitive

		// Python
		{"main.py", "python"},
		{"scripts/utils.py", "python"},
		{"MAIN.PY", "python"},

		// TypeScript
		{"component.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"src/app/page.TSX", "typescript"},

		// JavaScript
		{"script.js", "javascript"},
		{"script.jsx", "javascript"},
		{"src/index.JS", "javascript"},

		// Rust
		{"main.rs", "rust"},
		{"lib.rs", "rust"},
		{"src/utils.RS", "rust"},

		// Java
		{"Main.java", "java"},
		{"com/example/Service.JAVA", "java"},

		// Unknown
		{"file.txt", "unknown"},
		{"file.c", "unknown"},
		{"file.cpp", "unknown"},
		{"file.h", "unknown"},
		{"file", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got := detectLanguage(tt.filePath)
			if got != tt.want {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestFindTypeArgs_Defaults(t *testing.T) {
	// Verify default values behavior
	args := FindTypeArgs{
		Name: "Handler",
	}

	if args.Name != "Handler" {
		t.Errorf("Name should be set, got %q", args.Name)
	}
	if args.Kind != "" {
		t.Errorf("Default Kind should be empty, got %q", args.Kind)
	}
	if args.PathPattern != "" {
		t.Errorf("Default PathPattern should be empty, got %q", args.PathPattern)
	}
	if args.Limit != 0 {
		t.Errorf("Default Limit should be 0 (to be filled in by function), got %d", args.Limit)
	}
}

func TestTypeInfo_Struct(t *testing.T) {
	// Verify TypeInfo struct fields can be set correctly
	ti := TypeInfo{
		Name:      "UserService",
		Kind:      "struct",
		FilePath:  "internal/service/user.go",
		StartLine: 10,
		EndLine:   50,
		CodeText:  "type UserService struct { ... }",
	}

	if ti.Name != "UserService" {
		t.Error("Name not set correctly")
	}
	if ti.Kind != "struct" {
		t.Error("Kind not set correctly")
	}
	if ti.FilePath != "internal/service/user.go" {
		t.Error("FilePath not set correctly")
	}
	if ti.StartLine != 10 {
		t.Error("StartLine not set correctly")
	}
	if ti.EndLine != 50 {
		t.Error("EndLine not set correctly")
	}
	if ti.CodeText != "type UserService struct { ... }" {
		t.Error("CodeText not set correctly")
	}
}

// Integration tests below - these require CozoDB and use the cozodb build tag
