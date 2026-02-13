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

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// EmbeddedBackend implements Backend using a local CozoDB instance.
// This is the default backend for standalone/open-source CIE.
type EmbeddedBackend struct {
	db                  *cozo.CozoDB
	mu                  sync.RWMutex
	closed              bool
	embeddingDimensions int
}

// EmbeddedConfig configures the embedded backend.
type EmbeddedConfig struct {
	// DataDir is the directory where CozoDB stores its data.
	// Defaults to ~/.cie/data/<project_id>
	DataDir string

	// Engine is the CozoDB storage engine: "rocksdb", "sqlite", or "mem".
	// Defaults to "rocksdb" for persistence.
	Engine string

	// ProjectID is used to namespace the data directory.
	ProjectID string

	// EmbeddingDimensions is the vector size for embeddings.
	// Defaults to 768 (nomic-embed-text). Use 1536 for OpenAI.
	EmbeddingDimensions int
}

// NewEmbeddedBackend creates a new embedded CozoDB backend.
func NewEmbeddedBackend(config EmbeddedConfig) (*EmbeddedBackend, error) {
	// Set defaults
	if config.Engine == "" {
		config.Engine = "rocksdb"
	}
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		config.DataDir = filepath.Join(homeDir, ".cie", "data")
		if config.ProjectID != "" {
			config.DataDir = filepath.Join(config.DataDir, config.ProjectID)
		}
	}

	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Open CozoDB
	db, err := cozo.New(config.Engine, config.DataDir, nil)
	if err != nil {
		if config.Engine == "rocksdb" && isStaleLock(config.DataDir) {
			lockPath := filepath.Join(config.DataDir, "LOCK")
			if err2 := os.Remove(lockPath); err2 == nil {
				db, err = cozo.New(config.Engine, config.DataDir, nil)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("open cozodb: %w", err)
		}
	}

	// Default embedding dimensions to 768 (nomic-embed-text)
	embeddingDim := config.EmbeddingDimensions
	if embeddingDim <= 0 {
		embeddingDim = 768
	}

	return &EmbeddedBackend{
		db:                  &db,
		embeddingDimensions: embeddingDim,
	}, nil
}

// Query executes a read-only Datalog query.
func (b *EmbeddedBackend) Query(ctx context.Context, datalog string) (*QueryResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return nil, fmt.Errorf("backend is closed")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result, err := b.db.RunReadOnly(datalog, nil)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return FromNamedRows(result), nil
}

