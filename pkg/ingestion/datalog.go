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
	"fmt"
	"math"
	"strconv"
	"strings"
)

// DatalogBuilder generates Datalog mutation scripts from entities.
// The generated mutations must match the schema defined in schema.go (v3):
//   - cie_file: id, path, hash, language, size
//   - cie_function: id, name, signature, file_path, start_line, end_line, start_col, end_col
//   - cie_function_code: function_id, code_text
//   - cie_function_embedding: function_id, embedding
//   - cie_type: id, name, kind, file_path, start_line, end_line, start_col, end_col
//   - cie_type_code: type_id, code_text
//   - cie_type_embedding: type_id, embedding
//   - cie_defines: file_id, function_id
//   - cie_calls: caller_id, callee_id
type DatalogBuilder struct {
}

// NewDatalogBuilder creates a new Datalog builder.
func NewDatalogBuilder() *DatalogBuilder {
	return &DatalogBuilder{}
}

// ValidationError represents a validation error with details.
type ValidationError struct {
	EntityType string
	EntityID   string
	Field      string
	Message    string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s:%s] field %s: %s", e.EntityType, e.EntityID, e.Field, e.Message)
}

// ValidateEntities validates that all entities have required fields and correct types.
// Returns an error if validation fails.
func ValidateEntities(files []FileEntity, functions []FunctionEntity, defines []DefinesEdge, calls []CallsEdge) error {
	var errors []*ValidationError
	errors = append(errors, validateFiles(files)...)
	errors = append(errors, validateFunctions(functions)...)
	errors = append(errors, validateDefinesEdges(defines)...)
	errors = append(errors, validateCallsEdges(calls)...)

	if len(errors) > 0 {
		var msgs []string
		for _, err := range errors {
			msgs = append(msgs, err.Error())
		}
		return fmt.Errorf("validation failed with %d error(s):\n%s", len(errors), strings.Join(msgs, "\n"))
	}
	return nil
}

func validateFiles(files []FileEntity) []*ValidationError {
	var errors []*ValidationError
	for _, file := range files {
		if file.ID == "" {
			errors = append(errors, &ValidationError{EntityType: "file", EntityID: file.Path, Field: "id", Message: "file ID cannot be empty"})
		}
		if file.Path == "" {
			errors = append(errors, &ValidationError{EntityType: "file", EntityID: file.ID, Field: "path", Message: "file path cannot be empty"})
		}
		if file.Hash == "" {
			errors = append(errors, &ValidationError{EntityType: "file", EntityID: file.ID, Field: "hash", Message: "file hash cannot be empty"})
		}
	}
	return errors
}

func validateFunctions(functions []FunctionEntity) []*ValidationError {
	var errors []*ValidationError
	embeddingDimension := -1

	for _, fn := range functions {
		errors = append(errors, validateFunctionBasic(fn)...)
		embErrors, dim := validateFunctionEmbedding(fn, embeddingDimension)
		errors = append(errors, embErrors...)
		if dim > 0 && embeddingDimension == -1 {
			embeddingDimension = dim
		}
	}
	return errors
}

func validateFunctionBasic(fn FunctionEntity) []*ValidationError {
	var errors []*ValidationError
	if fn.ID == "" {
		errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.Name, Field: "id", Message: "function ID cannot be empty"})
	}
	if fn.FilePath == "" {
		errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "file_path", Message: "function file_path cannot be empty"})
	}
	if fn.StartLine < 1 {
		errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "start_line", Message: "start_line must be >= 1"})
	}
	if fn.EndLine < fn.StartLine {
		errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "end_line", Message: "end_line must be >= start_line"})
	}
	return errors
}

func validateFunctionEmbedding(fn FunctionEntity, expectedDim int) ([]*ValidationError, int) {
	if fn.Embedding == nil {
		return nil, 0
	}
	var errors []*ValidationError
	dim := len(fn.Embedding)

	if dim > 0 && expectedDim > 0 && dim != expectedDim {
		errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "embedding", Message: fmt.Sprintf("embedding dimension mismatch: expected %d, got %d", expectedDim, dim)})
	}

	for i, v := range fn.Embedding {
		if math.IsNaN(float64(v)) {
			errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "embedding", Message: fmt.Sprintf("embedding contains NaN at index %d", i)})
			break
		}
		if math.IsInf(float64(v), 0) {
			errors = append(errors, &ValidationError{EntityType: "function", EntityID: fn.ID, Field: "embedding", Message: fmt.Sprintf("embedding contains Inf at index %d", i)})
			break
		}
	}
	return errors, dim
}

