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

// Integration tests for code.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestGetFunctionCode_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest(c *gin.Context)",
		`func HandleRequest(c *gin.Context) {
	id := c.Param("id")
	result, err := service.Get(id)
	if err != nil {
		c.JSON(500, err)
		return
	}
	c.JSON(200, result)
}`, 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name         string
		args         GetFunctionCodeArgs
		wantContain  []string
		wantNotEmpty bool
	}{
		{
			name: "get function code by exact name",
			args: GetFunctionCodeArgs{
				FunctionName: "HandleRequest",
			},
			wantContain:  []string{"HandleRequest", "c.JSON", "handler.go"},
			wantNotEmpty: true,
		},
		{
			name: "get function code with full_code",
			args: GetFunctionCodeArgs{
				FunctionName: "HandleRequest",
				FullCode:     true,
			},
			wantContain:  []string{"HandleRequest", "service.Get"},
			wantNotEmpty: true,
		},
		{
			name: "function not found",
			args: GetFunctionCodeArgs{
				FunctionName: "NonExistent",
			},
			wantContain: []string{"not found"},
		},
		{
			name: "empty name error",
			args: GetFunctionCodeArgs{
				FunctionName: "",
			},
			wantContain: []string{"cannot be empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetFunctionCode(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("GetFunctionCode() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("GetFunctionCode() should contain %q, got:\n%s", want, result.Text)
				}
			}

			if tt.wantNotEmpty && result.Text == "" {
				t.Error("GetFunctionCode() returned empty result")
			}
		})
	}
}

func TestListFunctionsInFile_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFile(t, db, "file1", "internal/handler.go", "go")
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest(c *gin.Context)", "func HandleRequest(c *gin.Context) { }", 10)
	insertTestFunction(t, db, "func2", "HandleUser", "internal/handler.go",
		"func HandleUser(c *gin.Context)", "func HandleUser(c *gin.Context) { }", 30)
	insertTestFunction(t, db, "func3", "ProcessData", "internal/service.go",
		"func ProcessData()", "func ProcessData() { }", 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        ListFunctionsInFileArgs
		wantContain []string
		wantExclude []string
	}{
		{
			name: "list functions in handler.go",
			args: ListFunctionsInFileArgs{
				FilePath: "handler.go",
			},
			wantContain: []string{"HandleRequest", "HandleUser"},
			wantExclude: []string{"ProcessData"},
		},
		{
			name: "list functions with full path",
			args: ListFunctionsInFileArgs{
				FilePath: "internal/handler.go",
			},
			wantContain: []string{"HandleRequest", "HandleUser"},
		},
		{
			name: "file not found",
			args: ListFunctionsInFileArgs{
				FilePath: "nonexistent.go",
			},
			wantContain: []string{"No functions found", "NOT in the index"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ListFunctionsInFile(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("ListFunctionsInFile() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("ListFunctionsInFile() should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("ListFunctionsInFile() should NOT contain %q", exclude)
				}
			}
		})
	}
}

func TestGetCallGraph_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with call relationships
	insertTestFunction(t, db, "func1", "main", "cmd/main.go",
		"func main()", "func main() { handleRequest() }", 1)
	insertTestFunction(t, db, "func2", "handleRequest", "internal/handler.go",
		"func handleRequest()", "func handleRequest() { processData() }", 10)
	insertTestFunction(t, db, "func3", "processData", "internal/service.go",
		"func processData()", "func processData() { }", 20)

	// Insert call relationships
	insertTestCall(t, db, "call1", "func1", "func2") // main -> handleRequest
	insertTestCall(t, db, "call2", "func2", "func3") // handleRequest -> processData

	client := NewTestCIEClient(db)
	ctx := context.Background()

	result, err := GetCallGraph(ctx, client, GetCallGraphArgs{FunctionName: "handleRequest"})
	if err != nil {
		t.Fatalf("GetCallGraph() error = %v", err)
	}

	if result.IsError {
		t.Errorf("GetCallGraph() returned error: %s", result.Text)
		return
	}

	// Should contain both callers and callees sections
	if !strings.Contains(result.Text, "Callers") {
		t.Error("GetCallGraph() should contain 'Callers' section")
	}
	if !strings.Contains(result.Text, "Callees") {
		t.Error("GetCallGraph() should contain 'Callees' section")
	}
	if !strings.Contains(result.Text, "main") {
		t.Error("GetCallGraph() should show 'main' as caller")
	}
	if !strings.Contains(result.Text, "processData") {
		t.Error("GetCallGraph() should show 'processData' as callee")
	}
}

