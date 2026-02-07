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
	"strings"
	"testing"
)

// ============================================================================
// UNIT TESTS (no CozoDB required - use mocks)
// ============================================================================

// Mock result builders for unit tests

// mockSearchResult creates a QueryResult for search operations with the specified function names.
func mockSearchResult(names ...string) *QueryResult {
	rows := make([][]any, len(names))
	for i, name := range names {
		rows[i] = []any{"/pkg/file.go", name, "func " + name + "()", 10, 20}
	}
	return &QueryResult{
		Headers: []string{"file_path", "name", "signature", "start_line", "end_line"},
		Rows:    rows,
	}
}

// mockFunctionResult creates a QueryResult for FindFunction operations.
func mockFunctionResult(names ...string) *QueryResult {
	rows := make([][]any, len(names))
	for i, name := range names {
		rows[i] = []any{"/pkg/file.go", name, "func " + name + "()", 10, 20}
	}
	return &QueryResult{
		Headers: []string{"file_path", "name", "signature", "start_line", "end_line"},
		Rows:    rows,
	}
}

// mockFunctionResultWithCode creates a QueryResult for FindFunction with IncludeCode=true.
func mockFunctionResultWithCode(name, code string) *QueryResult {
	return &QueryResult{
		Headers: []string{"file_path", "name", "signature", "start_line", "end_line", "code_text"},
		Rows: [][]any{
			{"/pkg/file.go", name, "func " + name + "()", 10, 20, code},
		},
	}
}

// mockCallerResult creates a QueryResult for FindCallers operations.
func mockCallerResult(callerName, calleeName string) *QueryResult {
	return &QueryResult{
		Headers: []string{"caller_file", "caller_name", "caller_line", "callee_name"},
		Rows: [][]any{
			{"/pkg/caller.go", callerName, 15, calleeName},
		},
	}
}

// mockCalleeResult creates a QueryResult for FindCallees operations.
func mockCalleeResult(callerName, calleeName string) *QueryResult {
	return &QueryResult{
		Headers: []string{"caller_name", "callee_file", "callee_name", "callee_line"},
		Rows: [][]any{
			{callerName, "/pkg/callee.go", calleeName, 25},
		},
	}
}

// mockFileResult creates a QueryResult for ListFiles operations.
func mockFileResult(paths ...string) *QueryResult {
	rows := make([][]any, len(paths))
	for i, path := range paths {
		rows[i] = []any{path, "go", 1024}
	}
	return &QueryResult{
		Headers: []string{"path", "language", "size"},
		Rows:    rows,
	}
}