func validateDefinesEdges(defines []DefinesEdge) []*ValidationError {
	var errors []*ValidationError
	for i, edge := range defines {
		if edge.FileID == "" {
			errors = append(errors, &ValidationError{EntityType: "defines", EntityID: fmt.Sprintf("edge_%d", i), Field: "file_id", Message: "file_id cannot be empty"})
		}
		if edge.FunctionID == "" {
			errors = append(errors, &ValidationError{EntityType: "defines", EntityID: fmt.Sprintf("edge_%d", i), Field: "function_id", Message: "function_id cannot be empty"})
		}
	}
	return errors
}

func validateCallsEdges(calls []CallsEdge) []*ValidationError {
	var errors []*ValidationError
	for i, edge := range calls {
		if edge.CallerID == "" {
			errors = append(errors, &ValidationError{EntityType: "calls", EntityID: fmt.Sprintf("edge_%d", i), Field: "caller_id", Message: "caller_id cannot be empty"})
		}
		if edge.CalleeID == "" {
			errors = append(errors, &ValidationError{EntityType: "calls", EntityID: fmt.Sprintf("edge_%d", i), Field: "callee_id", Message: "callee_id cannot be empty"})
		}
	}
	return errors
}

// BuildMutations generates Datalog :put statements for all entities.
// Uses :put (upsert) for idempotency (re-running ingestion overwrites cleanly).
// The generated mutations are validated against server limits before sending,
// but field-level schema validation is the responsibility of the Primary Hub server.
//
// CozoDB requires each query to be wrapped in {} when executing multiple queries
// in a single script. See: https://docs.cozodb.org/en/latest/stored.html
func (db *DatalogBuilder) BuildMutations(files []FileEntity, functions []FunctionEntity, defines []DefinesEdge, calls []CallsEdge, imports ...[]ImportEntity) string {
	return db.BuildMutationsWithTypes(files, functions, nil, defines, nil, calls, imports...)
}

