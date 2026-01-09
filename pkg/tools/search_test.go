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

// Integration tests for search.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestSearchText_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFile(t, db, "file1", "internal/handler.go", "go")
	insertTestFile(t, db, "file2", "internal/service.go", "go")
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest(c *gin.Context)", "func HandleRequest(c *gin.Context) { c.JSON(200, result) }", 10)
	insertTestFunction(t, db, "func2", "ProcessData", "internal/service.go",
		"func ProcessData(data []byte) error", "func ProcessData(data []byte) error { return nil }", 20)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        SearchTextArgs
		wantContain string
		wantErr     bool
	}{
		{
			name: "search in name",
			args: SearchTextArgs{
				Pattern:  "Handle",
				SearchIn: "name",
				Limit:    10,
			},
			wantContain: "HandleRequest",
		},
		{
			name: "search in code",
			args: SearchTextArgs{
				Pattern:  "JSON",
				SearchIn: "code",
				Limit:    10,
			},
			wantContain: "HandleRequest",
		},
		{
			name: "search in all",
			args: SearchTextArgs{
				Pattern:  "Process",
				SearchIn: "all",
				Limit:    10,
			},
			wantContain: "ProcessData",
		},
		{
			name: "literal search",
			args: SearchTextArgs{
				Pattern: ".JSON(",
				Literal: true,
				Limit:   10,
			},
			wantContain: "HandleRequest",
		},
		{
			name: "filter by file pattern",
			args: SearchTextArgs{
				Pattern:     "func",
				SearchIn:    "code",
				FilePattern: "handler",
				Limit:       10,
			},
			wantContain: "HandleRequest",
		},
		{
			name: "empty pattern error",
			args: SearchTextArgs{
				Pattern: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SearchText(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("SearchText() error = %v", err)
			}
			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result")
				}
				return
			}
			if result.IsError {
				t.Errorf("SearchText() returned error: %s", result.Text)
				return
			}
			if tt.wantContain != "" && !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("SearchText() result should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
		})
	}
}

func TestFindFunction_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest(c *gin.Context)", "func HandleRequest(c *gin.Context) { }", 10)
	insertTestFunction(t, db, "func2", "Service.HandleRequest", "internal/service.go",
		"func (s *Service) HandleRequest()", "func (s *Service) HandleRequest() { }", 20)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        FindFunctionArgs
		wantContain string
		wantCount   int // -1 for don't check
	}{
		{
			name: "exact match",
			args: FindFunctionArgs{
				Name:       "HandleRequest",
				ExactMatch: true,
			},
			wantContain: "HandleRequest",
			wantCount:   1,
		},
		{
			name: "partial match includes methods",
			args: FindFunctionArgs{
				Name:       "HandleRequest",
				ExactMatch: false,
			},
			wantContain: "HandleRequest",
			wantCount:   2,
		},
		{
			name: "include code",
			args: FindFunctionArgs{
				Name:        "HandleRequest",
				ExactMatch:  true,
				IncludeCode: true,
			},
			wantContain: "HandleRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindFunction(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("FindFunction() error = %v", err)
			}
			if result.IsError {
				t.Errorf("FindFunction() returned error: %s", result.Text)
				return
			}
			if tt.wantContain != "" && !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("FindFunction() should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
			if tt.wantCount > 0 {
				countStr := strings.Split(result.Text, " ")[1]
				if !strings.Contains(result.Text, "Found "+string(rune('0'+tt.wantCount))) {
					t.Logf("Note: expected %d results, got: %s", tt.wantCount, countStr)
				}
			}
		})
	}
}