func TestSearchText(t *testing.T) {
	tests := []struct {
		name       string
		args       SearchTextArgs
		mockClient *MockCIEClient
		wantErr    bool
		wantText   string
	}{
		// Success cases
		{
			name: "basic regex search in all",
			args: SearchTextArgs{Pattern: "Handle.*", SearchIn: "all", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("HandleRequest", "HandleResponse").Headers,
				mockSearchResult("HandleRequest", "HandleResponse").Rows,
			),
			wantText: "HandleRequest",
		},
		{
			name: "search in code only",
			args: SearchTextArgs{Pattern: "TODO", SearchIn: "code", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("ProcessData").Headers,
				mockSearchResult("ProcessData").Rows,
			),
			wantText: "ProcessData",
		},
		{
			name: "search in signature",
			args: SearchTextArgs{Pattern: "ctx context.Context", SearchIn: "signature", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("HandleRequest").Headers,
				mockSearchResult("HandleRequest").Rows,
			),
			wantText: "HandleRequest",
		},
		{
			name: "search in name",
			args: SearchTextArgs{Pattern: "^Handle", SearchIn: "name", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("HandleRequest").Headers,
				mockSearchResult("HandleRequest").Rows,
			),
			wantText: "HandleRequest",
		},
		{
			name: "literal mode escapes regex chars",
			args: SearchTextArgs{Pattern: ".GET(", Literal: true, Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("RegisterRoutes").Headers,
				mockSearchResult("RegisterRoutes").Rows,
			),
			wantText: "RegisterRoutes",
		},
		{
			name: "with file pattern",
			args: SearchTextArgs{Pattern: "error", FilePattern: "internal/.*", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("HandleError").Headers,
				mockSearchResult("HandleError").Rows,
			),
			wantText: "HandleError",
		},
		{
			name: "with exclude pattern",
			args: SearchTextArgs{Pattern: "test", ExcludePattern: "_test.go", Limit: 10},
			mockClient: NewMockClientWithResults(
				mockSearchResult("TestHelper").Headers,
				mockSearchResult("TestHelper").Rows,
			),
			wantText: "TestHelper",
		},
		{
			name:       "no results found",
			args:       SearchTextArgs{Pattern: "NonExistent", SearchIn: "all", Limit: 10},
			mockClient: NewMockClientEmpty(),
			wantText:   "Found 0",
		},
		// Error cases
		{
			name:     "empty pattern error",
			args:     SearchTextArgs{Pattern: ""},
			wantErr:  true,
			wantText: "pattern' is required",
		},
		{
			name:     "invalid regex pattern",
			args:     SearchTextArgs{Pattern: "[invalid"},
			wantErr:  true,
			wantText: "Invalid Regex Pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := SearchText(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result, got success")
				}
				if tt.wantText != "" {
					assertContains(t, result.Text, tt.wantText)
				}
				return
			}

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestFindFunction(t *testing.T) {
	tests := []struct {
		name       string
		args       FindFunctionArgs
		mockClient *MockCIEClient
		wantErr    bool
		wantText   string
	}{
		{
			name: "exact match",
			args: FindFunctionArgs{Name: "NewCIEClient", ExactMatch: true},
			mockClient: NewMockClientWithResults(
				mockFunctionResult("NewCIEClient").Headers,
				mockFunctionResult("NewCIEClient").Rows,
			),
			wantText: "NewCIEClient",
		},
		{
			name: "partial match includes methods",
			args: FindFunctionArgs{Name: "Handle", ExactMatch: false},
			mockClient: NewMockClientWithResults(
				mockFunctionResult("Handle", "Service.Handle").Headers,
				mockFunctionResult("Handle", "Service.Handle").Rows,
			),
			wantText: "Service.Handle",
		},
		{
			name: "with receiver method",
			args: FindFunctionArgs{Name: "Client.Query"},
			mockClient: NewMockClientWithResults(
				mockFunctionResult("Client.Query").Headers,
				mockFunctionResult("Client.Query").Rows,
			),
			wantText: "Client.Query",
		},
		{
			name: "include code",
			args: FindFunctionArgs{Name: "main", IncludeCode: true},
			mockClient: NewMockClientWithResults(
				mockFunctionResultWithCode("main", "func main() { fmt.Println(\"hello\") }").Headers,
				mockFunctionResultWithCode("main", "func main() { fmt.Println(\"hello\") }").Rows,
			),
			wantText: "func main()",
		},
		{
			name:     "empty name error",
			args:     FindFunctionArgs{Name: ""},
			wantErr:  true,
			wantText: "name' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := FindFunction(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result, got success")
				}
				if tt.wantText != "" {
					assertContains(t, result.Text, tt.wantText)
				}
				return
			}

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestFindCallers(t *testing.T) {
	tests := []struct {
		name       string
		args       FindCallersArgs
		mockClient *MockCIEClient
		wantErr    bool
		wantText   string
	}{
		{
			name: "find callers of function",
			args: FindCallersArgs{FunctionName: "handleRequest"},
			mockClient: NewMockClientWithResults(
				mockCallerResult("main", "handleRequest").Headers,
				mockCallerResult("main", "handleRequest").Rows,
			),
			wantText: "main",
		},
		{
			name: "find callers with method receiver",
			args: FindCallersArgs{FunctionName: "Service.Process"},
			mockClient: NewMockClientWithResults(
				mockCallerResult("Controller.Handle", "Service.Process").Headers,
				mockCallerResult("Controller.Handle", "Service.Process").Rows,
			),
			wantText: "Controller.Handle",
		},
		{
			name:     "empty function name error",
			args:     FindCallersArgs{FunctionName: ""},
			wantErr:  true,
			wantText: "function_name' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := FindCallers(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result, got success")
				}
				if tt.wantText != "" {
					assertContains(t, result.Text, tt.wantText)
				}
				return
			}

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestFindCallees(t *testing.T) {
	tests := []struct {
		name       string
		args       FindCalleesArgs
		mockClient *MockCIEClient
		wantErr    bool
		wantText   string
	}{
		{
			name: "find callees of function",
			args: FindCalleesArgs{FunctionName: "main"},
			mockClient: NewMockClientWithResults(
				mockCalleeResult("main", "handleRequest").Headers,
				mockCalleeResult("main", "handleRequest").Rows,
			),
			wantText: "handleRequest",
		},
		{
			name: "find callees with method receiver",
			args: FindCalleesArgs{FunctionName: "Controller.Handle"},
			mockClient: NewMockClientWithResults(
				mockCalleeResult("Controller.Handle", "Service.Process").Headers,
				mockCalleeResult("Controller.Handle", "Service.Process").Rows,
			),
			wantText: "Service.Process",
		},
		{
			name:     "empty function name error",
			args:     FindCalleesArgs{FunctionName: ""},
			wantErr:  true,
			wantText: "function_name' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := FindCallees(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result, got success")
				}
				if tt.wantText != "" {
					assertContains(t, result.Text, tt.wantText)
				}
				return
			}

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestListFiles(t *testing.T) {
	tests := []struct {
		name       string
		args       ListFilesArgs
		mockClient *MockCIEClient
		wantText   string
	}{
		{
			name: "list all files",
			args: ListFilesArgs{Limit: 50},
			mockClient: NewMockClientWithResults(
				mockFileResult("handler.go", "service.go").Headers,
				mockFileResult("handler.go", "service.go").Rows,
			),
			wantText: "handler.go",
		},
		{
			name: "filter by path pattern",
			args: ListFilesArgs{PathPattern: "internal/.*", Limit: 50},
			mockClient: NewMockClientWithResults(
				mockFileResult("internal/handler.go").Headers,
				mockFileResult("internal/handler.go").Rows,
			),
			wantText: "internal",
		},
		{
			name: "filter by language",
			args: ListFilesArgs{Language: "python", Limit: 50},
			mockClient: NewMockClientWithResults(
				mockFileResult("script.py").Headers,
				mockFileResult("script.py").Rows,
			),
			wantText: "script.py",
		},
		{
			name: "both filters",
			args: ListFilesArgs{PathPattern: "test/.*", Language: "go", Limit: 50},
			mockClient: NewMockClientWithResults(
				mockFileResult("test/handler_test.go").Headers,
				mockFileResult("test/handler_test.go").Rows,
			),
			wantText: "test/handler_test.go",
		},
		{
			name: "default limit applied",
			args: ListFilesArgs{},
			mockClient: NewMockClientWithResults(
				mockFileResult("file.go").Headers,
				mockFileResult("file.go").Rows,
			),
			wantText: "file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := ListFiles(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestRawQuery(t *testing.T) {
	tests := []struct {
		name       string
		args       RawQueryArgs
		mockClient *MockCIEClient
		wantErr    bool
		wantText   string
	}{
		{
			name: "valid query",
			args: RawQueryArgs{Script: "?[name] := *cie_function {name}"},
			mockClient: NewMockClientWithResults(
				mockSearchResult("HandleRequest").Headers,
				mockSearchResult("HandleRequest").Rows,
			),
			wantText: "HandleRequest",
		},
		{
			name: "complex query",
			args: RawQueryArgs{Script: "?[caller, callee] := *cie_calls {caller_id, callee_id}"},
			mockClient: NewMockClientWithResults(
				mockCallerResult("main", "init").Headers,
				mockCallerResult("main", "init").Rows,
			),
			wantText: "main",
		},
		{
			name:     "empty script error",
			args:     RawQueryArgs{Script: ""},
			wantErr:  true,
			wantText: "script' is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupTest(t)
			result, err := RawQuery(ctx, tt.mockClient, tt.args)
			assertNoError(t, err)

			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result, got success")
				}
				if tt.wantText != "" {
					assertContains(t, result.Text, tt.wantText)
				}
				return
			}

			if result.IsError {
				t.Errorf("unexpected error result: %s", result.Text)
				return
			}

			if tt.wantText != "" {
				assertContains(t, result.Text, tt.wantText)
			}
		})
	}
}

