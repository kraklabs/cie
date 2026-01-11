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
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Assertion Helpers
// These helpers reduce boilerplate in test code and provide clear error messages.

// assertNoError fails the test if err is not nil.
// It uses t.Helper() to ensure the error is reported at the call site.
//
// Example:
//
//	result, err := SomeFunction()
//	assertNoError(t, err)
func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertEqual fails the test if got != want.
// It provides a detailed error message showing both values.
//
// Example:
//
//	assertEqual(t, result.Name, "ExpectedName")
func assertEqual(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		msg := ""
		if len(msgAndArgs) > 0 {
			if format, ok := msgAndArgs[0].(string); ok {
				msg = fmt.Sprintf(format, msgAndArgs[1:]...)
			}
		}
		if msg != "" {
			msg = ": " + msg
		}
		t.Fatalf("assertion failed%s\ngot:  %#v\nwant: %#v", msg, got, want)
	}
}

// assertContains fails the test if haystack does not contain needle.
//
// Example:
//
//	assertContains(t, result.Text, "expected substring")
func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected string to contain %q, got:\n%s", needle, haystack)
	}
}

// assertNotContains fails the test if haystack contains needle.
//
// Example:
//
//	assertNotContains(t, result.Text, "unwanted substring")
func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected string to NOT contain %q, got:\n%s", needle, haystack)
	}
}

// assertRowsContain fails the test if rows does not contain a row with the given name.
// This is useful for validating QueryResult rows where the first column is typically the name.
//
// Example:
//
//	assertRowsContain(t, result.Rows, "ExpectedFunctionName")
func assertRowsContain(t *testing.T, rows [][]any, wantName string) {
	t.Helper()
	for _, row := range rows {
		if len(row) > 0 {
			if name, ok := row[0].(string); ok && name == wantName {
				return // Found it
			}
		}
	}
	t.Fatalf("expected rows to contain %q, got %d rows", wantName, len(rows))
}

// assertRowsNotContain fails the test if rows contains a row with the given name.
//
// Example:
//
//	assertRowsNotContain(t, result.Rows, "UnwantedFunctionName")
func assertRowsNotContain(t *testing.T, rows [][]any, unwantedName string) {
	t.Helper()
	for _, row := range rows {
		if len(row) > 0 {
			if name, ok := row[0].(string); ok && name == unwantedName {
				t.Fatalf("expected rows to NOT contain %q", unwantedName)
			}
		}
	}
}

// assertRowCount fails the test if the number of rows doesn't match expected.
//
// Example:
//
//	assertRowCount(t, result.Rows, 5)
func assertRowCount(t *testing.T, rows [][]any, want int) {
	t.Helper()
	if got := len(rows); got != want {
		t.Fatalf("expected %d rows, got %d", want, got)
	}
}

// Fixture Builders
// These builders create test data structures matching the CozoDB schema format.

// createTestFunction creates a FunctionInfo fixture.
// This is useful for building expected results in tests.
//
// Example:
//
//	expected := createTestFunction("HandleRequest", "/internal/handler.go", 42)
func createTestFunction(name, file string, line int) FunctionInfo {
	return FunctionInfo{
		ID:        fmt.Sprintf("func_%s", name),
		Name:      name,
		Signature: fmt.Sprintf("func %s()", name),
		FilePath:  file,
		CodeText:  fmt.Sprintf("func %s() {\n\t// implementation\n}", name),
		StartLine: line,
		EndLine:   line + 5,
	}
}

// createTestFunctionWithCode creates a FunctionInfo with custom code.
//
// Example:
//
//	fn := createTestFunctionWithCode("Parse", "/pkg/parser.go", 10, "func Parse() error { return nil }")
func createTestFunctionWithCode(name, file string, line int, code string) FunctionInfo {
	return FunctionInfo{
		ID:        fmt.Sprintf("func_%s", name),
		Name:      name,
		Signature: fmt.Sprintf("func %s()", name),
		FilePath:  file,
		CodeText:  code,
		StartLine: line,
		EndLine:   line + strings.Count(code, "\n"),
	}
}

