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

// Integration tests for status.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...
//
// Note: IndexStatus uses CIEClient-specific fields (ProjectID, BaseURL) for output.
// The query correctness is tested via schema_test.go. This file tests the
// countWithFallback helper using a minimal approach.

package tools

import (
	"context"
	"testing"
)

func TestCountWithFallback_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFile(t, db, "file1", "internal/handler.go", "go")
	insertTestFile(t, db, "file2", "internal/service.go", "go")
	insertTestFile(t, db, "file3", "apps/gateway/routes.go", "go")

	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest()", "func HandleRequest() { }", 10)
	insertTestFunction(t, db, "func2", "ProcessData", "internal/service.go",
		"func ProcessData()", "func ProcessData() { }", 20)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name      string
		countQ    string
		listQ     string
		wantCount int
	}{
		{
			name:      "count files",
			countQ:    `?[cnt] := cnt = count(id), *cie_file { id }`,
			listQ:     `?[id] := *cie_file { id } :limit 10000`,
			wantCount: 3,
		},
		{
			name:      "count functions",
			countQ:    `?[cnt] := cnt = count(id), *cie_function { id }`,
			listQ:     `?[id] := *cie_function { id } :limit 10000`,
			wantCount: 2,
		},
		{
			name:      "count by path",
			countQ:    `?[cnt] := cnt = count(id), *cie_file { id, path }, regex_matches(path, "internal")`,
			listQ:     `?[id] := *cie_file { id, path }, regex_matches(path, "internal") :limit 10000`,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cnt := countWithFallback(ctx, client, tt.name, tt.countQ, tt.listQ)
			if cnt != tt.wantCount {
				t.Errorf("countWithFallback() = %d, want %d", cnt, tt.wantCount)
			}
		})
	}
}
