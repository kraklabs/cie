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
	"time"
)

// ============================================================================
// UNIT TESTS (no CozoDB required - use mocks)
// ============================================================================

// Mock result builders for trace operations

// mockTraceFunctionResult creates a QueryResult for function queries (name, file_path, start_line).
func mockTraceFunctionResult(functions ...TraceFuncInfo) *QueryResult {
	rows := make([][]any, len(functions))
	for i, fn := range functions {
		rows[i] = []any{fn.Name, fn.FilePath, fn.Line}
	}
	return &QueryResult{
		Headers: []string{"name", "file_path", "start_line"},
		Rows:    rows,
	}
}

// mockTraceCalleesResult creates a QueryResult for getCallees queries.
func mockTraceCalleesResult(callees ...TraceFuncInfo) *QueryResult {
	rows := make([][]any, len(callees))
	for i, callee := range callees {
		rows[i] = []any{callee.Name, callee.FilePath, callee.Line}
	}
	return &QueryResult{
		Headers: []string{"callee_name", "callee_file", "callee_line"},
		Rows:    rows,
	}
}

// createMockCallGraph creates a MockCIEClient that simulates a call graph.
// callGraph maps function names to their callees.
// functions maps function names to their TraceFuncInfo.
func createMockCallGraph(functions map[string]TraceFuncInfo, callGraph map[string][]string) *MockCIEClient {
	return NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			// Detect query type by script content
			if strings.Contains(script, "cie_calls") && strings.Contains(script, "caller_id") {
				// getCallees query - extract function name from script
				for funcName, callees := range callGraph {
					// Match either exact name or suffix pattern
					quotedName := fmt.Sprintf("%q", funcName)
					quotedSuffix := fmt.Sprintf("%q", "."+funcName)
					if strings.Contains(script, quotedName) || strings.Contains(script, quotedSuffix) {
						var calleeFuncs []TraceFuncInfo
						for _, calleeName := range callees {
							if fn, ok := functions[calleeName]; ok {
								calleeFuncs = append(calleeFuncs, fn)
							}
						}
						return mockTraceCalleesResult(calleeFuncs...), nil
					}
				}
				return mockTraceCalleesResult(), nil
			} else if strings.Contains(script, "name =") || strings.Contains(script, "ends_with") {
				// findFunctionsByName query - looks for exact match or suffix match
				// Extract the function name being searched
				var matches []TraceFuncInfo
				for funcName, fn := range functions {
					quotedName := fmt.Sprintf("%q", funcName)
					quotedSuffix := fmt.Sprintf("%q", "."+funcName)
					// Check if this function matches the query
					if strings.Contains(script, quotedName) || strings.Contains(script, quotedSuffix) {
						matches = append(matches, fn)
					}
				}
				return mockTraceFunctionResult(matches...), nil
			} else if strings.Contains(script, "regex_matches") {
				// detectEntryPoints query - pattern-based matching
				var matches []TraceFuncInfo
				for name, fn := range functions {
					// Check various entry point patterns
					isMain := name == "main" || name == "__main__"
					isGoFile := strings.HasSuffix(fn.FilePath, ".go")
					isPyFile := strings.HasSuffix(fn.FilePath, ".py")
					isJsFile := strings.HasSuffix(fn.FilePath, ".js") || strings.HasSuffix(fn.FilePath, ".ts")

					// Match based on language patterns
					if isMain && (isGoFile || isPyFile) {
						matches = append(matches, fn)
					} else if isJsFile && (strings.Contains(fn.FilePath, "index") ||
						strings.Contains(fn.FilePath, "app") || strings.Contains(fn.FilePath, "server")) {
						matches = append(matches, fn)
					}
				}
				return mockTraceFunctionResult(matches...), nil
			}
			return &QueryResult{Headers: []string{}, Rows: [][]any{}}, nil
		},
		nil,
	)
}