// createTestFunctionResult converts FunctionInfo fixtures to a QueryResult.
// This matches the format returned by CozoDB function queries.
//
// Example:
//
//	functions := []FunctionInfo{
//	    createTestFunction("Func1", "/file1.go", 10),
//	    createTestFunction("Func2", "/file2.go", 20),
//	}
//	result := createTestFunctionResult(functions...)
func createTestFunctionResult(functions ...FunctionInfo) *QueryResult {
	headers := []string{"id", "name", "signature", "file_path", "code", "start_line", "end_line"}
	rows := make([][]any, len(functions))
	for i, fn := range functions {
		rows[i] = []any{fn.ID, fn.Name, fn.Signature, fn.FilePath, fn.CodeText, fn.StartLine, fn.EndLine}
	}
	return &QueryResult{
		Headers: headers,
		Rows:    rows,
	}
}

// createTestSearchResult creates a QueryResult for search operations.
// This matches the format returned by semantic search and text search queries.
//
// Example:
//
//	result := createTestSearchResult(
//	    []string{"FindFunction", "SearchText"},
//	    []string{"/pkg/tools/code.go", "/pkg/tools/search.go"},
//	)
func createTestSearchResult(names []string, paths []string) *QueryResult {
	headers := []string{"name", "file_path"}
	rows := make([][]any, len(names))
	for i := range names {
		path := ""
		if i < len(paths) {
			path = paths[i]
		}
		rows[i] = []any{names[i], path}
	}
	return &QueryResult{
		Headers: headers,
		Rows:    rows,
	}
}

// createTestCallerResult creates a QueryResult for caller/callee queries.
//
// Example:
//
//	result := createTestCallerResult(
//	    []CallerInfo{
//	        {CallerName: "Main", CallerFile: "/main.go", CallerLine: 10, CalleeName: "Init"},
//	        {CallerName: "Init", CallerFile: "/init.go", CallerLine: 5, CalleeName: "Setup"},
//	    },
//	)
func createTestCallerResult(callers []CallerInfo) *QueryResult {
	headers := []string{"caller_name", "caller_file", "caller_line", "callee_name"}
	rows := make([][]any, len(callers))
	for i, c := range callers {
		rows[i] = []any{c.CallerName, c.CallerFile, c.CallerLine, c.CalleeName}
	}
	return &QueryResult{
		Headers: headers,
		Rows:    rows,
	}
}

// createTestFileResult creates a QueryResult for file listing queries.
//
// Example:
//
//	result := createTestFileResult(
//	    []FileInfo{
//	        {ID: "file1", Path: "/main.go", Language: "go", Size: 1024},
//	        {ID: "file2", Path: "/util.go", Language: "go", Size: 512},
//	    },
//	)
func createTestFileResult(files []FileInfo) *QueryResult {
	headers := []string{"id", "path", "language", "size"}
	rows := make([][]any, len(files))
	for i, f := range files {
		rows[i] = []any{f.ID, f.Path, f.Language, f.Size}
	}
	return &QueryResult{
		Headers: headers,
		Rows:    rows,
	}
}

// Test Setup Helpers

// setupTest creates a test context with timeout and registers cleanup.
// This ensures tests don't hang and resources are properly cleaned up.
//
// Example:
//
//	func TestSomething(t *testing.T) {
//	    ctx := setupTest(t)
//	    result, err := SomeOperation(ctx)
//	    // test continues...
//	}
func setupTest(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// setupTestWithMock creates a test context and a mock client.
// This is a convenience function for the common pattern of setting up both.
//
// Example:
//
//	func TestSearchText(t *testing.T) {
//	    ctx, client := setupTestWithMock(t, []string{"name"}, [][]any{{"Result1"}})
//	    result, err := SearchText(ctx, client, SearchTextArgs{Pattern: "test"})
//	    assertNoError(t, err)
//	}
func setupTestWithMock(t *testing.T, headers []string, rows [][]any) (context.Context, *MockCIEClient) {
	t.Helper()
	ctx := setupTest(t)
	client := NewMockClientWithResults(headers, rows)
	return ctx, client
}

// setupTestWithEmptyMock creates a test context and an empty mock client.
// Useful for testing "no results" scenarios.
//
// Example:
//
//	func TestSearchText_NoResults(t *testing.T) {
//	    ctx, client := setupTestWithEmptyMock(t)
//	    result, err := SearchText(ctx, client, SearchTextArgs{Pattern: "nonexistent"})
//	    assertNoError(t, err)
//	    assertRowCount(t, result.Rows, 0)
//	}
func setupTestWithEmptyMock(t *testing.T) (context.Context, *MockCIEClient) {
	t.Helper()
	ctx := setupTest(t)
	client := NewMockClientEmpty()
	return ctx, client
}
