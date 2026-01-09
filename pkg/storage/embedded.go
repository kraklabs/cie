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
	"sync"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// EmbeddedBackend implements Backend using a local CozoDB instance.
// This is the default backend for standalone/open-source CIE.
type EmbeddedBackend struct {
	db     *cozo.CozoDB
	mu     sync.RWMutex
	closed bool
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
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// Open CozoDB
	db, err := cozo.New(config.Engine, config.DataDir, nil)
	if err != nil {
		return nil, fmt.Errorf("open cozodb: %w", err)
	}

	return &EmbeddedBackend{
		db: &db,
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
func (b *EmbeddedBackend) EnsureSchema() error {
	schema := `
// CIE Ingestion Schema v3 - Vertically Partitioned
// File entities: represents source files in the repository
:create cie_file {
	id: String =>
	path: String,
	hash: String,
	language: String,
	size: Int
}

// Function entities: lightweight metadata (~500 bytes/row)
:create cie_function {
	id: String =>
	name: String,
	signature: String,
	file_path: String,
	start_line: Int,
	end_line: Int,
	start_col: Int,
	end_col: Int
}

// Function code text: lazy loaded only when displaying source
:create cie_function_code {
	function_id: String =>
	code_text: String
}

// Function embeddings: used only for HNSW semantic search
:create cie_function_embedding {
	function_id: String =>
	embedding: <F32; 1536>
}

// Defines edges: file -> function
:create cie_defines {
	id: String =>
	file_id: String,
	function_id: String
}

// Calls edges: function -> function
:create cie_calls {
	id: String =>
	caller_id: String,
	callee_id: String
}

// Import entities
:create cie_import {
	id: String =>
	file_path: String,
	import_path: String,
	alias: String,
	start_line: Int
}

// Type entities
:create cie_type {
	id: String =>
	name: String,
	kind: String,
	file_path: String,
	start_line: Int,
	end_line: Int,
	start_col: Int,
	end_col: Int
}

// Type code text
:create cie_type_code {
	type_id: String =>
	code_text: String
}

// Type embeddings
:create cie_type_embedding {
	type_id: String =>
	embedding: <F32; 1536>
}

// Defines type edges: file -> type
:create cie_defines_type {
	id: String =>
	file_id: String,
	type_id: String
}
`
	// Try to create each table individually, ignoring "already exists" errors
	tables := []string{
		`:create cie_file { id: String => path: String, hash: String, language: String, size: Int }`,
		`:create cie_function { id: String => name: String, signature: String, file_path: String, start_line: Int, end_line: Int, start_col: Int, end_col: Int }`,
		`:create cie_function_code { function_id: String => code_text: String }`,
		`:create cie_function_embedding { function_id: String => embedding: <F32; 1536> }`,
		`:create cie_defines { id: String => file_id: String, function_id: String }`,
		`:create cie_calls { id: String => caller_id: String, callee_id: String }`,
		`:create cie_import { id: String => file_path: String, import_path: String, alias: String, start_line: Int }`,
		`:create cie_type { id: String => name: String, kind: String, file_path: String, start_line: Int, end_line: Int, start_col: Int, end_col: Int }`,
		`:create cie_type_code { type_id: String => code_text: String }`,
		`:create cie_type_embedding { type_id: String => embedding: <F32; 1536> }`,
		`:create cie_defines_type { id: String => file_id: String, type_id: String }`,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, table := range tables {
		_, err := b.db.Run(table, nil)
		if err != nil {
			// Ignore "already exists" errors
			// CozoDB returns error message containing "already exists"
			continue
		}
	}

	// Suppress unused variable warning
	_ = schema

	return nil
}

// CreateHNSWIndex creates HNSW indexes for semantic search.
// Should be called after schema creation.
func (b *EmbeddedBackend) CreateHNSWIndex() error {
	indexes := []string{
		`::hnsw create cie_function_embedding:hnsw_idx { dim: 1536, m: 16, ef_construction: 200, fields: [embedding] }`,
		`::hnsw create cie_type_embedding:hnsw_idx { dim: 1536, m: 16, ef_construction: 200, fields: [embedding] }`,
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
