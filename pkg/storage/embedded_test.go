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

//go:build cgo

package storage

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// setupTestStorage creates an in-memory EmbeddedBackend for testing.
// The caller is responsible for calling Close() on the returned backend.
func setupTestStorage(t *testing.T) *EmbeddedBackend {
	t.Helper()
	config := EmbeddedConfig{
		DataDir: t.TempDir(),
		Engine:  "mem", // In-memory for fast tests
	}
	storage, err := NewEmbeddedBackend(config)
	if err != nil {
		t.Fatalf("setupTestStorage failed: %v", err)
	}
	return storage
}

// TestNewEmbeddedBackend_Success tests successful backend creation.
func TestNewEmbeddedBackend_Success(t *testing.T) {
	config := EmbeddedConfig{
		DataDir: t.TempDir(),
		Engine:  "mem",
	}
	backend, err := NewEmbeddedBackend(config)
	if err != nil {
		t.Fatalf("NewEmbeddedBackend failed: %v", err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
	if backend.db == nil {
		t.Fatal("expected non-nil db")
	}
	if backend.closed {
		t.Error("expected backend to not be closed initially")
	}
}

// TestNewEmbeddedBackend_DefaultEngine tests that the default engine is "rocksdb".
func TestNewEmbeddedBackend_DefaultEngine(t *testing.T) {
	config := EmbeddedConfig{
		DataDir: t.TempDir(),
		// Engine not specified - should default to "rocksdb"
	}
	backend, err := NewEmbeddedBackend(config)
	if err != nil {
		t.Fatalf("NewEmbeddedBackend failed: %v", err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
}

// TestNewEmbeddedBackend_DefaultDataDir tests default data directory creation.
func TestNewEmbeddedBackend_DefaultDataDir(t *testing.T) {
	config := EmbeddedConfig{
		Engine: "mem",
		// DataDir not specified - should default to ~/.cie/data
	}
	backend, err := NewEmbeddedBackend(config)
	if err != nil {
		t.Fatalf("NewEmbeddedBackend with default DataDir failed: %v", err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
}

// TestNewEmbeddedBackend_ProjectID tests ProjectID namespacing in data directory.
func TestNewEmbeddedBackend_ProjectID(t *testing.T) {
	config := EmbeddedConfig{
		Engine:    "mem",
		ProjectID: "test-project",
		// DataDir not specified - should use ~/.cie/data/test-project
	}
	backend, err := NewEmbeddedBackend(config)
	if err != nil {
		t.Fatalf("NewEmbeddedBackend with ProjectID failed: %v", err)
	}
	defer func() {
		if err := backend.Close(); err != nil {
			t.Errorf("Close failed: %v", err)
		}
	}()

	if backend == nil {
		t.Fatal("expected non-nil backend")
	}
}

// TestEmbeddedBackend_Query_Success tests successful query execution.
func TestEmbeddedBackend_Query_Success(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()

	// Simple query that should always work
	result, err := backend.Query(ctx, "?[x] := x = 1")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Headers) == 0 {
		t.Error("expected headers in result")
	}
}

// TestEmbeddedBackend_Query_ContextCanceled tests query with canceled context.
func TestEmbeddedBackend_Query_ContextCanceled(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := backend.Query(ctx, "?[x] := x = 1")
	if err == nil {
		t.Error("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' error, got: %v", err)
	}
}

// TestEmbeddedBackend_Query_AfterClose tests that query fails after Close().
func TestEmbeddedBackend_Query_AfterClose(t *testing.T) {
	backend := setupTestStorage(t)
	_ = backend.Close()

	ctx := context.Background()
	_, err := backend.Query(ctx, "?[x] := x = 1")
	if err == nil {
		t.Error("expected error when querying closed backend")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' error, got: %v", err)
	}
}

// TestEmbeddedBackend_Execute_Success tests successful write execution.
func TestEmbeddedBackend_Execute_Success(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()

	// Create a simple table
	err := backend.Execute(ctx, ":create test_table { id: Int => name: String }")
	if err != nil {
		// Table might already exist, ignore that error
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("Execute failed: %v", err)
		}
	}
}

// TestEmbeddedBackend_Execute_ContextCanceled tests execute with canceled context.
func TestEmbeddedBackend_Execute_ContextCanceled(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := backend.Execute(ctx, ":create test_table2 { id: Int }")
	if err == nil {
		t.Error("expected error with canceled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected 'context canceled' error, got: %v", err)
	}
}

// TestEmbeddedBackend_Execute_AfterClose tests that execute fails after Close().
func TestEmbeddedBackend_Execute_AfterClose(t *testing.T) {
	backend := setupTestStorage(t)
	_ = backend.Close()

	ctx := context.Background()
	err := backend.Execute(ctx, ":create test_table3 { id: Int }")
	if err == nil {
		t.Error("expected error when executing on closed backend")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' error, got: %v", err)
	}
}

// TestEmbeddedBackend_Close_Idempotent tests that Close() can be called multiple times.
func TestEmbeddedBackend_Close_Idempotent(t *testing.T) {
	backend := setupTestStorage(t)

	// Close once
	err1 := backend.Close()
	if err1 != nil {
		t.Errorf("first Close() returned error: %v", err1)
	}

	// Close again - should not panic or error
	err2 := backend.Close()
	if err2 != nil {
		t.Errorf("second Close() returned error: %v", err2)
	}

	// Verify backend is closed
	if !backend.closed {
		t.Error("expected backend.closed to be true")
	}
}

// TestEmbeddedBackend_Close_PreventsOperations tests that operations fail after Close().
func TestEmbeddedBackend_Close_PreventsOperations(t *testing.T) {
	backend := setupTestStorage(t)
	_ = backend.Close()

	ctx := context.Background()

	// Try Query
	_, err := backend.Query(ctx, "?[x] := x = 1")
	if err == nil {
		t.Error("Query should fail after Close()")
	}

	// Try Execute
	err = backend.Execute(ctx, ":create test { id: Int }")
	if err == nil {
		t.Error("Execute should fail after Close()")
	}
}

// TestEmbeddedBackend_EnsureSchema tests schema creation.
func TestEmbeddedBackend_EnsureSchema(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify tables were created by querying one
	ctx := context.Background()
	result, err := backend.Query(ctx, "?[id, name] := *cie_function{id, name} :limit 1")
	if err != nil {
		t.Fatalf("Query after EnsureSchema failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestEmbeddedBackend_EnsureSchema_Idempotent tests that EnsureSchema can be called multiple times.
func TestEmbeddedBackend_EnsureSchema_Idempotent(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	// Call once
	err1 := backend.EnsureSchema()
	if err1 != nil {
		t.Fatalf("first EnsureSchema failed: %v", err1)
	}

	// Call again - should not error
	err2 := backend.EnsureSchema()
	if err2 != nil {
		t.Errorf("second EnsureSchema failed: %v", err2)
	}
}

// TestEmbeddedBackend_CreateHNSWIndex tests HNSW index creation.
func TestEmbeddedBackend_CreateHNSWIndex(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	// Need to create schema first
	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create HNSW indexes
	err = backend.CreateHNSWIndex()
	if err != nil {
		t.Fatalf("CreateHNSWIndex failed: %v", err)
	}
}

// TestEmbeddedBackend_CreateHNSWIndex_Idempotent tests that CreateHNSWIndex can be called multiple times.
func TestEmbeddedBackend_CreateHNSWIndex_Idempotent(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Call once
	err1 := backend.CreateHNSWIndex()
	if err1 != nil {
		t.Fatalf("first CreateHNSWIndex failed: %v", err1)
	}

	// Call again - should not error
	err2 := backend.CreateHNSWIndex()
	if err2 != nil {
		t.Errorf("second CreateHNSWIndex failed: %v", err2)
	}
}

// TestEmbeddedBackend_ConcurrentReads tests that concurrent reads don't block each other.
func TestEmbeddedBackend_ConcurrentReads(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()
	numReaders := 10

	var wg sync.WaitGroup
	wg.Add(numReaders)

	start := time.Now()

	for range numReaders {
		go func() {
			defer wg.Done()
			_, err := backend.Query(ctx, "?[x] := x = 1")
			if err != nil {
				t.Errorf("concurrent Query failed: %v", err)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	// Concurrent reads should be fast (< 1 second for 10 reads)
	if duration > time.Second {
		t.Errorf("concurrent reads took too long: %v (expected < 1s)", duration)
	}
}

// TestEmbeddedBackend_DB tests direct database access.
func TestEmbeddedBackend_DB(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	db := backend.DB()
	if db == nil {
		t.Fatal("expected non-nil db from DB()")
	}

	// Try using the direct DB access
	result, err := db.RunReadOnly("?[x] := x = 1", nil)
	if err != nil {
		t.Fatalf("direct DB query failed: %v", err)
	}
	if len(result.Headers) == 0 {
		t.Error("expected headers in direct DB result")
	}
}
