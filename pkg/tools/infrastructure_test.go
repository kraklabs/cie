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
	"context"
	"errors"
	"testing"
)

// TestMockClient_Query verifies the mock client returns configured results.
func TestMockClient_Query(t *testing.T) {
	t.Parallel()

	headers := []string{"name", "file_path"}
	rows := [][]any{
		{"Function1", "/path/to/file1.go"},
		{"Function2", "/path/to/file2.go"},
	}

	client := NewMockClientWithResults(headers, rows)
	ctx := context.Background()

	result, err := client.Query(ctx, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Headers) != len(headers) {
		t.Errorf("expected %d headers, got %d", len(headers), len(result.Headers))
	}

	if len(result.Rows) != len(rows) {
		t.Errorf("expected %d rows, got %d", len(rows), len(result.Rows))
	}

	// Verify first row
	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		if name, ok := result.Rows[0][0].(string); ok {
			if name != "Function1" {
				t.Errorf("expected first row name to be 'Function1', got %q", name)
			}
		}
	}
}

// TestMockClient_QueryRaw verifies QueryRaw returns configured results.
func TestMockClient_QueryRaw(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	rows := [][]any{{"1", "Test"}}

	client := NewMockClientWithResults(headers, rows)
	ctx := context.Background()

	result, err := client.QueryRaw(ctx, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultHeaders, ok := result["Headers"].([]string)
	if !ok {
		t.Fatalf("expected Headers to be []string")
	}

	if len(resultHeaders) != len(headers) {
		t.Errorf("expected %d headers, got %d", len(headers), len(resultHeaders))
	}
}

// TestMockClient_Error verifies the mock client returns configured errors.
func TestMockClient_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("database connection failed")
	client := NewMockClientWithError(expectedErr)
	ctx := context.Background()

	_, err := client.Query(ctx, "test query")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != expectedErr.Error() {
		t.Errorf("expected error %q, got %q", expectedErr.Error(), err.Error())
	}
}

// TestMockClient_Empty verifies the empty mock client returns empty results.
func TestMockClient_Empty(t *testing.T) {
	t.Parallel()

	client := NewMockClientEmpty()
	ctx := context.Background()

	result, err := client.Query(ctx, "test query")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

// TestMockQueryResult verifies NewMockQueryResult creates correct structure.
func TestMockQueryResult(t *testing.T) {
	t.Parallel()

	headers := []string{"col1", "col2", "col3"}
	rows := [][]any{
		{"val1", "val2", "val3"},
		{"val4", "val5", "val6"},
	}

	result := NewMockQueryResult(headers, rows)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(result.Headers))
	}

	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}

	if result.Headers[0] != "col1" {
		t.Errorf("expected first header to be 'col1', got %q", result.Headers[0])
	}
}

// TestMockClient_Custom verifies custom mock functions work.
func TestMockClient_Custom(t *testing.T) {
	t.Parallel()

	callCount := 0
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			callCount++
			if script == "error" {
				return nil, errors.New("custom error")
			}
			return NewMockQueryResult([]string{"result"}, [][]any{{"ok"}}), nil
		},
		nil,
	)

	ctx := context.Background()

	// Test success case
	result, err := client.Query(ctx, "success")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}

	// Test error case
	_, err = client.Query(ctx, "error")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// TestSetupTest verifies the test setup helper.
func TestSetupTest(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	// Verify context is not already canceled
	select {
	case <-ctx.Done():
		t.Fatal("context should not be canceled immediately")
	default:
		// Good
	}
}

// TestSetupTestWithMock verifies the combined setup helper.
func TestSetupTestWithMock(t *testing.T) {
	t.Parallel()

	headers := []string{"name"}
	rows := [][]any{{"TestFunc"}}

	ctx, client := setupTestWithMock(t, headers, rows)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	result, err := client.Query(ctx, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

// TestSetupTestWithEmptyMock verifies the empty mock setup helper.
func TestSetupTestWithEmptyMock(t *testing.T) {
	t.Parallel()

	ctx, client := setupTestWithEmptyMock(t)

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	result, err := client.Query(ctx, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

// TestCreateTestFunction verifies the function fixture builder.
func TestCreateTestFunction(t *testing.T) {
	t.Parallel()

	fn := createTestFunction("TestFunc", "/test/file.go", 42)

	if fn.Name != "TestFunc" {
		t.Errorf("expected name 'TestFunc', got %q", fn.Name)
	}

	if fn.FilePath != "/test/file.go" {
		t.Errorf("expected file path '/test/file.go', got %q", fn.FilePath)
	}

	if fn.StartLine != 42 {
		t.Errorf("expected start line 42, got %d", fn.StartLine)
	}

	if fn.ID == "" {
		t.Error("expected non-empty ID")
	}
}

// TestCreateTestFunctionResult verifies the function result builder.
func TestCreateTestFunctionResult(t *testing.T) {
	t.Parallel()

	functions := []FunctionInfo{
		createTestFunction("Func1", "/file1.go", 10),
		createTestFunction("Func2", "/file2.go", 20),
	}

	result := createTestFunctionResult(functions...)

	if len(result.Headers) != 7 {
		t.Errorf("expected 7 headers, got %d", len(result.Headers))
	}

	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Rows))
	}

	// Verify first row matches first function
	if len(result.Rows) > 0 && len(result.Rows[0]) > 1 {
		if name, ok := result.Rows[0][1].(string); ok {
			if name != "Func1" {
				t.Errorf("expected first row name 'Func1', got %q", name)
			}
		}
	}
}

// TestCreateTestSearchResult verifies the search result builder.
func TestCreateTestSearchResult(t *testing.T) {
	t.Parallel()

	names := []string{"Func1", "Func2", "Func3"}
	paths := []string{"/file1.go", "/file2.go"}

	result := createTestSearchResult(names, paths)

	if len(result.Headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(result.Headers))
	}

	if len(result.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(result.Rows))
	}

	// Verify first row
	if len(result.Rows) > 0 {
		if name, ok := result.Rows[0][0].(string); ok {
			if name != "Func1" {
				t.Errorf("expected first name 'Func1', got %q", name)
			}
		}
	}

	// Verify path handling when names > paths
	if len(result.Rows) > 2 {
		if path, ok := result.Rows[2][1].(string); ok {
			if path != "" {
				t.Errorf("expected empty path for third row, got %q", path)
			}
		}
	}
}

// TestAssertionHelpers verifies the assertion helper functions.
func TestAssertionHelpers(t *testing.T) {
	t.Parallel()

	// Test assertNoError - should not panic with nil error
	assertNoError(t, nil)

	// Test assertEqual - should not fail with equal values
	assertEqual(t, 42, 42)
	assertEqual(t, "test", "test")

	// Test assertContains - should not fail when substring exists
	assertContains(t, "hello world", "world")

	// Test assertNotContains - should not fail when substring doesn't exist
	assertNotContains(t, "hello world", "xyz")

	// Test assertRowCount
	rows := [][]any{{"a"}, {"b"}}
	assertRowCount(t, rows, 2)

	// Test assertRowsContain
	rows = [][]any{{"Function1"}, {"Function2"}}
	assertRowsContain(t, rows, "Function1")

	// Test assertRowsNotContain
	assertRowsNotContain(t, rows, "Function3")
}