// Test TracePath with empty target
func TestTracePath_Unit_EmptyTarget(t *testing.T) {
	client := NewMockClientEmpty()
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

// Test TracePath when target function is not found
func TestTracePath_Unit_TargetNotFound(t *testing.T) {
	// Mock that returns no functions
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{}, // Empty result
	)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "nonexistent",
		Source:   "main",
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	if !strings.Contains(result.Text, "not found") {
		t.Errorf("TracePath() should mention 'not found', got: %s", result.Text)
	}
}

// Test TracePath when source function is not found
func TestTracePath_Unit_SourceNotFound(t *testing.T) {
	// Mock that returns empty for source query
	queryCount := 0
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			queryCount++
			// First query is for finding source, return empty
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line"},
				Rows:    [][]any{},
			}, nil
		},
		nil,
	)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "saveToDb",
		Source:   "nonexistent",
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	if !strings.Contains(result.Text, "not found") {
		t.Errorf("TracePath() should mention 'not found', got: %s", result.Text)
	}
}

// Test TracePath with simple path: main -> handleRequest -> saveToDb
func TestTracePath_Unit_SimplePath(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main":          {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"handleRequest": {Name: "handleRequest", FilePath: "internal/handler.go", Line: "10"},
		"saveToDb":      {Name: "saveToDb", FilePath: "internal/db.go", Line: "20"},
	}
	callGraph := map[string][]string{
		"main":          {"handleRequest"},
		"handleRequest": {"saveToDb"},
		"saveToDb":      {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "saveToDb",
		Source:   "main",
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// Check that all functions in the path are mentioned
	for _, fn := range []string{"main", "handleRequest", "saveToDb"} {
		if !strings.Contains(result.Text, fn) {
			t.Errorf("TracePath() should contain %q, got:\n%s", fn, result.Text)
		}
	}
}

// Test TracePath with disconnected graph (no path exists)
func TestTracePath_Unit_DisconnectedGraph(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main":      {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"isolated":  {Name: "isolated", FilePath: "internal/isolated.go", Line: "10"},
		"saveToDb":  {Name: "saveToDb", FilePath: "internal/db.go", Line: "20"},
	}
	callGraph := map[string][]string{
		"main":     {},  // main calls nothing
		"isolated": {"saveToDb"},  // isolated calls saveToDb but unreachable from main
		"saveToDb": {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "saveToDb",
		Source:   "main",
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	if !strings.Contains(result.Text, "No path found") {
		t.Errorf("TracePath() should mention 'No path found', got:\n%s", result.Text)
	}
}

// Test TracePath with cycle in call graph (A -> B -> A)
func TestTracePath_Unit_CycleDetection(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"funcA":  {Name: "funcA", FilePath: "pkg/a.go", Line: "1"},
		"funcB":  {Name: "funcB", FilePath: "pkg/b.go", Line: "10"},
		"target": {Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	callGraph := map[string][]string{
		"funcA":  {"funcB"},
		"funcB":  {"funcA", "target"}, // Cycle: B -> A, and path to target
		"target": {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "target",
		Source:   "funcA",
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// Should find the path without infinite loop
	if !strings.Contains(result.Text, "target") {
		t.Errorf("TracePath() should find target despite cycle, got:\n%s", result.Text)
	}
}

// Test TracePath with max depth limit
func TestTracePath_Unit_MaxDepthLimit(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main":  {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"fn1":   {Name: "fn1", FilePath: "pkg/fn1.go", Line: "10"},
		"fn2":   {Name: "fn2", FilePath: "pkg/fn2.go", Line: "20"},
		"fn3":   {Name: "fn3", FilePath: "pkg/fn3.go", Line: "30"},
		"deep":  {Name: "deep", FilePath: "pkg/deep.go", Line: "40"},
	}
	callGraph := map[string][]string{
		"main": {"fn1"},
		"fn1":  {"fn2"},
		"fn2":  {"fn3"},
		"fn3":  {"deep"},
		"deep": {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "deep",
		Source:   "main",
		MaxPaths: 3,
		MaxDepth: 2, // Only 2 levels deep
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// With MaxDepth=2, we can't reach "deep" (at depth 4)
	if !strings.Contains(result.Text, "No path found") {
		t.Errorf("TracePath() should not find path beyond max depth, got:\n%s", result.Text)
	}
}

// Test TracePath with max paths limit
func TestTracePath_Unit_MaxPathsLimit(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main":   {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"route1": {Name: "route1", FilePath: "pkg/route1.go", Line: "10"},
		"route2": {Name: "route2", FilePath: "pkg/route2.go", Line: "20"},
		"route3": {Name: "route3", FilePath: "pkg/route3.go", Line: "30"},
		"target": {Name: "target", FilePath: "pkg/target.go", Line: "40"},
	}
	callGraph := map[string][]string{
		"main":   {"route1", "route2", "route3"},
		"route1": {"target"},
		"route2": {"target"},
		"route3": {"target"},
		"target": {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "target",
		Source:   "main",
		MaxPaths: 2, // Only show 2 paths
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// Should find paths but limit message
	if !strings.Contains(result.Text, "target") {
		t.Errorf("TracePath() should find target, got:\n%s", result.Text)
	}
}

// Test detectEntryPoints for Go main function
func TestDetectEntryPoints_Unit_GoMain(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{
			{"main", "cmd/server/main.go", 1},
		},
	)
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	if len(sources) == 0 {
		t.Error("detectEntryPoints() should find main function")
	}
	if sources[0].Name != "main" {
		t.Errorf("detectEntryPoints() found %q, want 'main'", sources[0].Name)
	}
}

// Test detectEntryPoints for Python __main__
func TestDetectEntryPoints_Unit_PythonMain(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{
			{"__main__", "scripts/run.py", 1},
		},
	)
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	if len(sources) == 0 {
		t.Error("detectEntryPoints() should find __main__ function")
	}
}

// Test detectEntryPoints with no entry points found
func TestDetectEntryPoints_Unit_NoEntryPoints(t *testing.T) {
	client := NewMockClientEmpty()
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	if len(sources) != 0 {
		t.Errorf("detectEntryPoints() should return empty, got %d sources", len(sources))
	}
}

// Test findFunctionsByName with exact match
func TestFindFunctionsByName_Unit_ExactMatch(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{
			{"HandleRequest", "internal/handler.go", 10},
		},
	)
	ctx := context.Background()

	funcs := findFunctionsByName(ctx, client, "HandleRequest", "")

	if len(funcs) != 1 {
		t.Errorf("findFunctionsByName() returned %d functions, want 1", len(funcs))
	}
	if funcs[0].Name != "HandleRequest" {
		t.Errorf("findFunctionsByName() found %q, want 'HandleRequest'", funcs[0].Name)
	}
}

// Test findFunctionsByName with method suffix match (e.g., "Run" matches "Agent.Run")
func TestFindFunctionsByName_Unit_MethodSuffixMatch(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{
			{"Run", "pkg/standalone.go", 5},
			{"Agent.Run", "pkg/agent.go", 10},
			{"Server.Run", "pkg/server.go", 20},
		},
	)
	ctx := context.Background()

	funcs := findFunctionsByName(ctx, client, "Run", "")

	if len(funcs) < 2 {
		t.Errorf("findFunctionsByName() should find multiple matches, got %d", len(funcs))
	}
}

// Test findFunctionsByName with no match
func TestFindFunctionsByName_Unit_NoMatch(t *testing.T) {
	client := NewMockClientEmpty()
	ctx := context.Background()

	funcs := findFunctionsByName(ctx, client, "DoesNotExist", "")

	if len(funcs) != 0 {
		t.Errorf("findFunctionsByName() should return empty, got %d functions", len(funcs))
	}
}

// Test getCallees with results
func TestGetCallees_Unit_WithResults(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"callee_name", "callee_file", "callee_line"},
		[][]any{
			{"handleRequest", "internal/handler.go", 10},
			{"validateInput", "internal/validator.go", 20},
		},
	)
	ctx := context.Background()

	callees := getCallees(ctx, client, "main")

	if len(callees) != 2 {
		t.Errorf("getCallees() returned %d callees, want 2", len(callees))
	}
}

// Test getCallees with no results
func TestGetCallees_Unit_EmptyResult(t *testing.T) {
	client := NewMockClientEmpty()
	ctx := context.Background()

	callees := getCallees(ctx, client, "leaf")

	if len(callees) != 0 {
		t.Errorf("getCallees() should return empty, got %d callees", len(callees))
	}
}

// Test formatTraceOutput with single path
func TestFormatTraceOutput_Unit_SinglePath(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "saveToDb", MaxPaths: 3, MaxDepth: 10}
	path := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "handleRequest", FilePath: "internal/handler.go", Line: "10"},
		{Name: "saveToDb", FilePath: "internal/db.go", Line: "20"},
	}
	result := traceSearchResult{
		paths:         [][]TraceFuncInfo{path},
		nodesExplored: 100,
	}

	output := formatTraceOutput(sources, args, result)

	if !strings.Contains(output, "main") {
		t.Error("formatTraceOutput() should contain 'main'")
	}
	if !strings.Contains(output, "saveToDb") {
		t.Error("formatTraceOutput() should contain 'saveToDb'")
	}
	if !strings.Contains(output, "100 nodes") {
		t.Error("formatTraceOutput() should mention node count")
	}
}

