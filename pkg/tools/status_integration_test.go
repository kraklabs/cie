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

// Integration tests for status.go helper functions.
// Run with: go test -tags=cozodb ./pkg/tools/...

package tools

import (
	"context"
	"testing"
)

func TestCountWithFallback_EmptyDB_Integration(t *testing.T) {
	db := openTestDB(t)

	// Test with empty database
	client := NewTestCIEClient(db)
	ctx := context.Background()

	// Test countWithFallback with empty database
	count := countWithFallback(ctx, client, "test_files",
		`?[cnt] := cnt = count(id), *cie_file { id }`,
		`?[id] := *cie_file { id } :limit 10000`)

	if count != 0 {
		t.Errorf("countWithFallback() on empty DB = %d; want 0", count)
	}

	// Add some data and test again
	insertTestFile(t, db, "file1", "test.go", "go")

	count = countWithFallback(ctx, client, "test_files",
		`?[cnt] := cnt = count(id), *cie_file { id }`,
		`?[id] := *cie_file { id } :limit 10000`)

	if count != 1 {
		t.Errorf("countWithFallback() after insert = %d; want 1", count)
	}
}

func TestCountWithFallback_PathFiltering_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with distinct paths
	insertTestFile(t, db, "file1", "internal/handler.go", "go")
	insertTestFile(t, db, "file2", "apps/gateway/main.go", "go")

	client := NewTestCIEClient(db)
	ctx := context.Background()

	// Test path filtering via regex
	count := countWithFallback(ctx, client, "internal_files",
		`?[cnt] := cnt = count(id), *cie_file { id, path }, regex_matches(path, "internal")`,
		`?[id] := *cie_file { id, path }, regex_matches(path, "internal") :limit 10000`)

	if count != 1 {
		t.Errorf("Path filter for 'internal' = %d; want 1", count)
	}

	count = countWithFallback(ctx, client, "gateway_files",
		`?[cnt] := cnt = count(id), *cie_file { id, path }, regex_matches(path, "gateway")`,
		`?[id] := *cie_file { id, path }, regex_matches(path, "gateway") :limit 10000`)

	if count != 1 {
		t.Errorf("Path filter for 'gateway' = %d; want 1", count)
	}
}
