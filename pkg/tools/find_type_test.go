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
