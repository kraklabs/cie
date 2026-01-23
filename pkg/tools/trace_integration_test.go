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

// Integration tests for trace.go functions.
// Run with: go test -tags=cozodb ./modules/cie/pkg/tools/...

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
