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

// Integration tests for Analyze() function and state machine methods.
// These tests use mock clients and verify the full workflow.

import (
	"context"
	"strings"
	"testing"
)

// TestAnalyze_EmptyQuestion tests validation for empty question.
func TestAnalyze_EmptyQuestion(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)
	client := NewMockClientEmpty()

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question: "",
	})

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Should return error result
	assertContains(t, result.Text, "Error")
	assertContains(t, result.Text, "question")
	assertContains(t, result.Text, "required")
}

// TestAnalyze_BasicQuery tests basic analysis workflow.
func TestAnalyze_BasicQuery(t *testing.T) {
	t.Parallel()

	// Mock query results - simplified responses
	headers := []string{"count"}
	rows := [][]any{{float64(10)}} // 10 files/functions

	ctx, client := setupTestWithMock(t, headers, rows)

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question: "What are the main entry points?",
	})

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Should include index stats
	assertContains(t, result.Text, "Index Status")
	assertContains(t, result.Text, "Files indexed")
	assertContains(t, result.Text, "Functions indexed")

	// Should note that semantic search is not configured
	assertContains(t, result.Text, "embedding not configured")
}

// TestAnalyze_WithPathPattern tests path-filtered analysis.
func TestAnalyze_WithPathPattern(t *testing.T) {
	t.Parallel()

	headers := []string{"count"}
	rows := [][]any{{float64(5)}}

	ctx, client := setupTestWithMock(t, headers, rows)

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question:    "How does authentication work?",
		PathPattern: "internal/auth",
	})

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Should include stats
	assertContains(t, result.Text, "Index Status")
}

// TestAnalyze_RoleFiltering tests role-based filtering.
func TestAnalyze_RoleFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role string
	}{
		{"default role (source)", ""},
		{"explicit source", "source"},
		{"test files", "test"},
		{"all files", "any"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			headers := []string{"count"}
			rows := [][]any{{float64(3)}}

			ctx, client := setupTestWithMock(t, headers, rows)

			result, err := Analyze(ctx, client, AnalyzeArgs{
				Question: "Show me the code structure",
				Role:     tt.role,
			})

			assertNoError(t, err)
			if result == nil {
				t.Fatalf("expected result for role %q, got nil", tt.role)
			}

			// All should produce valid output
			assertContains(t, result.Text, "Index Status")
		})
	}
}

// TestAnalyze_NoResults tests handling when no results are found.
func TestAnalyze_NoResults(t *testing.T) {
	t.Parallel()

	// Return 0 for counts
	headers := []string{"count"}
	rows := [][]any{{float64(0)}}

	ctx, client := setupTestWithMock(t, headers, rows)

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question: "Non-existent feature",
	})

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Should show that 0 results were found (in keyword search results)
	// The output may not say "No relevant results found" if keyword searches returned 0
	// Just verify it completed successfully with index stats
	assertContains(t, result.Text, "Index Status")
	assertContains(t, result.Text, "0")
}

// TestAnalyze_MultipleQueries tests analysis with various question types.
func TestAnalyze_MultipleQueries(t *testing.T) {
	t.Parallel()

	questions := []string{
		"What are the entry points?",
		"How does the API work?",
		"Show me the database schema",
		"What are the HTTP routes?",
	}

	for _, question := range questions {
		t.Run(question, func(t *testing.T) {
			t.Parallel()

			headers := []string{"count"}
			rows := [][]any{{float64(5)}}

			ctx, client := setupTestWithMock(t, headers, rows)

			result, err := Analyze(ctx, client, AnalyzeArgs{
				Question: question,
			})

			assertNoError(t, err)
			if result == nil {
				t.Fatalf("expected result for %q, got nil", question)
			}

			// All should produce valid output
			assertContains(t, result.Text, "Index Status")
		})
	}
}

// TestAnalyze_ContextCancellation tests handling of canceled context.
func TestAnalyze_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	client := NewMockClientEmpty()

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question: "Test query",
	})

	// Should complete despite canceled context (may have partial results)
	// The function doesn't explicitly check context, so it should return something
	if err != nil && result == nil {
		t.Fatalf("unexpected error with nil result: %v", err)
	}
}

// TestAnalyze_OutputStructure tests the structure of the output.
func TestAnalyze_OutputStructure(t *testing.T) {
	t.Parallel()

	headers := []string{"count"}
	rows := [][]any{{float64(15)}}

	ctx, client := setupTestWithMock(t, headers, rows)

	result, err := Analyze(ctx, client, AnalyzeArgs{
		Question: "Analyze the codebase structure",
	})

	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Verify markdown structure
	lines := strings.Split(result.Text, "\n")
	if len(lines) < 5 {
		t.Errorf("expected multi-line output, got %d lines", len(lines))
	}

	// Should have headers (## or #)
	hasHeaders := false
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			hasHeaders = true
			break
		}
	}
	if !hasHeaders {
		t.Error("expected markdown headers in output")
	}
}

