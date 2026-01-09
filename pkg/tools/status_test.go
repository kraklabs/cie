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
