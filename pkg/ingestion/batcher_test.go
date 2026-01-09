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

package ingestion

import (
	"fmt"
	"strings"
	"testing"
)

func TestBatcher_SplitStatements_MultiLine(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test multi-line statement with nested brackets
	script := `:replace cie_function[?id] <- [[?id = "func1", ?name = "test", ?embedding = [0.1, 0.2, 0.3]]]
:replace cie_file[?id] <- [[?id = "file1", ?path = "test.go"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify first statement contains the multi-line function
	if !strings.Contains(statements[0], "cie_function") {
		t.Error("first statement should contain cie_function")
	}
	if !strings.Contains(statements[0], "embedding = [0.1, 0.2, 0.3]") {
		t.Error("first statement should contain embedding array")
	}

	// Verify second statement
	if !strings.Contains(statements[1], "cie_file") {
		t.Error("second statement should contain cie_file")
	}
}

func TestBatcher_SplitStatements_ComplexNested(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test complex nested structures
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() {\n    return 1\n}", ?embedding = [0.1, 0.2]]]
:replace cie_defines[?file_id, ?function_id] <- [[?file_id = "file1", ?function_id = "func1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify nested structures are preserved
	if !strings.Contains(statements[0], "code_text") {
		t.Error("first statement should contain code_text")
	}
	if !strings.Contains(statements[0], "return 1") {
		t.Error("first statement should preserve nested content")
	}
}

func TestBatcher_SplitStatements_EmptyLines(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test with empty lines between statements
	script := `:replace cie_file[?id] <- [[?id = "file1"]]

:replace cie_file[?id] <- [[?id = "file2"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}
}

func TestBatcher_Batch_MultiLineStatements(t *testing.T) {
	batcher := NewBatcher(2, 10000) // Target 2 mutations per batch

	// Create script with 5 mutations
	script := ""
	for i := 0; i < 5; i++ {
		script += fmt.Sprintf(`:replace cie_file[?id] <- [[?id = "file%d", ?path = "test%d.go", ?hash = "hash%d", ?language = "go", ?size = %d]]
`, i, i, i, 100+i)
	}

	batches, err := batcher.Batch(script)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	// Should have 3 batches (2, 2, 1)
	if len(batches) < 2 || len(batches) > 3 {
		t.Errorf("expected 2-3 batches, got %d", len(batches))
	}

	// Verify each batch is valid
	for i, batch := range batches {
		statements := batcher.splitStatements(batch)
		if len(statements) == 0 {
			t.Errorf("batch %d is empty", i)
		}
	}
}

func TestBatcher_Batch_ExceedsMaxSize(t *testing.T) {
	batcher := NewBatcher(1000, 100) // Very small max size (100 bytes)

	// Create a single statement that exceeds max size
	largeCodeText := strings.Repeat("x", 200) // 200 bytes
	script := fmt.Sprintf(`:replace cie_function[?id] <- [[?id = "func1", ?code_text = "%s"]]`, largeCodeText)

	_, err := batcher.Batch(script)
	if err == nil {
		t.Error("expected error when statement exceeds max size")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Errorf("expected error about max size, got: %v", err)
	}
}

func TestBatcher_SplitStatements_WithStringLiterals(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test statements with string literals containing brackets
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() { return [1, 2, 3] }", ?embedding = [0.1, 0.2]]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify brackets inside string literals don't break parsing
	if !strings.Contains(statements[0], "return [1, 2, 3]") {
		t.Error("first statement should preserve brackets inside string literals")
	}
}

func TestBatcher_SplitStatements_WithEscapedQuotes(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test statements with escaped quotes in strings
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() { return \"hello\" }", ?embedding = [0.1]]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify escaped quotes are handled correctly
	if !strings.Contains(statements[0], "return \\\"hello\\\"") {
		t.Error("first statement should preserve escaped quotes")
	}
}

func TestBatcher_SplitStatements_ComplexMultiLine(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test very complex multi-line statement
	script := `:replace cie_function[?id] <- [[
	?id = "func1",
	?name = "test",
	?code_text = "func test() {
		if x > 0 {
			return [1, 2, 3]
		}
		return []
	}",
	?embedding = [0.1, 0.2, 0.3, 0.4, 0.5]
]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify complex nested structure is preserved
	if !strings.Contains(statements[0], "if x > 0") {
		t.Error("first statement should preserve complex nested content")
	}
	if !strings.Contains(statements[0], "return [1, 2, 3]") {
		t.Error("first statement should preserve brackets in code text")
	}
}

func TestBatcher_SplitStatements_WithComments(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test CozoDB batch statements with comments between them
	script := `{ ?[id, path] <- [["file1", "test.go"]] :put cie_file { id, path } }
// This is a comment between statements
{ ?[id, name] <- [["func1", "test"]] :put cie_function { id, name } }
{ ?[id] <- [["file2"]] :put cie_file { id } }`

	statements := batcher.splitStatements(script)
	if len(statements) != 3 {
		t.Errorf("expected 3 statements, got %d", len(statements))
	}

	// Verify statements contain expected content
	if !strings.Contains(statements[0], "file1") {
		t.Error("first statement should contain file1")
	}
	if !strings.Contains(statements[1], "func1") {
		t.Error("second statement should contain func1")
	}
	if !strings.Contains(statements[2], "file2") {
		t.Error("third statement should contain file2")
	}
}

func TestBatcher_SplitStatements_WithCommentsInStrings(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test that // inside strings is not treated as a comment
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() { // not a comment }", ?embedding = [0.1]]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify // inside string is preserved
	if !strings.Contains(statements[0], "// not a comment") {
		t.Error("first statement should preserve // inside string literal")
	}
}

func TestBatcher_Batch_RespectsMaxSize(t *testing.T) {
	batcher := NewBatcher(1000, 500) // Small max size (500 bytes)

	// Create statements that will exceed max size when batched
	script := ""
	for i := 0; i < 10; i++ {
		// Each statement is ~100 bytes
		script += fmt.Sprintf(`:replace cie_file[?id] <- [[?id = "file%d", ?path = "test%d.go", ?hash = "hash%d", ?language = "go", ?size = %d]]
`, i, i, i, 100+i)
	}

	batches, err := batcher.Batch(script)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	// Verify each batch is under max size
	for i, batch := range batches {
		batchSize := len(batch)
		if batchSize > 500 {
			t.Errorf("batch %d exceeds max size: %d bytes (limit: 500)", i, batchSize)
		}
		// Verify batch is not empty
		if batchSize == 0 {
			t.Errorf("batch %d is empty", i)
		}
	}
}

func TestBatcher_Batch_TargetMutations(t *testing.T) {
	batcher := NewBatcher(3, 10000) // Target 3 mutations per batch

	// Create 10 mutations
	script := ""
	for i := 0; i < 10; i++ {
		script += fmt.Sprintf(`:replace cie_file[?id] <- [[?id = "file%d", ?path = "test%d.go", ?hash = "hash%d", ?language = "go", ?size = %d]]
`, i, i, i, 100+i)
	}

	batches, err := batcher.Batch(script)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	// Should have approximately 4 batches (3, 3, 3, 1)
	if len(batches) < 3 || len(batches) > 4 {
		t.Errorf("expected 3-4 batches for 10 mutations with target 3, got %d", len(batches))
	}

	// Verify each batch (except last) has target mutations
	for i := 0; i < len(batches)-1; i++ {
		statements := batcher.splitStatements(batches[i])
		if len(statements) != 3 {
			t.Errorf("batch %d should have 3 mutations, got %d", i, len(statements))
		}
	}
}

func TestBatcher_SplitStatements_MultiLineWithComments(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test multi-line statement with comments
	script := `:replace cie_function[?id] <- [[
	?id = "func1",
	?name = "test",
	// Comment in the middle
	?code_text = "func test() {}",
	?embedding = [0.1, 0.2]
]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify first statement contains the function data
	if !strings.Contains(statements[0], "func1") {
		t.Error("first statement should contain func1")
	}
	if !strings.Contains(statements[0], "code_text") {
		t.Error("first statement should contain code_text")
	}
	// Comment should be removed
	if strings.Contains(statements[0], "Comment in the middle") {
		t.Error("first statement should not contain comment")
	}
}

func TestBatcher_SplitStatements_ComplexNestedBracketsInStrings(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test statements with complex nested brackets inside strings
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() { return map[string]int{\"a\": 1} }", ?embedding = [0.1, 0.2, 0.3]]]
:replace cie_file[?id] <- [[?id = "file1", ?path = "test.go"]]`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify brackets inside string are preserved
	if !strings.Contains(statements[0], "map[string]int") {
		t.Error("first statement should preserve brackets inside string")
	}
}

func TestBatcher_SplitStatements_CommentsInMultiLineStrings(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test CozoDB batch syntax with strings containing //
	script := `{ ?[id, code_text] <- [["func1", "func test() { // comment in code }"]] :put cie_function { id, code_text } }
{ ?[id] <- [["file1"]] :put cie_file { id } }`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements, got %d", len(statements))
	}

	// Verify // inside string is preserved
	if !strings.Contains(statements[0], "// comment in code") {
		t.Error("first statement should preserve // inside string literal")
	}
}

func TestBatcher_Batch_ExceedsMaxSize_Rejects(t *testing.T) {
	batcher := NewBatcher(1000, 100) // Very small max size (100 bytes)

	// Create a single statement that exceeds max size
	largeCodeText := strings.Repeat("x", 200) // 200 bytes
	script := fmt.Sprintf(`:replace cie_function[?id] <- [[?id = "func1", ?code_text = "%s", ?embedding = [0.1]]]`, largeCodeText)

	_, err := batcher.Batch(script)
	if err == nil {
		t.Error("expected error when statement exceeds max size")
	}
	if !strings.Contains(err.Error(), "exceeds max size") {
		t.Errorf("expected error about max size, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Statement preview") {
		t.Error("error should include statement preview for debugging")
	}
}

func TestBatcher_Batch_RespectsBothLimits(t *testing.T) {
	batcher := NewBatcher(2, 200) // Target 2 mutations, max 200 bytes

	// Create statements that will hit both limits
	script := ""
	for i := 0; i < 5; i++ {
		// Each statement is ~80 bytes
		script += fmt.Sprintf(`:replace cie_file[?id] <- [[?id = "file%d", ?path = "test%d.go", ?hash = "hash%d", ?language = "go", ?size = %d]]
`, i, i, i, 100+i)
	}

	batches, err := batcher.Batch(script)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}

	// Verify batches respect both limits
	for i, batch := range batches {
		batchSize := len(batch)
		statements := batcher.splitStatements(batch)
		mutationCount := len(statements)

		if batchSize > 200 {
			t.Errorf("batch %d exceeds max size: %d bytes (limit: 200)", i, batchSize)
		}
		if mutationCount > 2 && i < len(batches)-1 {
			// Last batch can have fewer, but others should respect target
			t.Errorf("batch %d exceeds target mutations: %d (target: 2)", i, mutationCount)
		}
	}
}

func TestBatcher_SplitStatements_EmptyScript(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	statements := batcher.splitStatements("")
	if len(statements) != 0 {
		t.Errorf("expected 0 statements for empty script, got %d", len(statements))
	}
}

func TestBatcher_SplitStatements_OnlyComments(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	script := `// This is a comment
// Another comment
// Yet another comment`

	statements := batcher.splitStatements(script)
	if len(statements) != 0 {
		t.Errorf("expected 0 statements for script with only comments, got %d", len(statements))
	}
}

func TestBatcher_SplitStatements_UnbalancedBrackets_HandlesGracefully(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test with potentially unbalanced brackets (should still try to parse)
	script := `:replace cie_function[?id] <- [[?id = "func1", ?code_text = "func test() {", ?embedding = [0.1]]]
:replace cie_file[?id] <- [[?id = "file1"]]`

	statements := batcher.splitStatements(script)
	// Should still attempt to split, even if brackets might be unbalanced
	if len(statements) == 0 {
		t.Error("should attempt to split even with potentially unbalanced brackets")
	}
}

func TestBatcher_SplitStatements_UnicodeCharacters(t *testing.T) {
	batcher := NewBatcher(10, 10000)

	// Test with Unicode characters that would produce false quote matches if truncated to byte
	// Ч (U+0427) truncates to 0x27 which is single quote
	// This was a real bug that caused all statements to be grouped into one
	script := `{ ?[id, code] <- [['f1', 'code with Ч Cyrillic']] :put cie_function { id, code } }
{ ?[id, code] <- [['f2', 'more code']] :put cie_function { id, code } }`

	statements := batcher.splitStatements(script)
	if len(statements) != 2 {
		t.Errorf("expected 2 statements with Unicode, got %d", len(statements))
		for i, s := range statements {
			preview := s
			if len(preview) > 100 {
				preview = preview[:100]
			}
			t.Logf("Statement %d (len=%d): %s", i+1, len(s), preview)
		}
	}

	// Also test with other problematic Unicode chars
	script2 := `{ ?[id, code] <- [['f1', 'math: ∧ ∨']] :put cie_function { id, code } }
{ ?[id, code] <- [['f2', 'arabic: ا ب']] :put cie_function { id, code } }`

	statements2 := batcher.splitStatements(script2)
	if len(statements2) != 2 {
		t.Errorf("expected 2 statements with math/arabic Unicode, got %d", len(statements2))
	}
}
