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

package ingestion

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestIncrementalIndexing_Integration tests the full incremental indexing flow:
// 1. Create a git repo with initial files
// 2. Run full indexing
// 3. Modify files and commit
// 4. Run incremental indexing
// 5. Verify only changed files were processed
func TestIncrementalIndexing_Integration(t *testing.T) {
	// Create a temporary directory for the test repo
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "testrepo")
	dataDir := filepath.Join(testDir, "data")

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")

	// Create initial files
	writeFile(t, filepath.Join(repoDir, "main.go"), `package main

func main() {
	Hello()
	Greet("world")
}
`)

	writeFile(t, filepath.Join(repoDir, "hello.go"), `package main

import "fmt"

func Hello() {
	fmt.Println("Hello!")
}

func Greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}
`)

	writeFile(t, filepath.Join(repoDir, "utils.go"), `package main

func Add(a, b int) int {
	return a + b
}

func Multiply(a, b int) int {
	return a * b
}
`)

	// Initial commit
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Initial commit")

	// Create pipeline configuration
	cfg := Config{
		ProjectID:  "test-incremental",
		RepoSource: RepoSource{Type: "local_path", Value: repoDir},
		IngestionConfig: IngestionConfig{
			LocalDataDir:        dataDir,
			LocalEngine:         "mem",
			EmbeddingProvider:   "mock",
			EmbeddingDimensions: 384,
			MaxFileSizeBytes:    1048576,
			ExcludeGlobs:        []string{".git/**"},
			ForceReindex:        false, // Enable incremental indexing
			UseGitDelta:         true,  // Используем git для детекции изменений (это git-репозиторий)
			Concurrency: ConcurrencyConfig{
				ParseWorkers: 2,
				EmbedWorkers: 2,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// ========================================
	// Step 1: Run full indexing (first time)
	// ========================================
	t.Log("Running full indexing (first time)...")
	pipeline, err := NewLocalPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}
	defer pipeline.Close()

	ctx := context.Background()
	result1, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("first indexing run failed: %v", err)
	}

	t.Logf("First run: %d files, %d functions", result1.FilesProcessed, result1.FunctionsExtracted)

	// Verify initial indexing results
	if result1.FilesProcessed != 3 {
		t.Errorf("expected 3 files processed in first run, got %d", result1.FilesProcessed)
	}
	// main.go has main, hello.go has Hello and Greet, utils.go has Add and Multiply = 5 functions
	if result1.FunctionsExtracted != 5 {
		t.Errorf("expected exactly 5 functions extracted (main, Hello, Greet, Add, Multiply), got %d", result1.FunctionsExtracted)
	}

	// Verify functions are in the database
	allFuncsQuery := `?[name, file_path] := *cie_function{name, file_path}`
	allFuncsResult, err := pipeline.backend.Query(ctx, allFuncsQuery)
	if err != nil {
		t.Fatalf("failed to query all functions: %v", err)
	}
	if len(allFuncsResult.Rows) != 5 {
		t.Errorf("expected 5 functions in database after first run, got %d", len(allFuncsResult.Rows))
	}

	// Verify last indexed SHA was saved
	lastSHA, err := pipeline.backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("failed to get last indexed SHA: %v", err)
	}
	if lastSHA == "" {
		t.Fatal("expected last indexed SHA to be set after first run")
	}
	t.Logf("Saved SHA after first run: %s", lastSHA[:8])

	// Note: We keep the pipeline open to preserve the in-memory database
	// In production with RocksDB, the data would persist between runs

	// ========================================
	// Step 2: Modify repository
	// ========================================
	t.Log("Modifying repository...")

	// Add a new file
	writeFile(t, filepath.Join(repoDir, "new_file.go"), `package main

func NewFunction() string {
	return "I'm new!"
}
`)

	// Modify an existing file
	writeFile(t, filepath.Join(repoDir, "hello.go"), `package main

import "fmt"

func Hello() {
	fmt.Println("Hello, World!")
}

func Greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}

func Goodbye() {
	fmt.Println("Goodbye!")
}
`)

	// Delete a file
	os.Remove(filepath.Join(repoDir, "utils.go"))

	// Commit changes
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Modify files")

	// ========================================
	// Step 3: Run incremental indexing
	// ========================================
	t.Log("Running incremental indexing...")
	// Reuse the same pipeline to preserve in-memory database state
	// (In production with RocksDB, you could create a new pipeline)
	result2, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("second indexing run failed: %v", err)
	}

	t.Logf("Second run: %d files, %d functions", result2.FilesProcessed, result2.FunctionsExtracted)

	// ========================================
	// Step 4: Verify incremental behavior
	// ========================================

	// Incremental run should process fewer files than the first run
	// We added 1 file and modified 1 file = 2 files processed
	// We deleted 1 file (utils.go) which doesn't count as "processed"
	if result2.FilesProcessed > result1.FilesProcessed {
		t.Errorf("incremental run should process fewer or equal files: first=%d, second=%d",
			result1.FilesProcessed, result2.FilesProcessed)
	}

	if result2.FilesProcessed != 2 {
		t.Errorf("expected 2 files processed in incremental run (new_file.go + hello.go), got %d",
			result2.FilesProcessed)
	}

	// Verify the new SHA was saved
	newSHA, err := pipeline.backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("failed to get new last indexed SHA: %v", err)
	}
	if newSHA == lastSHA {
		t.Error("expected last indexed SHA to be updated after incremental run")
	}
	t.Logf("Saved SHA after second run: %s", newSHA[:8])

	// ========================================
	// Step 5: Verify database state
	// ========================================

	// Verify utils.go functions were deleted
	utilsQuery := `?[name] := *cie_function{name, file_path}, file_path = "utils.go"`
	utilsResult, err := pipeline.backend.Query(ctx, utilsQuery)
	if err != nil {
		t.Fatalf("failed to query for utils.go functions: %v", err)
	}
	if len(utilsResult.Rows) != 0 {
		t.Errorf("expected 0 functions from deleted utils.go, got %d", len(utilsResult.Rows))
	}

	// Verify new_file.go functions exist
	newFileQuery := `?[name] := *cie_function{name, file_path}, file_path = "new_file.go"`
	newFileResult, err := pipeline.backend.Query(ctx, newFileQuery)
	if err != nil {
		t.Fatalf("failed to query for new_file.go functions: %v", err)
	}
	if len(newFileResult.Rows) != 1 {
		t.Errorf("expected 1 function from new_file.go, got %d", len(newFileResult.Rows))
	}

	// Verify hello.go has the new Goodbye function
	helloQuery := `?[name] := *cie_function{name, file_path}, file_path = "hello.go"`
	helloResult, err := pipeline.backend.Query(ctx, helloQuery)
	if err != nil {
		t.Fatalf("failed to query for hello.go functions: %v", err)
	}
	// Should have Hello, Greet, Goodbye = 3 functions
	if len(helloResult.Rows) != 3 {
		t.Errorf("expected 3 functions from hello.go (Hello, Greet, Goodbye), got %d", len(helloResult.Rows))
	}

	// Verify main.go (unchanged) still has its function
	mainQuery := `?[name] := *cie_function{name, file_path}, file_path = "main.go"`
	mainResult, err := pipeline.backend.Query(ctx, mainQuery)
	if err != nil {
		t.Fatalf("failed to query for main.go functions: %v", err)
	}
	if len(mainResult.Rows) != 1 {
		t.Errorf("expected 1 function from main.go (main), got %d", len(mainResult.Rows))
	}

	// Verify total function count in database after incremental run
	// main.go: main (1) + hello.go: Hello, Greet, Goodbye (3) + new_file.go: NewFunction (1) = 5
	allFuncsQuery = `?[name, file_path] := *cie_function{name, file_path}`
	allFuncsResult, err = pipeline.backend.Query(ctx, allFuncsQuery)
	if err != nil {
		t.Fatalf("failed to query all functions after incremental: %v", err)
	}
	if len(allFuncsResult.Rows) != 5 {
		t.Errorf("expected 5 functions in database after incremental (main + Hello + Greet + Goodbye + NewFunction), got %d", len(allFuncsResult.Rows))
		for _, row := range allFuncsResult.Rows {
			t.Logf("  function: %v", row)
		}
	}

	t.Log("Incremental indexing test passed!")
}