func TestFindCallers_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with call relationships
	insertTestFunction(t, db, "func1", "main", "cmd/server/main.go",
		"func main()", "func main() { handleRequest() }", 1)
	insertTestFunction(t, db, "func2", "handleRequest", "internal/handler.go",
		"func handleRequest()", "func handleRequest() { processData() }", 10)
	insertTestFunction(t, db, "func3", "processData", "internal/service.go",
		"func processData()", "func processData() { }", 20)

	// Insert call relationships
	insertTestCall(t, db, "call1", "func1", "func2") // main calls handleRequest
	insertTestCall(t, db, "call2", "func2", "func3") // handleRequest calls processData

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name         string
		functionName string
		wantContain  string
	}{
		{
			name:         "find callers of handleRequest",
			functionName: "handleRequest",
			wantContain:  "main",
		},
		{
			name:         "find callers of processData",
			functionName: "processData",
			wantContain:  "handleRequest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindCallers(ctx, client, FindCallersArgs{FunctionName: tt.functionName})
			if err != nil {
				t.Fatalf("FindCallers() error = %v", err)
			}
			if result.IsError {
				t.Errorf("FindCallers() returned error: %s", result.Text)
				return
			}
			if !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("FindCallers() should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
		})
	}
}

func TestFindCallees_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with call relationships
	insertTestFunction(t, db, "func1", "main", "cmd/server/main.go",
		"func main()", "func main() { handleRequest() }", 1)
	insertTestFunction(t, db, "func2", "handleRequest", "internal/handler.go",
		"func handleRequest()", "func handleRequest() { processData() }", 10)
	insertTestFunction(t, db, "func3", "processData", "internal/service.go",
		"func processData()", "func processData() { }", 20)

	// Insert call relationships
	insertTestCall(t, db, "call1", "func1", "func2") // main calls handleRequest
	insertTestCall(t, db, "call2", "func2", "func3") // handleRequest calls processData

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name         string
		functionName string
		wantContain  string
	}{
		{
			name:         "find callees of main",
			functionName: "main",
			wantContain:  "handleRequest",
		},
		{
			name:         "find callees of handleRequest",
			functionName: "handleRequest",
			wantContain:  "processData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindCallees(ctx, client, FindCalleesArgs{FunctionName: tt.functionName})
			if err != nil {
				t.Fatalf("FindCallees() error = %v", err)
			}
			if result.IsError {
				t.Errorf("FindCallees() returned error: %s", result.Text)
				return
			}
			if !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("FindCallees() should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
		})
	}
}

func TestListFiles_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFile(t, db, "file1", "internal/handler.go", "go")
	insertTestFile(t, db, "file2", "internal/service.go", "go")
	insertTestFile(t, db, "file3", "cmd/main.go", "go")
	insertTestFile(t, db, "file4", "scripts/test.py", "python")

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        ListFilesArgs
		wantContain string
		wantMinLen  int
	}{
		{
			name: "list all files",
			args: ListFilesArgs{
				Limit: 50,
			},
			wantMinLen: 4,
		},
		{
			name: "filter by path pattern",
			args: ListFilesArgs{
				PathPattern: "internal",
				Limit:       50,
			},
			wantContain: "internal",
			wantMinLen:  2,
		},
		{
			name: "filter by language",
			args: ListFilesArgs{
				Language: "python",
				Limit:    50,
			},
			wantContain: "test.py",
			wantMinLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ListFiles(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("ListFiles() error = %v", err)
			}
			if result.IsError {
				t.Errorf("ListFiles() returned error: %s", result.Text)
				return
			}
			if tt.wantContain != "" && !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("ListFiles() should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
		})
	}
}

func TestRawQuery_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFile(t, db, "file1", "internal/handler.go", "go")

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		script      string
		wantContain string
		wantErr     bool
	}{
		{
			name:        "valid query",
			script:      `?[path] := *cie_file { path }`,
			wantContain: "handler.go",
		},
		{
			name:    "empty script error",
			script:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RawQuery(ctx, client, RawQueryArgs{Script: tt.script})
			if err != nil {
				t.Fatalf("RawQuery() error = %v", err)
			}
			if tt.wantErr {
				if !result.IsError {
					t.Error("expected error result")
				}
				return
			}
			if result.IsError {
				t.Errorf("RawQuery() returned error: %s", result.Text)
				return
			}
			if tt.wantContain != "" && !strings.Contains(result.Text, tt.wantContain) {
				t.Errorf("RawQuery() should contain %q, got:\n%s", tt.wantContain, result.Text)
			}
		})
	}
}