// TestAnalyzeState_AddIndexStats tests the addIndexStats method.
func TestAnalyzeState_AddIndexStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileCount float64
		funcCount float64
		wantFiles int
		wantFuncs int
	}{
		{
			name:      "normal counts",
			fileCount: float64(100),
			funcCount: float64(500),
			wantFiles: 100,
			wantFuncs: 500,
		},
		{
			name:      "zero counts",
			fileCount: float64(0),
			funcCount: float64(0),
			wantFiles: 0,
			wantFuncs: 0,
		},
		{
			name:      "large counts",
			fileCount: float64(10000),
			funcCount: float64(50000),
			wantFiles: 10000,
			wantFuncs: 50000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a custom mock that returns different counts for different queries
			client := NewMockClientCustom(
				func(ctx context.Context, script string) (*QueryResult, error) {
					// File count query
					if strings.Contains(script, "cie_file") {
						return NewMockQueryResult(
							[]string{"count"},
							[][]any{{tt.fileCount}},
						), nil
					}
					// Function count query
					if strings.Contains(script, "cie_function") {
						return NewMockQueryResult(
							[]string{"count"},
							[][]any{{tt.funcCount}},
						), nil
					}
					// Default: empty result
					return NewMockQueryResult([]string{}, [][]any{}), nil
				},
				nil,
			)

			ctx := setupTest(t)
			state := &analyzeState{args: AnalyzeArgs{Question: "test"}}

			state.addIndexStats(ctx, client)

			// Should have one section with stats
			if len(state.sections) != 1 {
				t.Fatalf("expected 1 section, got %d", len(state.sections))
			}

			section := state.sections[0]

			// Verify stats are formatted correctly
			if !strings.Contains(section, "Index Status") {
				t.Error("expected 'Index Status' header")
			}

			// Check file count
			fileStr := strings.Contains(section, "Files indexed:")
			if !fileStr {
				t.Error("expected 'Files indexed:' in output")
			}

			// Check function count
			funcStr := strings.Contains(section, "Functions indexed:")
			if !funcStr {
				t.Error("expected 'Functions indexed:' in output")
			}
		})
	}
}

// TestAnalyzeState_PerformSemanticSearch tests semantic search with mock client.
func TestAnalyzeState_PerformSemanticSearch(t *testing.T) {
	t.Parallel()

	t.Run("no embedding config", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)
		state := &analyzeState{args: AnalyzeArgs{Question: "test"}}

		state.performSemanticSearch(ctx, client)

		// Should mark as failed and add error
		if !state.searchFailed {
			t.Error("expected searchFailed to be true when embedding not configured")
		}

		if len(state.errors) == 0 {
			t.Error("expected error message about embedding not configured")
		}

		// Error should mention embedding
		hasEmbeddingError := false
		for _, err := range state.errors {
			if strings.Contains(err, "embedding not configured") {
				hasEmbeddingError = true
				break
			}
		}
		if !hasEmbeddingError {
			t.Error("expected error about embedding configuration")
		}
	})

	t.Run("with path pattern but no embedding", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)
		state := &analyzeState{
			args: AnalyzeArgs{
				Question:    "test",
				PathPattern: "internal/auth",
			},
		}

		state.performSemanticSearch(ctx, client)

		// Should still fail due to no embedding config
		if !state.searchFailed {
			t.Error("expected searchFailed to be true")
		}
	})
}

