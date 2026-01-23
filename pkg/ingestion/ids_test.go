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

func TestGenerateFileID_Deterministic(t *testing.T) {
	path := "test/path/to/file.go"

	// Generate ID twice
	id1 := GenerateFileID(path)
	id2 := GenerateFileID(path)

	if id1 != id2 {
		t.Errorf("GenerateFileID should be deterministic: got %q and %q", id1, id2)
	}

	// Verify it starts with "file:"
	if !hasPrefix(id1, "file:") {
		t.Errorf("GenerateFileID should start with 'file:': got %q", id1)
	}
}

func TestGenerateFileID_DifferentPaths(t *testing.T) {
	path1 := "test/path/to/file1.go"
	path2 := "test/path/to/file2.go"

	id1 := GenerateFileID(path1)
	id2 := GenerateFileID(path2)

	if id1 == id2 {
		t.Errorf("GenerateFileID should produce different IDs for different paths: both got %q", id1)
	}
}

func TestGenerateFileID_NormalizesPath(t *testing.T) {
	path1 := "./test/path/to/file.go"
	path2 := "test/path/to/file.go"

	id1 := GenerateFileID(path1)
	id2 := GenerateFileID(path2)

	if id1 != id2 {
		t.Errorf("GenerateFileID should normalize paths: got %q and %q", id1, id2)
	}
}

func TestGenerateFunctionID_Deterministic(t *testing.T) {
	filePath := "test.go"
	name := "testFunction"
	signature := "func testFunction()"
	startLine := 10
	endLine := 15
	startCol := 1
	endCol := 20

	// Generate ID twice
	id1 := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol, endCol)
	id2 := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol, endCol)

	if id1 != id2 {
		t.Errorf("GenerateFunctionID should be deterministic: got %q and %q", id1, id2)
	}

	// Verify it starts with "func:"
	if !hasPrefix(id1, "func:") {
		t.Errorf("GenerateFunctionID should start with 'func:': got %q", id1)
	}
}

func TestGenerateFunctionID_DifferentFunctions(t *testing.T) {
	filePath := "test.go"
	name1 := "function1"
	name2 := "function2"
	signature := "func test()"
	startLine := 10
	endLine := 15
	startCol := 1
	endCol := 20

	id1 := GenerateFunctionID(filePath, name1, signature, startLine, endLine, startCol, endCol)
	id2 := GenerateFunctionID(filePath, name2, signature, startLine, endLine, startCol, endCol)

	if id1 == id2 {
		t.Errorf("GenerateFunctionID should produce different IDs for different functions: both got %q", id1)
	}
}

func TestGenerateFunctionID_DifferentRanges(t *testing.T) {
	filePath := "test.go"
	name := "testFunction"
	signature := "func testFunction()"
	startLine1 := 10
	endLine1 := 15
	startCol1 := 1
	endCol1 := 20
	startLine2 := 20
	endLine2 := 25
	startCol2 := 1
	endCol2 := 25

	id1 := GenerateFunctionID(filePath, name, signature, startLine1, endLine1, startCol1, endCol1)
	id2 := GenerateFunctionID(filePath, name, signature, startLine2, endLine2, startCol2, endCol2)

	if id1 == id2 {
		t.Errorf("GenerateFunctionID should produce different IDs for different ranges: both got %q", id1)
	}
}

func TestGenerateFunctionID_DifferentSignatures(t *testing.T) {
	filePath := "test.go"
	name := "testFunction"
	signature1 := "func testFunction()"
	signature2 := "func testFunction(x int)"
	startLine := 10
	endLine := 15
	startCol := 1
	endCol := 20

	id1 := GenerateFunctionID(filePath, name, signature1, startLine, endLine, startCol, endCol)
	id2 := GenerateFunctionID(filePath, name, signature2, startLine, endLine, startCol, endCol)

	// With the new implementation, signature is NOT included in the ID
	// This ensures IDs remain stable when parser improvements change signature extraction
	// Different signatures with same path, name, and range should produce the SAME ID
	if id1 != id2 {
		t.Errorf("GenerateFunctionID should produce the same ID for different signatures (signature not in ID): got %q and %q", id1, id2)
	}
}

// TestGenerateFunctionID_DifferentColumns tests that different columns produce different IDs
func TestGenerateFunctionID_DifferentColumns(t *testing.T) {
	filePath := "test.go"
	name := "testFunction"
	signature := "func testFunction()"
	startLine := 10
	endLine := 15
	startCol1 := 1
	endCol1 := 20
	startCol2 := 5
	endCol2 := 25

	id1 := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol1, endCol1)
	id2 := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol2, endCol2)

	// Different columns should produce different IDs (prevents collisions)
	if id1 == id2 {
		t.Errorf("GenerateFunctionID should produce different IDs for different columns: both got %q", id1)
	}
}

// Helper function to check prefix (avoid importing strings package)
func hasPrefix(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}
