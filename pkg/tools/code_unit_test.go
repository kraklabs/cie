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
	"strings"
	"testing"
)

func TestGetFunctionCode_Unit(t *testing.T) {
	tests := []struct {
		name        string
		args        GetFunctionCodeArgs
		setupMock   func() Querier
		wantContain []string
		wantErr     bool
	}{
		{
			name: "exact_match",
			args: GetFunctionCodeArgs{
				FunctionName: "HandleRequest",
			},
			setupMock: func() Querier {
				headers := []string{"name", "file_path", "signature", "code_text", "start_line", "end_line"}
				rows := [][]any{
					{"HandleRequest", "api/handler.go", "func HandleRequest()", "func HandleRequest() {\n\treturn nil\n}", int64(10), int64(12)},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"HandleRequest", "api/handler.go", "func HandleRequest"},
		},
		{
			name: "partial_match_fallback",
			args: GetFunctionCodeArgs{
				FunctionName: "Handle",
			},
			setupMock: func() Querier {
				// First query returns empty (exact match fails)
				// Second query returns partial match
				callCount := 0
				return NewMockClientCustom(
					func(ctx context.Context, script string) (*QueryResult, error) {
						callCount++
						if callCount == 1 {
							// Exact match query returns nothing
							return &QueryResult{
								Headers: []string{"name", "file_path", "signature", "code_text", "start_line", "end_line"},
								Rows:    [][]any{},
							}, nil
						}
						// Partial match query returns result
						return &QueryResult{
							Headers: []string{"name", "file_path", "signature", "code_text", "start_line", "end_line"},
							Rows: [][]any{
								{"HandleRequest", "api/handler.go", "func HandleRequest()", "func HandleRequest() { return nil }", int64(10), int64(12)},
							},
						}, nil
					},
					nil,
				)
			},
			wantContain: []string{"HandleRequest"},
		},
		{
			name: "long_code_truncated",
			args: GetFunctionCodeArgs{
				FunctionName: "VeryLongFunction",
				FullCode:     false,
			},
			setupMock: func() Querier {
				// Create code >3000 chars
				longCode := "func VeryLongFunction() {\n"
				for i := 0; i < 150; i++ {
					longCode += "\t// This is a very long comment that will push the code over 3000 characters\n"
				}
				longCode += "}"
				headers := []string{"name", "file_path", "signature", "code_text", "start_line", "end_line"}
				rows := [][]any{
					{"VeryLongFunction", "utils/long.go", "func VeryLongFunction()", longCode, int64(1), int64(200)},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"truncated", "utils/long.go:1-200"},
		},
		{
			name: "long_code_full",
			args: GetFunctionCodeArgs{
				FunctionName: "VeryLongFunction",
				FullCode:     true,
			},
			setupMock: func() Querier {
				// Create code >3000 chars
				longCode := "func VeryLongFunction() {\n"
				for i := 0; i < 150; i++ {
					longCode += "\t// This is a very long comment that will push the code over 3000 characters\n"
				}
				longCode += "}"
				headers := []string{"name", "file_path", "signature", "code_text", "start_line", "end_line"}
				rows := [][]any{
					{"VeryLongFunction", "utils/long.go", "func VeryLongFunction()", longCode, int64(1), int64(200)},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"VeryLongFunction", "This is a very long comment"},
		},
		{
			name: "not_found",
			args: GetFunctionCodeArgs{
				FunctionName: "NonExistent",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"not found"},
		},
		{
			name: "empty_name",
			args: GetFunctionCodeArgs{
				FunctionName: "",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"cannot be empty"},
		},
		{
			name: "query_error",
			args: GetFunctionCodeArgs{
				FunctionName: "SomeFunction",
			},
			setupMock: func() Querier {
				return NewMockClientWithError(errors.New("database connection failed"))
			},
			wantContain: []string{"Query error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupMock()

			result, err := GetFunctionCode(ctx, client, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetFunctionCode() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetFunctionCode() unexpected error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("GetFunctionCode() result should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestListFunctionsInFile_Unit(t *testing.T) {
	tests := []struct {
		name        string
		args        ListFunctionsInFileArgs
		setupMock   func() Querier
		wantContain []string
		wantExclude []string
		wantErr     bool
	}{
		{
			name: "suffix_match",
			args: ListFunctionsInFileArgs{
				FilePath: "handler.go",
			},
			setupMock: func() Querier {
				headers := []string{"name", "signature", "start_line", "file_path"}
				rows := [][]any{
					{"init", "func init()", int64(3), "api/handler.go"},
					{"HandleRequest", "func HandleRequest()", int64(10), "api/handler.go"},
					{"HandleResponse", "func HandleResponse()", int64(20), "api/handler.go"},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"HandleRequest", "HandleResponse", "init", "Line 3", "Line 10", "Line 20"},
		},
		{
			name: "single_function",
			args: ListFunctionsInFileArgs{
				FilePath: "main.go",
			},
			setupMock: func() Querier {
				headers := []string{"name", "signature", "start_line", "file_path"}
				rows := [][]any{
					{"main", "func main()", int64(5), "cmd/main.go"},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"main", "Line 5"},
		},
		{
			name: "no_functions_indexed_file",
			args: ListFunctionsInFileArgs{
				FilePath: "types.go",
			},
			setupMock: func() Querier {
				// First query (suffix match) returns empty
				// Second query (regex match) returns empty
				return NewMockClientEmpty()
			},
			wantContain: []string{"No functions found", "NOT in the index"},
		},
		{
			name: "file_not_found",
			args: ListFunctionsInFileArgs{
				FilePath: "nonexistent.go",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"No functions found", "NOT in the index"},
		},
		{
			name: "empty_path",
			args: ListFunctionsInFileArgs{
				FilePath: "",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"cannot be empty"},
		},
		{
			name: "query_error",
			args: ListFunctionsInFileArgs{
				FilePath: "some.go",
			},
			setupMock: func() Querier {
				return NewMockClientWithError(errors.New("connection timeout"))
			},
			wantContain: []string{"Query error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupMock()

			result, err := ListFunctionsInFile(ctx, client, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ListFunctionsInFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ListFunctionsInFile() unexpected error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("ListFunctionsInFile() result should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("ListFunctionsInFile() result should NOT contain %q", exclude)
				}
			}
		})
	}
}

func TestGetCallGraph_Unit(t *testing.T) {
	tests := []struct {
		name        string
		args        GetCallGraphArgs
		setupMock   func() Querier
		wantContain []string
		wantErr     bool
	}{
		{
			name: "empty_name",
			args: GetCallGraphArgs{
				FunctionName: "",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"cannot be empty"},
		},
		{
			name: "basic_call_no_results",
			args: GetCallGraphArgs{
				FunctionName: "IsolatedFunc",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"Callers", "Callees"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupMock()

			result, err := GetCallGraph(ctx, client, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetCallGraph() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetCallGraph() unexpected error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("GetCallGraph() result should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestFindSimilarFunctions_Unit(t *testing.T) {
	tests := []struct {
		name        string
		args        FindSimilarFunctionsArgs
		setupMock   func() Querier
		wantContain []string
		wantExclude []string
		wantErr     bool
	}{
		{
			name: "multiple_matches",
			args: FindSimilarFunctionsArgs{
				Pattern: "Handle",
			},
			setupMock: func() Querier {
				headers := []string{"name", "file_path", "signature"}
				rows := [][]any{
					{"HandleRequest", "api/handler.go", "func HandleRequest()"},
					{"HandleResponse", "api/handler.go", "func HandleResponse()"},
					{"HandleError", "api/error.go", "func HandleError()"},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"HandleRequest", "HandleResponse", "HandleError"},
		},
		{
			name: "single_match",
			args: FindSimilarFunctionsArgs{
				Pattern: "main",
			},
			setupMock: func() Querier {
				headers := []string{"name", "file_path", "signature"}
				rows := [][]any{
					{"main", "cmd/main.go", "func main()"},
				}
				return NewMockClientWithResults(headers, rows)
			},
			wantContain: []string{"main", "cmd/main.go"},
		},
		{
			name: "no_matches",
			args: FindSimilarFunctionsArgs{
				Pattern: "NonExistent",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"No functions matching"},
		},
		{
			name: "empty_pattern",
			args: FindSimilarFunctionsArgs{
				Pattern: "",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"cannot be empty"},
		},
		{
			name: "query_error",
			args: FindSimilarFunctionsArgs{
				Pattern: "SomePattern",
			},
			setupMock: func() Querier {
				return NewMockClientWithError(errors.New("query timeout"))
			},
			wantContain: []string{"Query error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupMock()

			result, err := FindSimilarFunctions(ctx, client, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FindSimilarFunctions() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("FindSimilarFunctions() unexpected error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("FindSimilarFunctions() result should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("FindSimilarFunctions() result should NOT contain %q", exclude)
				}
			}
		})
	}
}

func TestGetFileSummary_Unit(t *testing.T) {
	tests := []struct {
		name        string
		args        GetFileSummaryArgs
		setupMock   func() Querier
		wantContain []string
		wantErr     bool
	}{
		{
			name: "complete_file",
			args: GetFileSummaryArgs{
				FilePath: "handler.go",
			},
			setupMock: func() Querier {
				callCount := 0
				return NewMockClientCustom(
					func(ctx context.Context, script string) (*QueryResult, error) {
						callCount++
						if callCount == 1 {
							// First call: types query
							return &QueryResult{
								Headers: []string{"name", "kind", "signature", "start_line"},
								Rows: [][]any{
									{"Handler", "struct", "type Handler struct", int64(5)},
									{"RequestHandler", "interface", "type RequestHandler interface", int64(15)},
								},
							}, nil
						}
						// Second call: functions query
						return &QueryResult{
							Headers: []string{"name", "signature", "start_line"},
							Rows: [][]any{
								{"NewHandler", "func NewHandler() *Handler", int64(25)},
								{"Handler.Handle", "func (h *Handler) Handle()", int64(35)},
								{"Handler.Process", "func (h *Handler) Process()", int64(45)},
							},
						}, nil
					},
					nil,
				)
			},
			wantContain: []string{"Types", "Handler", "struct", "Functions", "NewHandler", "Methods", "Handler.Handle", "Handler.Process"},
		},
		{
			name: "types_only",
			args: GetFileSummaryArgs{
				FilePath: "types.go",
			},
			setupMock: func() Querier {
				callCount := 0
				return NewMockClientCustom(
					func(ctx context.Context, script string) (*QueryResult, error) {
						callCount++
						if callCount == 1 {
							// Types query returns results
							return &QueryResult{
								Headers: []string{"name", "kind", "signature", "start_line"},
								Rows: [][]any{
									{"User", "struct", "type User struct", int64(10)},
								},
							}, nil
						}
						// Functions query returns empty
						return &QueryResult{
							Headers: []string{"name", "signature", "start_line"},
							Rows:    [][]any{},
						}, nil
					},
					nil,
				)
			},
			wantContain: []string{"Types", "User", "struct"},
		},
		{
			name: "functions_only",
			args: GetFileSummaryArgs{
				FilePath: "utils.go",
			},
			setupMock: func() Querier {
				callCount := 0
				return NewMockClientCustom(
					func(ctx context.Context, script string) (*QueryResult, error) {
						callCount++
						if callCount == 1 {
							// Types query returns empty
							return &QueryResult{
								Headers: []string{"name", "kind", "signature", "start_line"},
								Rows:    [][]any{},
							}, nil
						}
						// Functions query returns results (no methods, no "." in name)
						return &QueryResult{
							Headers: []string{"name", "signature", "start_line"},
							Rows: [][]any{
								{"FormatString", "func FormatString(s string) string", int64(5)},
								{"ParseInt", "func ParseInt(s string) int", int64(15)},
							},
						}, nil
					},
					nil,
				)
			},
			wantContain: []string{"Functions", "FormatString", "ParseInt"},
		},
		{
			name: "methods_only",
			args: GetFileSummaryArgs{
				FilePath: "methods.go",
			},
			setupMock: func() Querier {
				callCount := 0
				return NewMockClientCustom(
					func(ctx context.Context, script string) (*QueryResult, error) {
						callCount++
						if callCount == 1 {
							// Types query returns empty
							return &QueryResult{
								Headers: []string{"name", "kind", "signature", "start_line"},
								Rows:    [][]any{},
							}, nil
						}
						// Functions query returns methods (with "." in name)
						return &QueryResult{
							Headers: []string{"name", "signature", "start_line"},
							Rows: [][]any{
								{"User.Save", "func (u *User) Save()", int64(10)},
								{"User.Delete", "func (u *User) Delete()", int64(20)},
							},
						}, nil
					},
					nil,
				)
			},
			wantContain: []string{"Methods", "User.Save", "User.Delete"},
		},
		{
			name: "empty_file",
			args: GetFileSummaryArgs{
				FilePath: "empty.go",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"No entities found"},
		},
		{
			name: "not_found",
			args: GetFileSummaryArgs{
				FilePath: "nonexistent.go",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"No entities found"},
		},
		{
			name: "empty_path",
			args: GetFileSummaryArgs{
				FilePath: "",
			},
			setupMock: func() Querier {
				return NewMockClientEmpty()
			},
			wantContain: []string{"cannot be empty"},
		},
		{
			name: "query_error",
			args: GetFileSummaryArgs{
				FilePath: "some.go",
			},
			setupMock: func() Querier {
				return NewMockClientWithError(errors.New("database error"))
			},
			wantContain: []string{"Query error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := tt.setupMock()

			result, err := GetFileSummary(ctx, client, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetFileSummary() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("GetFileSummary() unexpected error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("GetFileSummary() result should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}