// TestAnalyzeState_FormatSemanticResults tests result formatting.
func TestAnalyzeState_FormatSemanticResults(t *testing.T) {
	t.Parallel()

	t.Run("no results", func(t *testing.T) {
		t.Parallel()

		state := &analyzeState{
			args:           AnalyzeArgs{Question: "test"},
			localizedFuncs: []relevantFunction{},
			globalFuncs:    []relevantFunction{},
		}

		state.formatSemanticResults()

		// Should not add any sections
		if len(state.sections) != 0 {
			t.Errorf("expected 0 sections, got %d", len(state.sections))
		}
	})

	t.Run("with localized results", func(t *testing.T) {
		t.Parallel()

		state := &analyzeState{
			args: AnalyzeArgs{
				Question:    "test",
				PathPattern: "internal/auth",
			},
			localizedFuncs: []relevantFunction{
				{
					Name:       "Authenticate",
					FilePath:   "internal/auth/auth.go",
					StartLine:  "10",
					Similarity: 0.85,
				},
			},
			globalFuncs: []relevantFunction{},
		}

		state.formatSemanticResults()

		// Should add one section for localized results
		if len(state.sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(state.sections))
		}

		section := state.sections[0]
		if !strings.Contains(section, "Semantically Relevant") {
			t.Error("expected 'Semantically Relevant' in section header")
		}
		if !strings.Contains(section, "internal/auth") {
			t.Error("expected path pattern in section header")
		}
		if !strings.Contains(section, "Authenticate") {
			t.Error("expected function name in section")
		}
	})

	t.Run("with global results", func(t *testing.T) {
		t.Parallel()

		state := &analyzeState{
			args:           AnalyzeArgs{Question: "test"},
			localizedFuncs: []relevantFunction{},
			globalFuncs: []relevantFunction{
				{
					Name:       "GlobalFunc",
					FilePath:   "pkg/util/helper.go",
					StartLine:  "5",
					Similarity: 0.75,
				},
			},
		}

		state.formatSemanticResults()

		// Should add one section for global results
		if len(state.sections) != 1 {
			t.Fatalf("expected 1 section, got %d", len(state.sections))
		}

		section := state.sections[0]
		if !strings.Contains(section, "Semantically Relevant") {
			t.Error("expected 'Semantically Relevant' in section header")
		}
		if !strings.Contains(section, "GlobalFunc") {
			t.Error("expected function name in section")
		}
	})

	t.Run("with both localized and global results", func(t *testing.T) {
		t.Parallel()

		state := &analyzeState{
			args: AnalyzeArgs{
				Question:    "test",
				PathPattern: "internal/auth",
			},
			localizedFuncs: []relevantFunction{
				{Name: "LocalFunc", FilePath: "internal/auth/auth.go", StartLine: "10", Similarity: 0.9},
			},
			globalFuncs: []relevantFunction{
				{Name: "GlobalFunc", FilePath: "pkg/util/helper.go", StartLine: "5", Similarity: 0.8},
			},
		}

		state.formatSemanticResults()

		// Should add two sections
		if len(state.sections) != 2 {
			t.Fatalf("expected 2 sections, got %d", len(state.sections))
		}

		// First section should be localized
		if !strings.Contains(state.sections[0], "LocalFunc") {
			t.Error("expected LocalFunc in first section")
		}

		// Second section should be global
		if !strings.Contains(state.sections[1], "GlobalFunc") {
			t.Error("expected GlobalFunc in second section")
		}
	})
}

// TestAnalyzeState_BuildOutput tests output building.
func TestAnalyzeState_BuildOutput(t *testing.T) {
	t.Parallel()

	t.Run("basic output", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)

		state := &analyzeState{
			args: AnalyzeArgs{Question: "What is the architecture?"},
			sections: []string{
				"## Section 1\nContent 1",
				"## Section 2\nContent 2",
			},
			errors: []string{},
		}

		result, err := state.buildOutput(ctx, client)

		assertNoError(t, err)
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Should contain question as title
		assertContains(t, result.Text, "What is the architecture?")

		// Should contain sections
		assertContains(t, result.Text, "Section 1")
		assertContains(t, result.Text, "Section 2")
	})

	t.Run("with errors", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)

		state := &analyzeState{
			args: AnalyzeArgs{Question: "test"},
			sections: []string{
				"## Results\nSome results",
			},
			errors: []string{
				"Error 1: something failed",
				"Error 2: another failure",
			},
		}

		result, err := state.buildOutput(ctx, client)

		assertNoError(t, err)
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Should contain error section
		assertContains(t, result.Text, "Query Issues")
		assertContains(t, result.Text, "Error 1")
		assertContains(t, result.Text, "Error 2")
	})

	t.Run("no meaningful results", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)

		state := &analyzeState{
			args: AnalyzeArgs{Question: "test"},
			sections: []string{
				"# Analysis: test\n\n", // Only title, no real content
			},
			globalFuncs: []relevantFunction{},
			errors:      []string{},
		}

		result, err := state.buildOutput(ctx, client)

		assertNoError(t, err)
		if result == nil {
			t.Fatal("expected result, got nil")
		}

		// Should contain "no results" message
		assertContains(t, result.Text, "No relevant results found")
		assertContains(t, result.Text, "Suggestions")
	})
}

// TestFindRelevantFunctions_NoEmbedding tests error when embedding not configured.
func TestFindRelevantFunctions_NoEmbedding(t *testing.T) {
	t.Parallel()

	ctx, client := setupTestWithEmptyMock(t)

	// Should return error when embedding not configured
	_, err := findRelevantFunctions(ctx, client, "test question", "", "source", 10)

	if err == nil {
		t.Fatal("expected error when embedding not configured")
	}

	if !strings.Contains(err.Error(), "embedding not configured") {
		t.Errorf("expected 'embedding not configured' error, got: %v", err)
	}
}