func TestSearchText_QueryError(t *testing.T) {
	ctx := setupTest(t)
	mockErr := fmt.Errorf("database connection failed")
	client := NewMockClientWithError(mockErr)

	result, err := SearchText(ctx, client, SearchTextArgs{Pattern: "test", Limit: 10})
	assertNoError(t, err)

	if !result.IsError {
		t.Error("expected error result when query fails")
	}
	assertContains(t, result.Text, "Query error")
}

func TestFindFunction_QueryError(t *testing.T) {
	ctx := setupTest(t)
	mockErr := fmt.Errorf("database connection failed")
	client := NewMockClientWithError(mockErr)

	result, err := FindFunction(ctx, client, FindFunctionArgs{Name: "test"})
	assertNoError(t, err)

	if !result.IsError {
		t.Error("expected error result when query fails")
	}
	assertContains(t, result.Text, "Query error")
	assertContains(t, result.Text, "Generated query")
}

// Test FindFunction case-insensitive matching uses regex
func TestFindFunction_CaseInsensitive(t *testing.T) {
	var capturedScript string
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			capturedScript = script
			return &QueryResult{
				Headers: []string{"file_path", "name", "signature", "start_line", "end_line"},
				Rows:    [][]any{{"pkg/db.go", "CozoDB.runQuery", "func (c *CozoDB) runQuery()", 42, 60}},
			}, nil
		},
		nil,
	)
	ctx := setupTest(t)

	result, err := FindFunction(ctx, client, FindFunctionArgs{Name: "runQuery", ExactMatch: false})
	assertNoError(t, err)

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Text)
	}
	// The query should use case-insensitive regex
	assertContains(t, capturedScript, "regex_matches")
	assertContains(t, capturedScript, "(?i)")
	assertContains(t, result.Text, "CozoDB.runQuery")
}