func TestFindSimilarFunctions_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest()", "func HandleRequest() { }", 10)
	insertTestFunction(t, db, "func2", "HandleUser", "internal/handler.go",
		"func HandleUser()", "func HandleUser() { }", 20)
	insertTestFunction(t, db, "func3", "HandleOrder", "internal/handler.go",
		"func HandleOrder()", "func HandleOrder() { }", 30)
	insertTestFunction(t, db, "func4", "ProcessData", "internal/service.go",
		"func ProcessData()", "func ProcessData() { }", 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		pattern     string
		wantContain []string
		wantExclude []string
	}{
		{
			name:        "find Handle functions",
			pattern:     "Handle",
			wantContain: []string{"HandleRequest", "HandleUser", "HandleOrder"},
			wantExclude: []string{"ProcessData"},
		},
		{
			name:        "find Process functions",
			pattern:     "Process",
			wantContain: []string{"ProcessData"},
			wantExclude: []string{"Handle"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindSimilarFunctions(ctx, client, FindSimilarFunctionsArgs{Pattern: tt.pattern})
			if err != nil {
				t.Fatalf("FindSimilarFunctions() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("FindSimilarFunctions() should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("FindSimilarFunctions() should NOT contain %q", exclude)
				}
			}
		})
	}
}

func TestGetFileSummary_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with types and functions
	insertTestFile(t, db, "file1", "internal/handler.go", "go")

	// Insert types
	insertTestType(t, db, "type1", "Handler", "struct", "internal/handler.go",
		"type Handler struct { service *Service }", 5)
	insertTestType(t, db, "type2", "RequestHandler", "interface", "internal/handler.go",
		"type RequestHandler interface { Handle() }", 15)

	// Insert functions and methods
	insertTestFunction(t, db, "func1", "NewHandler", "internal/handler.go",
		"func NewHandler() *Handler", "func NewHandler() *Handler { return &Handler{} }", 25)
	insertTestFunction(t, db, "func2", "Handler.Handle", "internal/handler.go",
		"func (h *Handler) Handle()", "func (h *Handler) Handle() { }", 35)
	insertTestFunction(t, db, "func3", "Handler.Process", "internal/handler.go",
		"func (h *Handler) Process()", "func (h *Handler) Process() { }", 45)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	result, err := GetFileSummary(ctx, client, GetFileSummaryArgs{FilePath: "handler.go"})
	if err != nil {
		t.Fatalf("GetFileSummary() error = %v", err)
	}

	if result.IsError {
		t.Errorf("GetFileSummary() returned error: %s", result.Text)
		return
	}

	// Check for types section
	if !strings.Contains(result.Text, "Types") {
		t.Error("GetFileSummary() should contain 'Types' section")
	}
	if !strings.Contains(result.Text, "Handler") {
		t.Error("GetFileSummary() should contain 'Handler' type")
	}
	if !strings.Contains(result.Text, "struct") {
		t.Error("GetFileSummary() should show 'struct' kind")
	}

	// Check for functions section
	if !strings.Contains(result.Text, "Functions") {
		t.Error("GetFileSummary() should contain 'Functions' section")
	}
	if !strings.Contains(result.Text, "NewHandler") {
		t.Error("GetFileSummary() should contain 'NewHandler' function")
	}

	// Check for methods section
	if !strings.Contains(result.Text, "Methods") {
		t.Error("GetFileSummary() should contain 'Methods' section")
	}
	if !strings.Contains(result.Text, "Handler.Handle") {
		t.Error("GetFileSummary() should contain 'Handler.Handle' method")
	}
}