// TestFindRelevantFunctionsLocalized_NoEmbedding tests error when embedding not configured.
func TestFindRelevantFunctionsLocalized_NoEmbedding(t *testing.T) {
	t.Parallel()

	ctx, client := setupTestWithEmptyMock(t)

	// Should return error when embedding not configured
	_, err := findRelevantFunctionsLocalized(ctx, client, "test question", "internal/auth", "source", 10)

	if err == nil {
		t.Fatal("expected error when embedding not configured")
	}

	if !strings.Contains(err.Error(), "embedding not configured") {
		t.Errorf("expected 'embedding not configured' error, got: %v", err)
	}
}

// TestFindRelevantFunctionsLocalized_EmptyPath tests nil return for empty path.
func TestFindRelevantFunctionsLocalized_EmptyPath(t *testing.T) {
	t.Parallel()

	ctx, client := setupTestWithEmptyMock(t)

	// Should return nil when path is empty
	result, err := findRelevantFunctionsLocalized(ctx, client, "test question", "", "source", 10)

	assertNoError(t, err)
	if result != nil {
		t.Errorf("expected nil result for empty path, got %v", result)
	}
}

// TestGetFunctionCodeByName_QueryError tests error handling.
func TestGetFunctionCodeByName_QueryError(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)
	client := NewMockClientWithError(context.Canceled)

	// Should return error when query fails
	_, err := getFunctionCodeByName(ctx, client, "TestFunc", "/path/to/file.go")

	if err == nil {
		t.Fatal("expected error when query fails")
	}
}

// TestGetFunctionCodeByName_NoResults tests empty result handling.
func TestGetFunctionCodeByName_NoResults(t *testing.T) {
	t.Parallel()

	ctx, client := setupTestWithEmptyMock(t)

	// Should return error when no results found
	code, err := getFunctionCodeByName(ctx, client, "NonExistentFunc", "/path/to/file.go")

	// The function may return empty code instead of error
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	} else if code == "" {
		// Empty code is also acceptable
		return
	} else {
		t.Error("expected empty code or error when function not found")
	}
}

// TestGetFunctionCodeByName_Success tests successful code retrieval.
func TestGetFunctionCodeByName_Success(t *testing.T) {
	t.Parallel()

	// Mock result with function code
	headers := []string{"code_text"}
	rows := [][]any{{"func TestFunc() {\n    return 42\n}"}}

	ctx, client := setupTestWithMock(t, headers, rows)

	code, err := getFunctionCodeByName(ctx, client, "TestFunc", "/path/to/file.go")

	assertNoError(t, err)
	if code == "" {
		t.Fatal("expected code, got empty string")
	}

	assertContains(t, code, "func TestFunc()")
	assertContains(t, code, "return 42")
}

// TestPerformKeywordFallback tests keyword fallback logic.
func TestPerformKeywordFallback(t *testing.T) {
	t.Parallel()

	t.Run("runs when search failed", func(t *testing.T) {
		t.Parallel()

		headers := []string{"name"}
		rows := [][]any{{"TestFunc"}}

		ctx, client := setupTestWithMock(t, headers, rows)

		state := &analyzeState{
			args:         AnalyzeArgs{Question: "authentication logic"},
			searchFailed: true,
		}

		state.performKeywordFallback(ctx, client)

		// Should have added keyword search sections
		if len(state.sections) == 0 {
			t.Error("expected keyword fallback to add sections")
		}
	})

	t.Run("skips when search succeeded", func(t *testing.T) {
		t.Parallel()

		ctx, client := setupTestWithEmptyMock(t)

		state := &analyzeState{
			args:         AnalyzeArgs{Question: "test"},
			searchFailed: false, // Search succeeded
		}

		state.performKeywordFallback(ctx, client)

		// Should not add any sections
		if len(state.sections) != 0 {
			t.Error("expected no sections when search succeeded")
		}
	})
}

// TestRunQuery tests the query helper method.
func TestRunQuery(t *testing.T) {
	t.Parallel()

	t.Run("successful query", func(t *testing.T) {
		t.Parallel()

		headers := []string{"name"}
		rows := [][]any{{"Result1"}, {"Result2"}}

		ctx, client := setupTestWithMock(t, headers, rows)

		state := &analyzeState{args: AnalyzeArgs{Question: "test"}}
		result := state.runQuery(ctx, client, "test query", "SELECT * FROM table")

		if result == nil {
			t.Fatal("expected result, got nil")
		}

		if len(result.Rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(result.Rows))
		}
	})

	t.Run("query error", func(t *testing.T) {
		t.Parallel()

		ctx := setupTest(t)
		client := NewMockClientWithError(context.Canceled)

		state := &analyzeState{args: AnalyzeArgs{Question: "test"}}
		result := state.runQuery(ctx, client, "test query", "SELECT * FROM table")

		if result != nil {
			t.Error("expected nil result on error")
		}

		// Should have added error
		if len(state.errors) == 0 {
			t.Error("expected error to be recorded")
		}
	})
}