// Test FindFunction exact match still uses direct comparison
func TestFindFunction_ExactMatchStillExact(t *testing.T) {
	var capturedScript string
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			capturedScript = script
			return &QueryResult{
				Headers: []string{"file_path", "name", "signature", "start_line", "end_line"},
				Rows:    [][]any{{"pkg/db.go", "RunQuery", "func RunQuery()", 10, 20}},
			}, nil
		},
		nil,
	)
	ctx := setupTest(t)

	_, err := FindFunction(ctx, client, FindFunctionArgs{Name: "RunQuery", ExactMatch: true})
	assertNoError(t, err)

	// Exact match should use name = "...", not regex
	if !strings.Contains(capturedScript, "name = ") {
		t.Error("exact_match=true should use direct name comparison")
	}
}

// Test FindBySignature with param type
func TestFindBySignature_ParamType(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "signature", "start_line"},
		[][]any{
			{"Writer.StoreFact", "pkg/tools/writer.go", "func (w *Writer) StoreFact(backend storage.Backend, fact string) error", 45},
			{"ProcessBatch", "pkg/ingestion/batch.go", "func ProcessBatch(b *storage.Backend, items []Item) error", 23},
		},
	)
	ctx := setupTest(t)

	result, err := FindBySignature(ctx, client, FindBySignatureArgs{
		ParamType: "Backend",
		Limit:     20,
	})
	assertNoError(t, err)

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Text)
	}
	assertContains(t, result.Text, "parameter type `Backend`")
	assertContains(t, result.Text, "Writer.StoreFact")
	assertContains(t, result.Text, "ProcessBatch")
}

// Test FindBySignature with return type
func TestFindBySignature_ReturnType(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "signature", "start_line"},
		[][]any{
			{"NewClient", "pkg/client.go", "func NewClient(url string) *Client", 10},
		},
	)
	ctx := setupTest(t)

	result, err := FindBySignature(ctx, client, FindBySignatureArgs{
		ReturnType: "Client",
		Limit:      20,
	})
	assertNoError(t, err)

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Text)
	}
	assertContains(t, result.Text, "return type `Client`")
	assertContains(t, result.Text, "NewClient")
}

// Test FindBySignature with no results
func TestFindBySignature_NoResults(t *testing.T) {
	client := NewMockClientEmpty()
	ctx := setupTest(t)

	result, err := FindBySignature(ctx, client, FindBySignatureArgs{
		ParamType: "NonExistentType",
		Limit:     20,
	})
	assertNoError(t, err)

	assertContains(t, result.Text, "No matching functions found")
}

// Test FindBySignature requires at least one filter
func TestFindBySignature_RequiresFilter(t *testing.T) {
	ctx := setupTest(t)

	result, err := FindBySignature(ctx, nil, FindBySignatureArgs{})
	assertNoError(t, err)

	if !result.IsError {
		t.Error("expected error when both param_type and return_type are empty")
	}
	assertContains(t, result.Text, "required")
}

// Test extractReturnPart helper
func TestExtractReturnPart(t *testing.T) {
	tests := []struct {
		sig  string
		want string
	}{
		{"func Foo() error", "error"},
		{"func (s *Server) Run(ctx Context) error", "error"},
		{"func NewClient(url string) *Client", "*Client"},
		{"func Process(x int) (int, error)", "(int, error)"},
		{"func NoReturn()", ""},
	}

	for _, tt := range tests {
		got := extractReturnPart(tt.sig)
		if got != tt.want {
			t.Errorf("extractReturnPart(%q) = %q, want %q", tt.sig, got, tt.want)
		}
	}
}

// Test containsCaseInsensitive helper
func TestContainsCaseInsensitive(t *testing.T) {
	tests := []struct {
		s, substr string
		want      bool
	}{
		{"Hello World", "hello", true},
		{"func() error", "Error", true},
		{"*Client", "client", true},
		{"nothing", "xyz", false},
	}

	for _, tt := range tests {
		got := containsCaseInsensitive(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("containsCaseInsensitive(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}