// BuildMutationsWithTypes generates Datalog :put statements for all entities including types.
// This is the full version that supports type indexing.
func (db *DatalogBuilder) BuildMutationsWithTypes(
	files []FileEntity,
	functions []FunctionEntity,
	types []TypeEntity,
	defines []DefinesEdge,
	definesTypes []DefinesTypeEdge,
	calls []CallsEdge,
	imports ...[]ImportEntity,
) string {
	// Emit Cozo-compatible statements wrapped in {} for batch execution:
	// Each query block must be enclosed in curly braces when chaining multiple queries.
	var buf strings.Builder

	// File entities
	for _, file := range files {
		buf.WriteString("{ ?[id, path, hash, language, size] <- [[")
		buf.WriteString(strings.Join([]string{
			quoteString(file.ID),
			quoteString(file.Path),
			quoteString(file.Hash),
			quoteString(file.Language),
			fmt.Sprintf("%d", file.Size),
		}, ", "))
		buf.WriteString("]] :put cie_file { id, path, hash, language, size } }\n")
	}

	// Function entities (v3: split into 3 tables for performance)
	for _, fn := range functions {
		// 1. Core metadata (cie_function) - lightweight, ~500 bytes/row
		buf.WriteString("{ ?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [[")
		buf.WriteString(strings.Join([]string{
			quoteString(fn.ID),
			quoteString(fn.Name),
			quoteString(fn.Signature),
			quoteString(fn.FilePath),
			fmt.Sprintf("%d", fn.StartLine),
			fmt.Sprintf("%d", fn.EndLine),
			fmt.Sprintf("%d", fn.StartCol),
			fmt.Sprintf("%d", fn.EndCol),
		}, ", "))
		buf.WriteString("]] :put cie_function { id, name, signature, file_path, start_line, end_line, start_col, end_col } }\n")

		// 2. Code text (cie_function_code) - lazy loaded
		buf.WriteString("{ ?[function_id, code_text] <- [[")
		buf.WriteString(strings.Join([]string{
			quoteString(fn.ID),
			quoteString(fn.CodeText),
		}, ", "))
		buf.WriteString("]] :put cie_function_code { function_id, code_text } }\n")

		// 3. Embedding (cie_function_embedding) - used by HNSW
		// Skip if embedding is empty (e.g., embedding provider unavailable)
		if len(fn.Embedding) > 0 {
			embeddingStr := formatFloatArray(fn.Embedding)
			buf.WriteString("{ ?[function_id, embedding] <- [[")
			buf.WriteString(strings.Join([]string{
				quoteString(fn.ID),
				embeddingStr,
			}, ", "))
			buf.WriteString("]] :put cie_function_embedding { function_id, embedding } }\n")
		}
	}

	// Type entities (v3: split into 3 tables for performance)
	for _, t := range types {
		// 1. Core metadata (cie_type) - lightweight
		buf.WriteString("{ ?[id, name, kind, file_path, start_line, end_line, start_col, end_col] <- [[")
		buf.WriteString(strings.Join([]string{
			quoteString(t.ID),
			quoteString(t.Name),
			quoteString(t.Kind),
			quoteString(t.FilePath),
			fmt.Sprintf("%d", t.StartLine),
			fmt.Sprintf("%d", t.EndLine),
			fmt.Sprintf("%d", t.StartCol),
			fmt.Sprintf("%d", t.EndCol),
		}, ", "))
		buf.WriteString("]] :put cie_type { id, name, kind, file_path, start_line, end_line, start_col, end_col } }\n")

		// 2. Code text (cie_type_code) - lazy loaded
		buf.WriteString("{ ?[type_id, code_text] <- [[")
		buf.WriteString(strings.Join([]string{
			quoteString(t.ID),
			quoteString(t.CodeText),
		}, ", "))
		buf.WriteString("]] :put cie_type_code { type_id, code_text } }\n")

		// 3. Embedding (cie_type_embedding) - used by HNSW
		// Skip if embedding is empty (e.g., embedding provider unavailable)
		if len(t.Embedding) > 0 {
			embeddingStr := formatFloatArray(t.Embedding)
			buf.WriteString("{ ?[type_id, embedding] <- [[")
			buf.WriteString(strings.Join([]string{
				quoteString(t.ID),
				embeddingStr,
			}, ", "))
			buf.WriteString("]] :put cie_type_embedding { type_id, embedding } }\n")
		}
	}

	// Defines edges (store as entity with stable id to avoid composite-key quirks)
	for _, edge := range defines {
		edgeID := quoteString("def:" + edge.FileID + "|" + edge.FunctionID)
		buf.WriteString("{ ?[id, file_id, function_id] <- [[")
		buf.WriteString(strings.Join([]string{
			edgeID,
			quoteString(edge.FileID),
			quoteString(edge.FunctionID),
		}, ", "))
		buf.WriteString("]] :put cie_defines { id, file_id, function_id } }\n")
	}

	// DefinesType edges (store as entity with stable id)
	for _, edge := range definesTypes {
		edgeID := quoteString("deft:" + edge.FileID + "|" + edge.TypeID)
		buf.WriteString("{ ?[id, file_id, type_id] <- [[")
		buf.WriteString(strings.Join([]string{
			edgeID,
			quoteString(edge.FileID),
			quoteString(edge.TypeID),
		}, ", "))
		buf.WriteString("]] :put cie_defines_type { id, file_id, type_id } }\n")
	}

	// Calls edges (store as entity with stable id)
	for _, edge := range calls {
		edgeID := quoteString("call:" + edge.CallerID + "|" + edge.CalleeID)
		buf.WriteString("{ ?[id, caller_id, callee_id] <- [[")
		buf.WriteString(strings.Join([]string{
			edgeID,
			quoteString(edge.CallerID),
			quoteString(edge.CalleeID),
		}, ", "))
		buf.WriteString("]] :put cie_calls { id, caller_id, callee_id } }\n")
	}

	// Import entities (optional, for cross-package resolution)
	if len(imports) > 0 {
		for _, imp := range imports[0] {
			buf.WriteString("{ ?[id, file_path, import_path, alias, start_line] <- [[")
			buf.WriteString(strings.Join([]string{
				quoteString(imp.ID),
				quoteString(imp.FilePath),
				quoteString(imp.ImportPath),
				quoteString(imp.Alias),
				fmt.Sprintf("%d", imp.StartLine),
			}, ", "))
			buf.WriteString("]] :put cie_import { id, file_path, import_path, alias, start_line } }\n")
		}
	}

	return buf.String()
}