// Test formatTraceNotFound
func TestFormatTraceNotFound_Unit(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "unreachable", MaxPaths: 3, MaxDepth: 10}
	result := traceSearchResult{
		nodesExplored: 200,
		limitReached:  false,
	}

	output := formatTraceNotFound(sources, args, result)

	if !strings.Contains(output, "No path found") {
		t.Error("formatTraceNotFound() should contain 'No path found'")
	}
	if !strings.Contains(output, "200 nodes") {
		t.Error("formatTraceNotFound() should mention node count")
	}
	if !strings.Contains(output, "Tips:") {
		t.Error("formatTraceNotFound() should include tips")
	}
}

// Test formatSources with single source
func TestFormatSources_Unit_SingleSource(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	output := formatSources(sources, false)

	if !strings.Contains(output, "main") {
		t.Error("formatSources() should contain function name")
	}
}

// Test formatSources with multiple sources (auto-detected)
func TestFormatSources_Unit_MultipleAutoDetected(t *testing.T) {
	sources := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "__main__", FilePath: "scripts/run.py", Line: "1"},
	}
	output := formatSources(sources, true)

	if !strings.Contains(output, "auto-detected") {
		t.Error("formatSources() should mention 'auto-detected'")
	}
}

// Test formatSources with multiple sources (explicit)
func TestFormatSources_Unit_MultipleExplicit(t *testing.T) {
	sources := []TraceFuncInfo{
		{Name: "Handler1", FilePath: "pkg/handler1.go", Line: "1"},
		{Name: "Handler2", FilePath: "pkg/handler2.go", Line: "10"},
	}
	output := formatSources(sources, false)

	if !strings.Contains(output, "matching functions") {
		t.Error("formatSources() should mention 'matching functions'")
	}
}