// Execute runs a Datalog mutation.
func (b *EmbeddedBackend) Execute(ctx context.Context, datalog string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return fmt.Errorf("backend is closed")
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	_, err := b.db.Run(datalog, nil)
	if err != nil {
		return fmt.Errorf("execute failed: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (b *EmbeddedBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	b.db.Close()
	return nil
}

// DB returns the underlying CozoDB instance for advanced operations.
// Use with caution - prefer the Backend interface methods.
func (b *EmbeddedBackend) DB() *cozo.CozoDB {
	return b.db
}

// EnsureSchema creates the CIE tables if they don't exist.
// This is idempotent and safe to call multiple times.
// Uses the embedding dimensions configured in the backend.
func (b *EmbeddedBackend) EnsureSchema() error {
	dim := b.embeddingDimensions
	if dim <= 0 {
		dim = 768 // default for nomic-embed-text
	}

	// Create each table individually, ignoring "already exists" errors
	tables := []string{
		`:create cie_file { id: String => path: String, hash: String, language: String, size: Int }`,
		`:create cie_function { id: String => name: String, signature: String, file_path: String, start_line: Int, end_line: Int, start_col: Int, end_col: Int }`,
		`:create cie_function_code { function_id: String => code_text: String }`,
		fmt.Sprintf(`:create cie_function_embedding { function_id: String => embedding: <F32; %d> }`, dim),
		`:create cie_defines { id: String => file_id: String, function_id: String }`,
		`:create cie_calls { id: String => caller_id: String, callee_id: String, call_line: Int default 0 }`,
		`:create cie_import { id: String => file_path: String, import_path: String, alias: String, start_line: Int }`,
		`:create cie_type { id: String => name: String, kind: String, file_path: String, start_line: Int, end_line: Int, start_col: Int, end_col: Int }`,
		`:create cie_type_code { type_id: String => code_text: String }`,
		fmt.Sprintf(`:create cie_type_embedding { type_id: String => embedding: <F32; %d> }`, dim),
		`:create cie_defines_type { id: String => file_id: String, type_id: String }`,
		// Struct field entities for interface dispatch resolution
		`:create cie_field { id: String => struct_name: String, field_name: String, field_type: String, file_path: String, line: Int }`,
		// Implements edges: concrete type -> interface
		`:create cie_implements { id: String => type_name: String, interface_name: String, file_path: String }`,
		// Project metadata for incremental indexing
		`:create cie_project_meta { key: String => value: String }`,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, table := range tables {
		_, err := b.db.Run(table, nil)
		if err != nil {
			// Ignore "already exists" errors, but log others
			errStr := err.Error()
			if strings.Contains(errStr, "already exists") ||
				strings.Contains(errStr, "conflicts with an existing one") {
				continue
			}
			// For other errors (like schema mismatch), return the error
			return fmt.Errorf("create table failed: %w", err)
		}
	}

	// Schema migrations: add columns introduced in newer versions.
	// CozoDB doesn't support ALTER TABLE, so we migrate by copying data.
	b.migrateCallsCallLine()

	return nil
}

// migrateCallsCallLine adds the call_line column to cie_calls if it was created
// with an older schema (pre-v0.7.9). CozoDB doesn't support ALTER TABLE, so we
// copy data to a temp table, recreate with the new schema, and copy back.
// Caller must hold b.mu.
func (b *EmbeddedBackend) migrateCallsCallLine() {
	// Probe: try reading call_line — if it works, no migration needed.
	_, err := b.db.Run(`?[id] := *cie_calls { id, call_line } :limit 1`, nil)
	if err == nil {
		return
	}

	// Copy existing data to temp table
	_, err = b.db.Run(`?[id, caller_id, callee_id] := *cie_calls { id, caller_id, callee_id } :replace cie_calls_mig { id: String => caller_id: String, callee_id: String }`, nil)
	if err != nil {
		return // can't migrate, queries will use fallback
	}

	// Drop old table and recreate with new schema
	_, _ = b.db.Run(`::remove cie_calls`, nil)
	_, err = b.db.Run(`:create cie_calls { id: String => caller_id: String, callee_id: String, call_line: Int default 0 }`, nil)
	if err != nil {
		// Restore from temp if create fails
		_, _ = b.db.Run(`?[id, caller_id, callee_id] := *cie_calls_mig { id, caller_id, callee_id } :replace cie_calls { id: String => caller_id: String, callee_id: String }`, nil)
		_, _ = b.db.Run(`::remove cie_calls_mig`, nil)
		return
	}

	// Copy data back with call_line=0
	_, _ = b.db.Run(`?[id, caller_id, callee_id, call_line] := *cie_calls_mig { id, caller_id, callee_id }, call_line = 0 :put cie_calls { id, caller_id, callee_id, call_line }`, nil)
	_, _ = b.db.Run(`::remove cie_calls_mig`, nil)
}

// CreateHNSWIndex creates HNSW indexes for semantic search.
// Should be called after schema creation.
// dimensions: embedding vector size (768 for nomic-embed-text, 1536 for OpenAI)
func (b *EmbeddedBackend) CreateHNSWIndex(dimensions int) error {
	if dimensions <= 0 {
		dimensions = 768 // default for nomic-embed-text
	}
	// Use Cosine distance for semantic similarity (returns 0-2, where 0 = identical)
	indexes := []string{
		fmt.Sprintf(`::hnsw create cie_function_embedding:embedding_idx { dim: %d, m: 16, ef_construction: 200, distance: Cosine, fields: [embedding] }`, dimensions),
		fmt.Sprintf(`::hnsw create cie_type_embedding:embedding_idx { dim: %d, m: 16, ef_construction: 200, distance: Cosine, fields: [embedding] }`, dimensions),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, idx := range indexes {
		_, err := b.db.Run(idx, nil)
		if err != nil {
			// Ignore "already exists" errors
			continue
		}
	}

	return nil
}

// GetProjectMeta retrieves a metadata value by key.
// Returns empty string if key doesn't exist.
func (b *EmbeddedBackend) GetProjectMeta(key string) (string, error) {
	query := `?[value] := *cie_project_meta{key, value}, key = $key`
	params := map[string]interface{}{"key": key}

	b.mu.RLock()
	result, err := b.db.Run(query, params)
	b.mu.RUnlock()

	if err != nil {
		return "", err
	}

	if len(result.Rows) == 0 {
		return "", nil
	}

	if val, ok := result.Rows[0][0].(string); ok {
		return val, nil
	}
	return "", nil
}

// SetProjectMeta sets a metadata value by key.
func (b *EmbeddedBackend) SetProjectMeta(key, value string) error {
	query := `?[key, value] <- [[$key, $value]] :put cie_project_meta { key, value }`
	params := map[string]interface{}{"key": key, "value": value}

	b.mu.Lock()
	_, err := b.db.Run(query, params)
	b.mu.Unlock()

	return err
}

// GetLastIndexedSHA retrieves the last successfully indexed git SHA.
func (b *EmbeddedBackend) GetLastIndexedSHA() (string, error) {
	return b.GetProjectMeta("last_indexed_sha")
}

// SetLastIndexedSHA stores the last successfully indexed git SHA.
func (b *EmbeddedBackend) SetLastIndexedSHA(sha string) error {
	return b.SetProjectMeta("last_indexed_sha", sha)
}

// DeleteEntitiesForFile removes all entities associated with a file path.
// This is used during incremental indexing when files are deleted or modified.
func (b *EmbeddedBackend) DeleteEntitiesForFile(filePath string) error {
	// Delete in order: edges first, then entities
	queries := []string{
		// Delete call edges where caller or callee is in this file
		`?[id] := *cie_calls{id, caller_id}, *cie_function{id: caller_id, file_path}, file_path = $path
		 :rm cie_calls {id}`,
		`?[id] := *cie_calls{id, callee_id}, *cie_function{id: callee_id, file_path}, file_path = $path
		 :rm cie_calls {id}`,
		// Delete defines edges for this file
		`?[id] := *cie_defines{id, file_id}, *cie_file{id: file_id, path}, path = $path
		 :rm cie_defines {id}`,
		// Delete defines_type edges for this file
		`?[id] := *cie_defines_type{id, file_id}, *cie_file{id: file_id, path}, path = $path
		 :rm cie_defines_type {id}`,
		// Delete function embeddings
		`?[function_id] := *cie_function{id: function_id, file_path}, file_path = $path
		 :rm cie_function_embedding {function_id}`,
		// Delete function code
		`?[function_id] := *cie_function{id: function_id, file_path}, file_path = $path
		 :rm cie_function_code {function_id}`,
		// Delete functions
		`?[id] := *cie_function{id, file_path}, file_path = $path
		 :rm cie_function {id}`,
		// Delete type embeddings
		`?[type_id] := *cie_type{id: type_id, file_path}, file_path = $path
		 :rm cie_type_embedding {type_id}`,
		// Delete type code
		`?[type_id] := *cie_type{id: type_id, file_path}, file_path = $path
		 :rm cie_type_code {type_id}`,
		// Delete types
		`?[id] := *cie_type{id, file_path}, file_path = $path
		 :rm cie_type {id}`,
		// Delete imports for this file
		`?[id] := *cie_import{id, file_path}, file_path = $path
		 :rm cie_import {id}`,
		// Delete the file itself
		`?[id] := *cie_file{id, path}, path = $path
		 :rm cie_file {id}`,
	}

	params := map[string]interface{}{"path": filePath}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, query := range queries {
		if _, err := b.db.Run(query, params); err != nil {
			// Log but continue - some queries may fail if entities don't exist
			continue
		}
	}

	return nil
}

// isStaleLock checks if the RocksDB LOCK file in dataDir is stale.
// A lock is stale if the LOCK file exists but no process holds an
// exclusive flock on it (the original process died).
func isStaleLock(dataDir string) bool {
	lockPath := filepath.Join(dataDir, "LOCK")
	f, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	// Non-blocking exclusive lock attempt
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return false // Another process holds the lock
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return true
}