// quoteString quotes a string for CozoDB Datalog using single quotes.
// Single-quoted strings in CozoDB:
// - Backslash must be escaped: \ -> \\
// - Single quote must be escaped: ' -> \'
// - Double quotes are literal (no escape needed)
// - Other characters including newlines are preserved as-is
func quoteString(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 10)
	buf.WriteByte('\'')

	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString("\\\\")
		case '\'':
			buf.WriteString("\\'")
		default:
			// Skip null bytes and other problematic control chars (except whitespace)
			if r == 0 {
				continue
			}
			buf.WriteRune(r)
		}
	}

	buf.WriteByte('\'')
	return buf.String()
}

// formatFloatArray formats a float32 array as a Datalog array literal.
// Supports any embedding dimension (768 for nomic-embed-text, 1536 for OpenAI, etc.)
// If input is empty, returns an empty array "[]".
func formatFloatArray(arr []float32) string {
	if len(arr) == 0 {
		return "[]"
	}

	var parts []string
	for _, v := range arr {
		parts = append(parts, formatFloat(v))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

// formatFloat formats a float32 for Datalog.
// Returns "0" for NaN/Inf as a safety fallback (validation should catch these earlier).
func formatFloat(f float32) string {
	f64 := float64(f)
	// Safety check: NaN/Inf should be caught by validation, but if they slip through,
	// serialize as 0 to avoid breaking Datalog syntax
	if math.IsNaN(f64) || math.IsInf(f64, 0) {
		return "0"
	}
	// Use enough precision but avoid scientific notation for readability
	return strconv.FormatFloat(f64, 'f', -1, 32)
}

// CountMutations estimates the number of mutations in a Datalog script.
// This is approximate but useful for batching decisions.
func CountMutations(script string) int {
	// Count :insert, :replace, :put, :update, :rm statements
	count := 0
	lines := strings.Split(script, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ":insert") ||
			strings.HasPrefix(trimmed, ":replace") ||
			strings.HasPrefix(trimmed, ":put") ||
			strings.HasPrefix(trimmed, ":update") ||
			strings.HasPrefix(trimmed, ":rm") {
			count++
		}
	}
	return count
}

// =============================================================================
// INCREMENTAL MUTATIONS (F1.M2)
// =============================================================================
//
// For incremental ingestion, we need to emit :rm statements to delete stale
// entities that no longer exist in the codebase.

// DeletionSet specifies entities to delete.
type DeletionSet struct {
	// FileIDs are file IDs to delete
	FileIDs []string

	// FunctionIDs are function IDs to delete
	FunctionIDs []string

	// TypeIDs are type IDs to delete (v3: cascades to cie_type_code and cie_type_embedding)
	TypeIDs []string

	// DefinesEdges are (file_id, function_id) pairs to delete (DEPRECATED: use DefinesEdgeIDs)
	DefinesEdges []DefinesEdge

	// DefinesEdgeIDs are the primary key IDs of defines edges to delete
	DefinesEdgeIDs []string

	// DefinesTypeEdges are (file_id, type_id) pairs to delete (DEPRECATED: use DefinesTypeEdgeIDs)
	DefinesTypeEdges []DefinesTypeEdge

	// DefinesTypeEdgeIDs are the primary key IDs of defines_type edges to delete
	DefinesTypeEdgeIDs []string

	// CallsEdges are (caller_id, callee_id) pairs to delete (DEPRECATED: use CallsEdgeIDs)
	CallsEdges []CallsEdge

	// CallsEdgeIDs are the primary key IDs of calls edges to delete
	CallsEdgeIDs []string

	// PathsToSweep are file paths for which we must perform defensive cleanup
	// when the manifest does not contain prior state (e.g., rename/delete after crash).
	// For each path we will issue query-driven deletions in safe order:
	//   1) calls edges with caller in file_path
	//   2) defines edges by file_path
	//   3) functions by file_path
	//   4) file entity by path
	PathsToSweep []string

	// EdgesOnlyPaths are file paths for which we must defensively remove only edges
	// (calls and defines) when the manifest entry is missing/stale for a modified file.
	// This avoids deleting entities for files that still exist.
	EdgesOnlyPaths []string
}

// BuildDeletions generates Datalog :rm statements for deleting stale entities.
// Order: edges first (to avoid orphan references), then entities.
//
// NOTE: EdgesOnlyPaths and PathsToSweep are no longer used.
// Path-based deletion requires proper CozoScript with joins, but with the
// CozoDB-based incremental approach we always have IDs from queries.
// These fields are kept for backwards compatibility but generate no output.
func (db *DatalogBuilder) BuildDeletions(deletions DeletionSet) string {
	var buf strings.Builder

	// EdgesOnlyPaths and PathsToSweep previously used custom commands like
	// :rm_calls_by_caller_file_path that only worked in MockPrimaryHub.
	// With CozoDB-based approach, we always query for IDs before deletion,
	// so these fields should be empty. Skip them to avoid parse errors.
	// If needed, proper path-based deletion would require chained queries:
	//   { ?[caller_id, callee_id] := *cie_calls{...}, *cie_function{...}, file_path = "..."
	//     :rm cie_calls { caller_id, callee_id } }

	// Delete calls edges first (references to functions)
	// CozoDB syntax: ?[keys] <- [[values]] :rm table {keys}
	// Use CallsEdgeIDs (primary key) instead of caller_id/callee_id
	for _, id := range deletions.CallsEdgeIDs {
		stmt := fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_calls {id} }\n", quoteString(id))
		buf.WriteString(stmt)
	}
	// DEPRECATED: CallsEdges with caller_id/callee_id (kept for backwards compatibility)
	// The schema uses 'id' as primary key, so this won't work with real CozoDB
	for _, edge := range deletions.CallsEdges {
		stmt := fmt.Sprintf("{ ?[caller_id, callee_id] <- [[%s, %s]] :rm cie_calls {caller_id, callee_id} }\n",
			quoteString(edge.CallerID),
			quoteString(edge.CalleeID),
		)
		buf.WriteString(stmt)
	}

	// Delete defines edges using primary key 'id'
	for _, id := range deletions.DefinesEdgeIDs {
		stmt := fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_defines {id} }\n", quoteString(id))
		buf.WriteString(stmt)
	}
	// DEPRECATED: DefinesEdges with file_id/function_id (kept for backwards compatibility)
	// The schema uses 'id' as primary key, so this won't work with real CozoDB
	for _, edge := range deletions.DefinesEdges {
		stmt := fmt.Sprintf("{ ?[file_id, function_id] <- [[%s, %s]] :rm cie_defines {file_id, function_id} }\n",
			quoteString(edge.FileID),
			quoteString(edge.FunctionID),
		)
		buf.WriteString(stmt)
	}

	// Delete function entities (v3: cascade to code and embedding tables)
	for _, id := range deletions.FunctionIDs {
		qid := quoteString(id)
		// Delete from all 3 tables using chained queries
		buf.WriteString(fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_function {id} }\n", qid))
		buf.WriteString(fmt.Sprintf("{ ?[function_id] <- [[%s]] :rm cie_function_code {function_id} }\n", qid))
		buf.WriteString(fmt.Sprintf("{ ?[function_id] <- [[%s]] :rm cie_function_embedding {function_id} }\n", qid))
	}

	// Delete defines_type edges using primary key 'id'
	for _, id := range deletions.DefinesTypeEdgeIDs {
		stmt := fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_defines_type {id} }\n", quoteString(id))
		buf.WriteString(stmt)
	}
	// DEPRECATED: DefinesTypeEdges with file_id/type_id (kept for backwards compatibility)
	// The schema uses 'id' as primary key, so this won't work with real CozoDB
	for _, edge := range deletions.DefinesTypeEdges {
		stmt := fmt.Sprintf("{ ?[file_id, type_id] <- [[%s, %s]] :rm cie_defines_type {file_id, type_id} }\n",
			quoteString(edge.FileID),
			quoteString(edge.TypeID),
		)
		buf.WriteString(stmt)
	}

	// Delete type entities (v3: cascade to code and embedding tables)
	for _, id := range deletions.TypeIDs {
		qid := quoteString(id)
		// Delete from all 3 tables using chained queries
		buf.WriteString(fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_type {id} }\n", qid))
		buf.WriteString(fmt.Sprintf("{ ?[type_id] <- [[%s]] :rm cie_type_code {type_id} }\n", qid))
		buf.WriteString(fmt.Sprintf("{ ?[type_id] <- [[%s]] :rm cie_type_embedding {type_id} }\n", qid))
	}

	// Delete file entities
	for _, id := range deletions.FileIDs {
		stmt := fmt.Sprintf("{ ?[id] <- [[%s]] :rm cie_file {id} }\n", quoteString(id))
		buf.WriteString(stmt)
	}

	return buf.String()
}

