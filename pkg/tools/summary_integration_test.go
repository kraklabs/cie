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
//go:build cozodb
// +build cozodb

// Integration tests for summary.go functions.
// Run with: go test -tags=cozodb ./pkg/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestDirectorySummaryMain_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data - files and functions in a directory
	insertTestFile(t, db, "file1", "internal/handler/user.go", "go")
	insertTestFile(t, db, "file2", "internal/handler/product.go", "go")
	insertTestFile(t, db, "file3", "internal/service/user.go", "go")

	insertTestFunction(t, db, "func1", "HandleUserCreate", "internal/handler/user.go",
		"func HandleUserCreate(c *gin.Context)", "func HandleUserCreate(c *gin.Context) { }", 10)
	insertTestFunction(t, db, "func2", "HandleUserDelete", "internal/handler/user.go",
		"func HandleUserDelete(c *gin.Context)", "func HandleUserDelete(c *gin.Context) { }", 20)
	insertTestFunction(t, db, "func3", "handleUserValidation", "internal/handler/user.go",
		"func handleUserValidation()", "func handleUserValidation() { }", 30)

	insertTestFunction(t, db, "func4", "HandleProductList", "internal/handler/product.go",
		"func HandleProductList(c *gin.Context)", "func HandleProductList(c *gin.Context) { }", 10)
	insertTestFunction(t, db, "func5", "HandleProductCreate", "internal/handler/product.go",
		"func HandleProductCreate(c *gin.Context)", "func HandleProductCreate(c *gin.Context) { }", 20)

	insertTestFunction(t, db, "func6", "GetUserByID", "internal/service/user.go",
		"func GetUserByID(id string)", "func GetUserByID(id string) { }", 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name            string
		path            string
		maxFuncsPerFile int
		wantContain     []string
		wantExclude     []string
	}{
		{
			name:            "summary of handler directory",
			path:            "internal/handler",
			maxFuncsPerFile: 3,
			wantContain:     []string{"user.go", "product.go", "HandleUserCreate", "HandleProductList"},
			wantExclude:     []string{"service/user.go"},
		},
		{
			name:            "summary with maxFuncs=2",
			path:            "internal/handler",
			maxFuncsPerFile: 2,
			wantContain:     []string{"user.go", "HandleUserCreate", "HandleUserDelete"},
		},
		{
			name:            "directory with trailing slash",
			path:            "internal/handler/",
			maxFuncsPerFile: 5,
			wantContain:     []string{"user.go", "product.go"},
		},
		{
			name:            "empty directory",
			path:            "nonexistent/dir",
			maxFuncsPerFile: 5,
			wantContain:     []string{"No files found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DirectorySummary(ctx, client, tt.path, tt.maxFuncsPerFile)
			if err != nil {
				t.Fatalf("DirectorySummary() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("DirectorySummary() should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("DirectorySummary() should NOT contain %q", exclude)
				}
			}
		})
	}
}

func TestDirectorySummary_ExportedFunctionsFirst(t *testing.T) {
	db := openTestDB(t)

	// Setup test with mix of exported and unexported functions
	insertTestFile(t, db, "file1", "internal/handler.go", "go")

	// Unexported function first in line number order
	insertTestFunction(t, db, "func1", "handleInternal", "internal/handler.go",
		"func handleInternal()", "func handleInternal() { }", 10)

	// Exported function second
	insertTestFunction(t, db, "func2", "HandleRequest", "internal/handler.go",
		"func HandleRequest()", "func HandleRequest() { }", 20)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	result, err := DirectorySummary(ctx, client, "internal", 5)
	if err != nil {
		t.Fatalf("DirectorySummary() error = %v", err)
	}

	// Exported functions should appear first in output
	// (this tests the formatDirFuncs sorting logic)
	exportedPos := strings.Index(result.Text, "HandleRequest")
	unexportedPos := strings.Index(result.Text, "handleInternal")

	if exportedPos < 0 {
		t.Error("Should contain exported function HandleRequest")
	}
	if unexportedPos >= 0 && exportedPos > unexportedPos {
		t.Error("Exported function should appear before unexported in output")
	}
}