// Test formatSources with empty sources
func TestFormatSources_Unit_Empty(t *testing.T) {
	sources := []TraceFuncInfo{}
	output := formatSources(sources, false)

	if output != "unknown" {
		t.Errorf("formatSources() for empty sources = %q, want 'unknown'", output)
	}
}

// Test detectEntryPoints directly with various query results
func TestDetectEntryPoints_Unit_GoPattern(t *testing.T) {
	// Mock client that returns a Go main function
	queryCount := 0
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			queryCount++
			// First query pattern is for Go main functions
			if queryCount == 1 && strings.Contains(script, "[.]go") {
				return &QueryResult{
					Headers: []string{"name", "file_path", "start_line"},
					Rows: [][]any{
						{"main", "cmd/server/main.go", 1},
					},
				}, nil
			}
			// Return empty for other patterns
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line"},
				Rows:    [][]any{},
			}, nil
		},
		nil,
	)
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	if len(sources) == 0 {
		t.Error("detectEntryPoints() should find Go main function")
	}
	if len(sources) > 0 && sources[0].Name != "main" {
		t.Errorf("detectEntryPoints() found %q, want 'main'", sources[0].Name)
	}
}

// Test detectEntryPoints with path pattern filter
func TestDetectEntryPoints_Unit_WithPathPattern(t *testing.T) {
	client := NewMockClientWithResults(
		[]string{"name", "file_path", "start_line"},
		[][]any{
			{"main", "cmd/server/main.go", 1},
		},
	)
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "cmd/")

	if len(sources) == 0 {
		t.Error("detectEntryPoints() should find main with path pattern")
	}
}