// BuildFileDefinesDeletion generates :rm statements to delete all defines edges for a file.
// This is used when a file is modified to clear old edges before adding new ones.
// The pattern uses a query to find all edges for the file.
func (db *DatalogBuilder) BuildFileDefinesDeletion(fileID string) string {
	// Use a query-based deletion: find all function_ids defined by this file and remove them
	// CozoDB syntax: :rm table[key] - removes the row with that key
	// For defines edges where file_id is part of the composite key, we need to list them
	// This approach requires knowing the function_ids, which we have from the manifest
	return "" // Handled by explicit edge deletion in DeletionSet
}

// BuildIncrementalMutations generates a combined script with deletions and upserts.
// Order: deletions first, then upserts. This ensures:
// - Old edges are removed before new entities might reuse IDs
// - No orphan references during the transaction
func (db *DatalogBuilder) BuildIncrementalMutations(
	deletions DeletionSet,
	files []FileEntity,
	functions []FunctionEntity,
	defines []DefinesEdge,
	calls []CallsEdge,
) string {
	return db.BuildIncrementalMutationsWithTypes(deletions, files, functions, nil, defines, nil, calls)
}

// BuildIncrementalMutationsWithTypes generates a combined script with deletions and upserts including types.
func (db *DatalogBuilder) BuildIncrementalMutationsWithTypes(
	deletions DeletionSet,
	files []FileEntity,
	functions []FunctionEntity,
	types []TypeEntity,
	defines []DefinesEdge,
	definesTypes []DefinesTypeEdge,
	calls []CallsEdge,
) string {
	var buf strings.Builder

	// Phase 1: Deletions (edges first, then entities)
	deletionScript := db.BuildDeletions(deletions)
	if deletionScript != "" {
		buf.WriteString("// === DELETIONS (stale entities/edges) ===\n")
		buf.WriteString(deletionScript)
		buf.WriteString("\n")
	}

	// Phase 2: Upserts (entities first, then edges)
	upsertScript := db.BuildMutationsWithTypes(files, functions, types, defines, definesTypes, calls)
	if upsertScript != "" {
		buf.WriteString("// === UPSERTS (new/modified entities) ===\n")
		buf.WriteString(upsertScript)
	}

	return buf.String()
}