// TestIncrementalIndexing_NoChanges tests that when there are no changes,
// the indexer returns immediately without reprocessing.
func TestIncrementalIndexing_NoChanges(t *testing.T) {
	// Create a temporary directory for the test repo
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "testrepo")
	dataDir := filepath.Join(testDir, "data")

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")

	// Create initial files
	writeFile(t, filepath.Join(repoDir, "main.go"), `package main

func main() {
	println("hello")
}
`)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Initial commit")

	cfg := Config{
		ProjectID:  "test-no-changes",
		RepoSource: RepoSource{Type: "local_path", Value: repoDir},
		IngestionConfig: IngestionConfig{
			LocalDataDir:        dataDir,
			LocalEngine:         "mem",
			EmbeddingProvider:   "mock",
			EmbeddingDimensions: 384,
			MaxFileSizeBytes:    1048576,
			ExcludeGlobs:        []string{".git/**"},
			ForceReindex:        false,
			UseGitDelta:         true, // Используем git для детекции изменений
			Concurrency: ConcurrencyConfig{
				ParseWorkers: 2,
				EmbedWorkers: 2,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	// First run
	pipeline, err := NewLocalPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	result1, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	t.Logf("First run: %d files", result1.FilesProcessed)
	// Keep pipeline open to preserve in-memory database
	defer pipeline.Close()

	// Verify first run processed files
	if result1.FilesProcessed != 1 {
		t.Errorf("expected 1 file processed in first run, got %d", result1.FilesProcessed)
	}

	// Get SHA after first run
	sha1, err := pipeline.backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("failed to get SHA after first run: %v", err)
	}
	if sha1 == "" {
		t.Fatal("expected SHA to be set after first run")
	}

	// Second run without any changes (reuse same pipeline)
	result2, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// With no changes, should process 0 files
	if result2.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed when no changes, got %d", result2.FilesProcessed)
	}

	// SHA should remain the same
	sha2, err := pipeline.backend.GetLastIndexedSHA()
	if err != nil {
		t.Fatalf("failed to get SHA after second run: %v", err)
	}
	if sha1 != sha2 {
		t.Errorf("expected SHA to remain %s, but got %s", sha1, sha2)
	}

	// Verify functions are still in database
	funcQuery := `?[name] := *cie_function{name}`
	funcResult, err := pipeline.backend.Query(ctx, funcQuery)
	if err != nil {
		t.Fatalf("failed to query functions: %v", err)
	}
	if len(funcResult.Rows) != 1 {
		t.Errorf("expected 1 function in database, got %d", len(funcResult.Rows))
	}

	t.Log("No-changes test passed!")
}

