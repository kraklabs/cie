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

// Integration tests for trace.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestTracePath_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data: main -> handleRequest -> processData -> saveToDb
	insertTestFunction(t, db, "func1", "main", "cmd/server/main.go",
		"func main()", "func main() { handleRequest() }", 1)
	insertTestFunction(t, db, "func2", "handleRequest", "internal/handler.go",
		"func handleRequest()", "func handleRequest() { processData() }", 10)
	insertTestFunction(t, db, "func3", "processData", "internal/service.go",
		"func processData()", "func processData() { saveToDb() }", 20)
	insertTestFunction(t, db, "func4", "saveToDb", "internal/db.go",
		"func saveToDb()", "func saveToDb() { }", 30)

	// Insert call relationships
	insertTestCall(t, db, "call1", "func1", "func2") // main -> handleRequest
	insertTestCall(t, db, "call2", "func2", "func3") // handleRequest -> processData
	insertTestCall(t, db, "call3", "func3", "func4") // processData -> saveToDb

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        TracePathArgs
		wantContain []string
		wantNotFind bool
	}{
		{
			name: "trace from main to saveToDb",
			args: TracePathArgs{
				Target:   "saveToDb",
				Source:   "main",
				MaxPaths: 3,
				MaxDepth: 10,
			},
			wantContain: []string{"main", "handleRequest", "processData", "saveToDb"},
		},
		{
			name: "trace from handleRequest to saveToDb",
			args: TracePathArgs{
				Target:   "saveToDb",
				Source:   "handleRequest",
				MaxPaths: 3,
				MaxDepth: 10,
			},
			wantContain: []string{"handleRequest", "processData", "saveToDb"},
		},
		{
			name: "auto-detect entry points",
			args: TracePathArgs{
				Target:   "processData",
				MaxPaths: 3,
				MaxDepth: 10,
			},
			wantContain: []string{"main"}, // should detect main as entry point
		},
		{
			name: "target not found",
			args: TracePathArgs{
				Target:   "nonexistent",
				Source:   "main",
				MaxPaths: 3,
				MaxDepth: 10,
			},
			wantContain: []string{"not found"},
			wantNotFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := TracePath(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("TracePath() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("TracePath() should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestTracePath_EmptyTarget(t *testing.T) {
	db := openTestDB(t)
	client := NewTestCIEClient(db)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{Target: ""})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	if !result.IsError {
		t.Error("TracePath() should return error for empty target")
	}
	if !strings.Contains(result.Text, "required") {
		t.Errorf("TracePath() error should mention 'required', got: %s", result.Text)
	}
}

func TestDetectEntryPoints_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with entry point patterns
	insertTestFunction(t, db, "func1", "main", "cmd/server/main.go",
		"func main()", "func main() { }", 1)
	insertTestFunction(t, db, "func2", "handleRequest", "internal/handler.go",
		"func handleRequest()", "func handleRequest() { }", 10)
	insertTestFunction(t, db, "func3", "__main__", "scripts/run.py",
		"def __main__():", "def __main__(): pass", 1)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	// Should find 'main' and '__main__' as entry points
	foundMain := false
	for _, src := range sources {
		if src.Name == "main" {
			foundMain = true
		}
	}

	if !foundMain {
		t.Error("detectEntryPoints() should find 'main' function")
	}
}

func TestFindFunctionsByName_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestFunction(t, db, "func1", "HandleRequest", "internal/handler.go",
		"func HandleRequest()", "func HandleRequest() { }", 10)
	insertTestFunction(t, db, "func2", "Service.HandleRequest", "internal/service.go",
		"func (s *Service) HandleRequest()", "func (s *Service) HandleRequest() { }", 20)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		funcName    string
		pathPattern string
		wantLen     int
	}{
		{
			name:     "find by exact name",
			funcName: "HandleRequest",
			wantLen:  2, // Both standalone and method
		},
		{
			name:        "filter by path",
			funcName:    "HandleRequest",
			pathPattern: "handler",
			wantLen:     1, // Only handler.go
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := findFunctionsByName(ctx, client, tt.funcName, tt.pathPattern)
			if len(funcs) != tt.wantLen {
				t.Errorf("findFunctionsByName() returned %d results, want %d", len(funcs), tt.wantLen)
			}
		})
	}
}

func TestTraceFuncInfo_Struct(t *testing.T) {
	info := TraceFuncInfo{
		Name:     "HandleRequest",
		FilePath: "internal/handler.go",
		Line:     "10",
	}

	if info.Name != "HandleRequest" {
		t.Error("Name not set correctly")
	}
	if info.FilePath != "internal/handler.go" {
		t.Error("FilePath not set correctly")
	}
	if info.Line != "10" {
		t.Error("Line not set correctly")
	}
}

func TestTracePathArgs_Defaults(t *testing.T) {
	args := TracePathArgs{
		Target: "saveToDb",
	}

	if args.Source != "" {
		t.Error("Default Source should be empty")
	}
	if args.PathPattern != "" {
		t.Error("Default PathPattern should be empty")
	}
	if args.MaxPaths != 0 {
		t.Error("Default MaxPaths should be 0 (filled by function)")
	}
	if args.MaxDepth != 0 {
		t.Error("Default MaxDepth should be 0 (filled by function)")
	}
}