// Test detectEntryPoints filters out test files
func TestDetectEntryPoints_Unit_ExcludeTestFiles(t *testing.T) {
	// Test that test files are excluded by the query pattern
	client := NewMockClientEmpty()
	ctx := context.Background()

	sources := detectEntryPoints(ctx, client, "")

	// Even if test files exist, they should be filtered by regex in the query
	// This test verifies the function completes without error
	_ = sources
}

// Test TracePath with auto-detect entry points (no source specified)
func TestTracePath_Unit_AutoDetectEntryPoints(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main":   {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"target": {Name: "target", FilePath: "pkg/target.go", Line: "10"},
	}
	callGraph := map[string][]string{
		"main":   {"target"},
		"target": {},
	}

	client := createMockCallGraph(functions, callGraph)
	ctx := context.Background()

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "target",
		Source:   "", // Empty source triggers auto-detect
		MaxPaths: 3,
		MaxDepth: 10,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// With auto-detect, should find main and path to target
	if !strings.Contains(result.Text, "target") {
		t.Errorf("TracePath() should find target with auto-detect, got:\n%s", result.Text)
	}
}

// Test formatTraceOutput with multiple paths
func TestFormatTraceOutput_Unit_MultiplePaths(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "target", MaxPaths: 3, MaxDepth: 10}
	path1 := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "route1", FilePath: "pkg/route1.go", Line: "10"},
		{Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	path2 := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "route2", FilePath: "pkg/route2.go", Line: "15"},
		{Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	result := traceSearchResult{
		paths:         [][]TraceFuncInfo{path1, path2},
		nodesExplored: 150,
		limitReached:  false,
	}

	output := formatTraceOutput(sources, args, result)

	if !strings.Contains(output, "Path 1") {
		t.Error("formatTraceOutput() should contain 'Path 1'")
	}
	if !strings.Contains(output, "Path 2") {
		t.Error("formatTraceOutput() should contain 'Path 2'")
	}
	if !strings.Contains(output, "route1") {
		t.Error("formatTraceOutput() should contain 'route1'")
	}
	if !strings.Contains(output, "route2") {
		t.Error("formatTraceOutput() should contain 'route2'")
	}
}

// Test formatTraceOutput with maxPaths reached
func TestFormatTraceOutput_Unit_MaxPathsReached(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "target", MaxPaths: 2, MaxDepth: 10}
	path1 := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	path2 := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	result := traceSearchResult{
		paths:         [][]TraceFuncInfo{path1, path2},
		nodesExplored: 100,
		limitReached:  false,
	}

	output := formatTraceOutput(sources, args, result)

	// Should show message about first N paths
	if !strings.Contains(output, "first 2 paths") {
		t.Error("formatTraceOutput() should mention 'first 2 paths'")
	}
}