// TestIncrementalIndexing_ForceReindex tests that ForceReindex=true forces full indexing.
func TestIncrementalIndexing_ForceReindex(t *testing.T) {
	// Create a temporary directory for the test repo
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "testrepo")
	dataDir := filepath.Join(testDir, "data")

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")

	// Create initial files
	writeFile(t, filepath.Join(repoDir, "main.go"), `package main

func main() {
	println("hello")
}

func Helper() {
	println("helper")
}
`)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Initial commit")

	cfg := Config{
		ProjectID:  "test-force-reindex",
		RepoSource: RepoSource{Type: "local_path", Value: repoDir},
		IngestionConfig: IngestionConfig{
			LocalDataDir:        dataDir,
			LocalEngine:         "mem",
			EmbeddingProvider:   "mock",
			EmbeddingDimensions: 384,
			MaxFileSizeBytes:    1048576,
			ExcludeGlobs:        []string{".git/**"},
			ForceReindex:        false,
			UseGitDelta:         true, // Используем git для детекции изменений
			Concurrency: ConcurrencyConfig{
				ParseWorkers: 2,
				EmbedWorkers: 2,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	// First run
	pipeline, err := NewLocalPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	result1, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	t.Logf("First run: %d files", result1.FilesProcessed)
	// Don't close pipeline - update config and run again with ForceReindex
	defer pipeline.Close()

	// Update config for second run (though ForceReindex is checked at Run time)
	pipeline.config.IngestionConfig.ForceReindex = true

	result2, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// With ForceReindex, should process all files again
	if result2.FilesProcessed != result1.FilesProcessed {
		t.Errorf("expected full reindex to process %d files, got %d",
			result1.FilesProcessed, result2.FilesProcessed)
	}

	t.Log("Force reindex test passed!")
}

// TestIncrementalIndexing_NonGitRepo tests that non-git repos fall back to full indexing.
func TestIncrementalIndexing_NonGitRepo(t *testing.T) {
	// Create a temporary directory (NOT a git repo)
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "testrepo")
	dataDir := filepath.Join(testDir, "data")

	// Create the repo directory
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Create files but DON'T initialize git
	writeFile(t, filepath.Join(repoDir, "main.go"), `package main

func main() {
	println("hello")
}
`)

	cfg := Config{
		ProjectID:  "test-non-git",
		RepoSource: RepoSource{Type: "local_path", Value: repoDir},
		IngestionConfig: IngestionConfig{
			LocalDataDir:        dataDir,
			LocalEngine:         "mem",
			EmbeddingProvider:   "mock",
			EmbeddingDimensions: 384,
			MaxFileSizeBytes:    1048576,
			ExcludeGlobs:        []string{".git/**"},
			ForceReindex:        false, // Try incremental
			UseGitDelta:         false, // Non-git repo - use hash-based detection
			Concurrency: ConcurrencyConfig{
				ParseWorkers: 2,
				EmbedWorkers: 2,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	// First run - should do full indexing since not a git repo
	pipeline, err := NewLocalPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}
	defer pipeline.Close()

	result1, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	t.Logf("First run: %d files", result1.FilesProcessed)

	if result1.FilesProcessed != 1 {
		t.Errorf("expected 1 file processed, got %d", result1.FilesProcessed)
	}

	// Second run - hash-based detection should find no changes (файлы не менялись)
	result2, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// С hash-based детекцией, если файлы не менялись — изменений 0
	if result2.FilesProcessed != 0 {
		t.Errorf("expected 0 files (no changes detected by hash), got %d", result2.FilesProcessed)
	}

	t.Log("Non-git repo with hash-based detection test passed!")
}

// TestIncrementalIndexing_RenamedFile tests that renamed files are handled correctly.
func TestIncrementalIndexing_RenamedFile(t *testing.T) {
	testDir := t.TempDir()
	repoDir := filepath.Join(testDir, "testrepo")
	dataDir := filepath.Join(testDir, "data")

	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test User")

	// Create initial file
	writeFile(t, filepath.Join(repoDir, "old_name.go"), `package main

func OldFunction() {
	println("old")
}
`)

	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "Initial commit")

	cfg := Config{
		ProjectID:  "test-rename",
		RepoSource: RepoSource{Type: "local_path", Value: repoDir},
		IngestionConfig: IngestionConfig{
			LocalDataDir:        dataDir,
			LocalEngine:         "mem",
			EmbeddingProvider:   "mock",
			EmbeddingDimensions: 384,
			MaxFileSizeBytes:    1048576,
			ExcludeGlobs:        []string{".git/**"},
			ForceReindex:        false,
			UseGitDelta:         true, // Используем git для детекции изменений
			Concurrency: ConcurrencyConfig{
				ParseWorkers: 2,
				EmbedWorkers: 2,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	// First run
	pipeline, err := NewLocalPipeline(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}
	defer pipeline.Close()

	result1, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	t.Logf("First run: %d files, %d functions", result1.FilesProcessed, result1.FunctionsExtracted)

	// Verify old_name.go function exists
	oldQuery := `?[name] := *cie_function{name, file_path}, file_path = "old_name.go"`
	oldResult, err := pipeline.backend.Query(ctx, oldQuery)
	if err != nil {
		t.Fatalf("failed to query old_name.go: %v", err)
	}
	if len(oldResult.Rows) != 1 {
		t.Errorf("expected 1 function from old_name.go, got %d", len(oldResult.Rows))
	}

	// Rename the file using git mv
	runGit(t, repoDir, "mv", "old_name.go", "new_name.go")
	runGit(t, repoDir, "commit", "-m", "Rename file")

	// Second run - incremental with rename
	result2, err := pipeline.Run(ctx)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	t.Logf("Second run: %d files processed", result2.FilesProcessed)

	// Renamed file should be processed (old deleted, new added)
	if result2.FilesProcessed != 1 {
		t.Errorf("expected 1 file processed (renamed file), got %d", result2.FilesProcessed)
	}

	// Verify old_name.go functions are deleted
	oldResult, err = pipeline.backend.Query(ctx, oldQuery)
	if err != nil {
		t.Fatalf("failed to query old_name.go after rename: %v", err)
	}
	if len(oldResult.Rows) != 0 {
		t.Errorf("expected 0 functions from old_name.go after rename, got %d", len(oldResult.Rows))
	}

	// Verify new_name.go functions exist
	newQuery := `?[name] := *cie_function{name, file_path}, file_path = "new_name.go"`
	newResult, err := pipeline.backend.Query(ctx, newQuery)
	if err != nil {
		t.Fatalf("failed to query new_name.go: %v", err)
	}
	if len(newResult.Rows) != 1 {
		t.Errorf("expected 1 function from new_name.go, got %d", len(newResult.Rows))
	}

	t.Log("Renamed file test passed!")
}

// runGit executes a git command in the specified directory.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

// writeFile writes content to a file, creating parent directories if needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