// BuildCallsEdgesDeletionForFile generates deletions for all calls edges where
// the caller belongs to the specified file (by function IDs).
func (db *DatalogBuilder) BuildCallsEdgesDeletionForFile(callerFunctionIDs []string, allCallsEdges []CallsEdge) []CallsEdge {
	// Build set of caller IDs for fast lookup
	callerSet := make(map[string]bool)
	for _, id := range callerFunctionIDs {
		callerSet[id] = true
	}

	// Find calls edges where caller is in the set
	var toDelete []CallsEdge
	for _, edge := range allCallsEdges {
		if callerSet[edge.CallerID] {
			toDelete = append(toDelete, edge)
		}
	}

	return toDelete
}

// IncrementalMutationStats tracks mutation counts for observability.
type IncrementalMutationStats struct {
	FilesDeleted      int
	FunctionsDeleted  int
	DefinesDeleted    int
	CallsDeleted      int
	FilesUpserted     int
	FunctionsUpserted int
	DefinesUpserted   int
	CallsUpserted     int
	TotalMutations    int
}

// ComputeStats computes mutation statistics from deletions and upserts.
func ComputeIncrementalStats(
	deletions DeletionSet,
	files []FileEntity,
	functions []FunctionEntity,
	defines []DefinesEdge,
	calls []CallsEdge,
) IncrementalMutationStats {
	return IncrementalMutationStats{
		FilesDeleted:      len(deletions.FileIDs),
		FunctionsDeleted:  len(deletions.FunctionIDs),
		DefinesDeleted:    len(deletions.DefinesEdges),
		CallsDeleted:      len(deletions.CallsEdges),
		FilesUpserted:     len(files),
		FunctionsUpserted: len(functions),
		DefinesUpserted:   len(defines),
		CallsUpserted:     len(calls),
		TotalMutations: len(deletions.FileIDs) + len(deletions.FunctionIDs) +
			len(deletions.DefinesEdges) + len(deletions.CallsEdges) +
			len(files) + len(functions) + len(defines) + len(calls),
	}
}