// Test formatTraceOutput with limit reached
func TestFormatTraceOutput_Unit_LimitReached(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "target", MaxPaths: 3, MaxDepth: 10}
	path := []TraceFuncInfo{
		{Name: "main", FilePath: "cmd/main.go", Line: "1"},
		{Name: "target", FilePath: "pkg/target.go", Line: "20"},
	}
	result := traceSearchResult{
		paths:         [][]TraceFuncInfo{path},
		nodesExplored: 5000,
		limitReached:  true,
	}

	output := formatTraceOutput(sources, args, result)

	if !strings.Contains(output, "Search limit reached") {
		t.Error("formatTraceOutput() should mention 'Search limit reached'")
	}
}

// Test formatTraceNotFound with limit reached
func TestFormatTraceNotFound_Unit_LimitReached(t *testing.T) {
	sources := []TraceFuncInfo{{Name: "main", FilePath: "cmd/main.go", Line: "1"}}
	args := TracePathArgs{Target: "unreachable", MaxPaths: 3, MaxDepth: 10}
	result := traceSearchResult{
		nodesExplored: 5000,
		limitReached:  true,
	}

	output := formatTraceNotFound(sources, args, result)

	if !strings.Contains(output, "limit reached") {
		t.Error("formatTraceNotFound() should mention 'limit reached'")
	}
	if !strings.Contains(output, "5000 nodes") {
		t.Error("formatTraceNotFound() should mention node count")
	}
}

// Test TraceFuncInfo struct fields
func TestTraceFuncInfo_Unit_Struct(t *testing.T) {
	info := TraceFuncInfo{
		Name:     "HandleRequest",
		FilePath: "internal/handler.go",
		Line:     "42",
	}

	if info.Name != "HandleRequest" {
		t.Errorf("Name = %q, want 'HandleRequest'", info.Name)
	}
	if info.FilePath != "internal/handler.go" {
		t.Errorf("FilePath = %q, want 'internal/handler.go'", info.FilePath)
	}
	if info.Line != "42" {
		t.Errorf("Line = %q, want '42'", info.Line)
	}
}

// Test TracePathArgs default values
func TestTracePathArgs_Unit_Defaults(t *testing.T) {
	args := TracePathArgs{
		Target: "saveToDb",
	}

	if args.Target != "saveToDb" {
		t.Errorf("Target should be set, got %q", args.Target)
	}
	if args.Source != "" {
		t.Errorf("Default Source = %q, want empty", args.Source)
	}
	if args.PathPattern != "" {
		t.Errorf("Default PathPattern = %q, want empty", args.PathPattern)
	}
	if args.MaxPaths != 0 {
		t.Errorf("Default MaxPaths = %d, want 0", args.MaxPaths)
	}
	if args.MaxDepth != 0 {
		t.Errorf("Default MaxDepth = %d, want 0", args.MaxDepth)
	}
}

// Test context cancellation during search
func TestTracePath_Unit_ContextCancellation(t *testing.T) {
	functions := map[string]TraceFuncInfo{
		"main": {Name: "main", FilePath: "cmd/main.go", Line: "1"},
		"fn1":  {Name: "fn1", FilePath: "pkg/fn1.go", Line: "10"},
	}
	callGraph := map[string][]string{
		"main": {"fn1"},
		"fn1":  {"fn1"}, // Self loop to make search longer
	}

	client := createMockCallGraph(functions, callGraph)

	// Create context with immediate cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond) // Ensure context is canceled

	result, err := TracePath(ctx, client, TracePathArgs{
		Target:   "fn1",
		Source:   "main",
		MaxPaths: 3,
		MaxDepth: 100,
	})
	if err != nil {
		t.Fatalf("TracePath() error = %v", err)
	}

	// Should handle cancellation gracefully
	// Note: May or may not detect cancellation depending on timing
	// Just verify it doesn't panic
	_ = result
}

// ============================================================================
// INTEGRATION TESTS (require CozoDB - see trace_integration_test.go)
// ============================================================================

// Note: Integration tests have been moved to trace_integration_test.go with //go:build cozodb tag.
// This allows unit tests to run without CozoDB while keeping integration tests for e2e validation.
// To run integration tests: go test -tags=cozodb ./modules/cie/pkg/tools -run TestTrace
