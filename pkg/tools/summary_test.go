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
