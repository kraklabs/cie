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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

	// Create HNSW indexes (768 = nomic-embed-text default)
	err = backend.CreateHNSWIndex(768)
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

	// Call once (768 = nomic-embed-text default)
	err1 := backend.CreateHNSWIndex(768)
	if err1 != nil {
		t.Fatalf("first CreateHNSWIndex failed: %v", err1)
	}

	// Call again - should not error
	err2 := backend.CreateHNSWIndex(768)
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

// TestEmbeddedBackend_ProjectMeta tests the project metadata storage.
func TestEmbeddedBackend_ProjectMeta(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Test GetProjectMeta with non-existent key returns empty string
	value, err := backend.GetProjectMeta("nonexistent")
	if err != nil {
		t.Fatalf("GetProjectMeta failed: %v", err)
	}
	if value != "" {
		t.Errorf("expected empty string for nonexistent key, got %q", value)
	}

	// Test SetProjectMeta
	err = backend.SetProjectMeta("test_key", "test_value")
	if err != nil {
		t.Fatalf("SetProjectMeta failed: %v", err)
	}

	// Test GetProjectMeta retrieves the value
	value, err = backend.GetProjectMeta("test_key")
	if err != nil {
		t.Fatalf("GetProjectMeta failed: %v", err)
	}
	if value != "test_value" {
		t.Errorf("expected 'test_value', got %q", value)
	}

	// Test SetProjectMeta overwrites existing value
	err = backend.SetProjectMeta("test_key", "new_value")
	if err != nil {
		t.Fatalf("SetProjectMeta overwrite failed: %v", err)
	}

	value, err = backend.GetProjectMeta("test_key")
	if err != nil {
		t.Fatalf("GetProjectMeta after overwrite failed: %v", err)
	}
	if value != "new_value" {
		t.Errorf("expected 'new_value', got %q", value)
	}
}

// TestEmbeddedBackend_LastIndexedSHA tests the SHA storage convenience methods.
func TestEmbeddedBackend_LastIndexedSHA(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Test GetLastIndexedSHA with no SHA set
	sha, err := backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("GetLastIndexedSHA failed: %v", err)
	}
	if sha != "" {
		t.Errorf("expected empty SHA initially, got %q", sha)
	}

	// Test SetLastIndexedSHA
	testSHA := "abc123def456"
	err = backend.SetLastIndexedSHA(testSHA)
	if err != nil {
		t.Fatalf("SetLastIndexedSHA failed: %v", err)
	}

	// Test GetLastIndexedSHA retrieves the SHA
	sha, err = backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("GetLastIndexedSHA failed: %v", err)
	}
	if sha != testSHA {
		t.Errorf("expected %q, got %q", testSHA, sha)
	}
}

// TestEmbeddedBackend_DeleteEntitiesForFile tests deletion of file entities.
func TestEmbeddedBackend_DeleteEntitiesForFile(t *testing.T) {
	backend := setupTestStorage(t)
	defer func() {
		_ = backend.Close()
	}()

	ctx := context.Background()

	err := backend.EnsureSchema()
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Insert test data for two files
	insertQueries := []string{
		// File 1: test.go
		`?[id, path, hash, language, size] <- [["file:test.go", "test.go", "hash1", "go", 100]] :put cie_file {id, path, hash, language, size}`,
		`?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [["func:TestFunc", "TestFunc", "func()", "test.go", 1, 10, 0, 0]] :put cie_function {id, name, signature, file_path, start_line, end_line, start_col, end_col}`,
		`?[function_id, code_text] <- [["func:TestFunc", "func TestFunc() {}"]] :put cie_function_code {function_id, code_text}`,
		`?[id, file_id, function_id] <- [["def:test.go:TestFunc", "file:test.go", "func:TestFunc"]] :put cie_defines {id, file_id, function_id}`,

		// File 2: other.go (should NOT be deleted)
		`?[id, path, hash, language, size] <- [["file:other.go", "other.go", "hash2", "go", 200]] :put cie_file {id, path, hash, language, size}`,
		`?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [["func:OtherFunc", "OtherFunc", "func()", "other.go", 1, 5, 0, 0]] :put cie_function {id, name, signature, file_path, start_line, end_line, start_col, end_col}`,
	}

	for _, query := range insertQueries {
		err := backend.Execute(ctx, query)
		if err != nil {
			t.Fatalf("insert query failed: %v\nQuery: %s", err, query)
		}
	}

	// Verify both files exist
	result, err := backend.Query(ctx, `?[path] := *cie_file{path}`)
	if err != nil {
		t.Fatalf("query files failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 files before delete, got %d", len(result.Rows))
	}

	// Verify both functions exist
	result, err = backend.Query(ctx, `?[name] := *cie_function{name}`)
	if err != nil {
		t.Fatalf("query functions failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 functions before delete, got %d", len(result.Rows))
	}

	// Delete entities for test.go
	err = backend.DeleteEntitiesForFile("test.go")
	if err != nil {
		t.Fatalf("DeleteEntitiesForFile failed: %v", err)
	}

	// Verify test.go file is deleted
	result, err = backend.Query(ctx, `?[path] := *cie_file{path}, path = "test.go"`)
	if err != nil {
		t.Fatalf("query test.go file failed: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected test.go file to be deleted, but found %d", len(result.Rows))
	}

	// Verify test.go function is deleted
	result, err = backend.Query(ctx, `?[name] := *cie_function{name, file_path}, file_path = "test.go"`)
	if err != nil {
		t.Fatalf("query test.go functions failed: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected test.go functions to be deleted, but found %d", len(result.Rows))
	}

	// Verify other.go is NOT deleted
	result, err = backend.Query(ctx, `?[path] := *cie_file{path}, path = "other.go"`)
	if err != nil {
		t.Fatalf("query other.go file failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected other.go file to still exist, but found %d", len(result.Rows))
	}

	// Verify other.go function is NOT deleted
	result, err = backend.Query(ctx, `?[name] := *cie_function{name, file_path}, file_path = "other.go"`)
	if err != nil {
		t.Fatalf("query other.go functions failed: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Errorf("expected other.go functions to still exist, but found %d", len(result.Rows))
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

// TestIsStaleLock_NoLockFile tests isStaleLock when LOCK file doesn't exist.
func TestIsStaleLock_NoLockFile(t *testing.T) {
	dir := t.TempDir()
	if isStaleLock(dir) {
		t.Error("expected false when LOCK file doesn't exist")
	}
}

// TestIsStaleLock_StaleLock tests isStaleLock with a stale LOCK file (no flock held).
func TestIsStaleLock_StaleLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "LOCK")

	// Create a LOCK file without holding flock
	f, err := os.Create(lockPath)
	if err != nil {
		t.Fatalf("failed to create LOCK file: %v", err)
	}
	f.Close()

	if !isStaleLock(dir) {
		t.Error("expected true for stale LOCK file (no process holding flock)")
	}
}

// TestIsStaleLock_ActiveLock tests isStaleLock with an active flock held.
func TestIsStaleLock_ActiveLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "LOCK")

	// Create and hold an exclusive flock
	f, err := os.Create(lockPath)
	if err != nil {
		t.Fatalf("failed to create LOCK file: %v", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("failed to acquire flock: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	}()

	if isStaleLock(dir) {
		t.Error("expected false when another process holds the flock")
	}
}
