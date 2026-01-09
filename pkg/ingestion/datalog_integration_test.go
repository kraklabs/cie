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
package ingestion

import (
	"strings"
	"testing"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// TestBuildDeletions_CozoDBIntegration tests that the generated :rm statements
// work with a real CozoDB instance (in-memory mode).
func TestBuildDeletions_CozoDBIntegration(t *testing.T) {
	// Create in-memory CozoDB
	dir := t.TempDir()
	db, err := cozo.New("mem", dir, nil)
	if err != nil {
		t.Fatalf("create cozodb: %v", err)
	}
	defer db.Close()

	// Create test tables matching real schema (with 'id' as primary key for edge tables)
	createTables := []string{
		`:create cie_file { id: String => path: String, language: String, role: String }`,
		`:create cie_function { id: String => name: String, signature: String, file_path: String, start_line: Int, end_line: Int, start_col: Int, end_col: Int }`,
		`:create cie_function_code { function_id: String => code_text: String }`,
		`:create cie_function_embedding { function_id: String => embedding: <F32; 1536> }`,
		`:create cie_calls { id: String => caller_id: String, callee_id: String }`,
		`:create cie_defines { id: String => file_id: String, function_id: String }`,
	}

	for _, stmt := range createTables {
		if _, err := db.Run(stmt, nil); err != nil {
			t.Fatalf("create table: %v\nStatement: %s", err, stmt)
		}
	}

	// Insert test data (using 'id' as primary key for edge tables)
	insertData := []string{
		`?[id, path, language, role] <- [["file:1", "test.go", "go", "source"]] :put cie_file {id => path, language, role}`,
		`?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [["func:1", "TestFunc", "func()", "test.go", 1, 10, 0, 0]] :put cie_function {id => name, signature, file_path, start_line, end_line, start_col, end_col}`,
		`?[function_id, code_text] <- [["func:1", "func TestFunc() {}"]] :put cie_function_code {function_id => code_text}`,
		`?[id, caller_id, callee_id] <- [["call:func:1|func:external", "func:1", "func:external"]] :put cie_calls {id => caller_id, callee_id}`,
		`?[id, file_id, function_id] <- [["def:file:1|func:1", "file:1", "func:1"]] :put cie_defines {id => file_id, function_id}`,
	}

	for _, stmt := range insertData {
		if _, err := db.Run(stmt, nil); err != nil {
			t.Fatalf("insert data: %v\nStatement: %s", err, stmt)
		}
	}

	// Verify data exists
	result, err := db.Run(`?[id] := *cie_function{id}`, nil)
	if err != nil {
		t.Fatalf("query functions: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Rows))
	}

	// Build deletion script using our BuildDeletions function
	// Use CallsEdgeIDs and DefinesEdgeIDs (primary key 'id') instead of deprecated composite keys
	builder := &DatalogBuilder{}
	deletions := DeletionSet{
		FileIDs:        []string{"file:1"},
		FunctionIDs:    []string{"func:1"},
		CallsEdgeIDs:   []string{"call:func:1|func:external"},
		DefinesEdgeIDs: []string{"def:file:1|func:1"},
	}

	script := builder.BuildDeletions(deletions)
	t.Logf("Generated deletion script:\n%s", script)

	// Execute deletion script - each statement is wrapped in {}
	// We need to execute them as chained queries
	statements := strings.Split(strings.TrimSpace(script), "\n")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "//") {
			continue
		}
		if _, err := db.Run(stmt, nil); err != nil {
			t.Fatalf("execute deletion: %v\nStatement: %s", err, stmt)
		}
	}

	// Verify data was deleted
	result, err = db.Run(`?[id] := *cie_function{id}`, nil)
	if err != nil {
		t.Fatalf("query functions after delete: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 functions after delete, got %d", len(result.Rows))
	}

	result, err = db.Run(`?[id] := *cie_calls{id}`, nil)
	if err != nil {
		t.Fatalf("query calls after delete: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 calls edges after delete, got %d", len(result.Rows))
	}

	result, err = db.Run(`?[id] := *cie_defines{id}`, nil)
	if err != nil {
		t.Fatalf("query defines after delete: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 defines edges after delete, got %d", len(result.Rows))
	}

	result, err = db.Run(`?[id] := *cie_file{id}`, nil)
	if err != nil {
		t.Fatalf("query files after delete: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 files after delete, got %d", len(result.Rows))
	}
}

// TestRmSyntax_CozoDB tests the basic :rm syntax directly with CozoDB.
func TestRmSyntax_CozoDB(t *testing.T) {
	dir := t.TempDir()
	db, err := cozo.New("mem", dir, nil)
	if err != nil {
		t.Fatalf("create cozodb: %v", err)
	}
	defer db.Close()

	// Create a simple table
	if _, err := db.Run(`:create test_table { id: String => value: String }`, nil); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert data
	if _, err := db.Run(`?[id, value] <- [["key1", "val1"]] :put test_table {id => value}`, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Verify data exists
	result, err := db.Run(`?[id] := *test_table{id}`, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}

	// Test the correct :rm syntax
	rmScript := `{ ?[id] <- [["key1"]] :rm test_table {id} }`
	if _, err := db.Run(rmScript, nil); err != nil {
		t.Fatalf("rm failed: %v\nScript: %s", err, rmScript)
	}

	// Verify deletion
	result, err = db.Run(`?[id] := *test_table{id}`, nil)
	if err != nil {
		t.Fatalf("query after delete: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows after delete, got %d", len(result.Rows))
	}
}

// TestRmSyntax_CompositeKey_CozoDB tests :rm with composite keys.
func TestRmSyntax_CompositeKey_CozoDB(t *testing.T) {
	dir := t.TempDir()
	db, err := cozo.New("mem", dir, nil)
	if err != nil {
		t.Fatalf("create cozodb: %v", err)
	}
	defer db.Close()

	// Create a table with composite key (like cie_calls)
	if _, err := db.Run(`:create edge_table { from_id: String, to_id: String => }`, nil); err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert data
	if _, err := db.Run(`?[from_id, to_id] <- [["a", "b"], ["c", "d"]] :put edge_table {from_id, to_id}`, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Verify data exists
	result, err := db.Run(`?[from_id] := *edge_table{from_id, to_id}`, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(result.Rows))
	}

	// Test :rm with composite key
	rmScript := `{ ?[from_id, to_id] <- [["a", "b"]] :rm edge_table {from_id, to_id} }`
	if _, err := db.Run(rmScript, nil); err != nil {
		t.Fatalf("rm failed: %v\nScript: %s", err, rmScript)
	}

	// Verify only one row remains
	result, err = db.Run(`?[from_id, to_id] := *edge_table{from_id, to_id}`, nil)
	if err != nil {
		t.Fatalf("query after delete: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row after delete, got %d", len(result.Rows))
	}
}
