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
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestDirectorySummary_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data - files in a directory structure
	insertTestFile(t, db, "file1", "internal/handler/user.go", "go")
	insertTestFile(t, db, "file2", "internal/handler/order.go", "go")
	insertTestFile(t, db, "file3", "internal/service/user.go", "go")

	insertTestFunction(t, db, "func1", "HandleUser", "internal/handler/user.go",
		"func HandleUser()", "func HandleUser() { }", 10)
	insertTestFunction(t, db, "func2", "HandleGetUser", "internal/handler/user.go",
		"func HandleGetUser()", "func HandleGetUser() { }", 20)
	insertTestFunction(t, db, "func3", "HandleOrder", "internal/handler/order.go",
		"func HandleOrder()", "func HandleOrder() { }", 10)
	insertTestFunction(t, db, "func4", "GetUser", "internal/service/user.go",
		"func GetUser()", "func GetUser() { }", 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		path        string
		maxFuncs    int
		wantContain []string
	}{
		{
			name:        "handler directory",
			path:        "internal/handler",
			maxFuncs:    5,
			wantContain: []string{"user.go", "order.go", "HandleUser"},
		},
		{
			name:        "service directory",
			path:        "internal/service",
			maxFuncs:    5,
			wantContain: []string{"user.go", "GetUser"},
		},
		{
			name:        "limit functions per file",
			path:        "internal/handler",
			maxFuncs:    1,
			wantContain: []string{"user.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DirectorySummary(ctx, client, tt.path, tt.maxFuncs)
			if err != nil {
				t.Fatalf("DirectorySummary() error = %v", err)
			}

			if result.IsError {
				t.Errorf("DirectorySummary() returned error: %s", result.Text)
				return
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("DirectorySummary() should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}
