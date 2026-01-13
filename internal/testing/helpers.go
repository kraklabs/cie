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

package testing

import (
	"context"
	"testing"

	"github.com/kraklabs/cie/pkg/storage"
)

// SetupTestBackend creates an in-memory CIE backend for testing.
// The backend is automatically cleaned up when the test finishes.
//
// This helper:
//   - Creates a temporary directory
//   - Initializes an in-memory CozoDB backend
//   - Ensures the CIE schema is created
//   - Registers cleanup to close the backend
//
// Example:
//
//	func TestMyFeature(t *testing.T) {
//	    backend := testing.SetupTestBackend(t)
//
//	    // Backend is ready with CIE schema initialized
//	    testing.InsertTestFunction(t, backend, "func1", "TestFunc", "test.go", 10, 20)
//
//	    // Run your tests...
//	}
func SetupTestBackend(t *testing.T) *storage.EmbeddedBackend {
	t.Helper()

	// Use in-memory engine for fast tests
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		Engine:  "mem",
		DataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	// Ensure schema is initialized
	if err := backend.EnsureSchema(); err != nil {
		t.Fatalf("failed to ensure schema: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		backend.Close()
	})

	return backend
}

// InsertTestFunction adds a test function to the database.
// This is a convenience helper for seeding test data.
//
// Example:
//
//	backend := testing.SetupTestBackend(t)
//	testing.InsertTestFunction(t, backend, "func_123", "HandleAuth", "auth.go", 10, 25)
func InsertTestFunction(t *testing.T, backend *storage.EmbeddedBackend, id, name, filePath string, startLine, endLine int) {
	t.Helper()

	query := `?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [[
		$id, $name, "", $file_path, $start_line, $end_line, 0, 0
	]]
	:put cie_function { id, name, signature, file_path, start_line, end_line, start_col, end_col }`

	ctx := context.Background()
	err := backend.Execute(ctx, query)

	if err != nil {
		// Try with params (CozoDB API may require this)
		db := backend.DB()
		_, err = db.Run(query, map[string]any{
			"id":         id,
			"name":       name,
			"file_path":  filePath,
			"start_line": startLine,
			"end_line":   endLine,
		})
		if err != nil {
			t.Fatalf("failed to insert test function: %v", err)
		}
	}
}

// InsertTestFunctionWithSignature adds a test function with a signature to the database.
// This is like InsertTestFunction but includes the full function signature.
//
// Example:
//
//	testing.InsertTestFunctionWithSignature(t, backend,
//	    "func_123", "HandleAuth", "func(r *http.Request) error", "auth.go", 10, 25)
func InsertTestFunctionWithSignature(t *testing.T, backend *storage.EmbeddedBackend, id, name, signature, filePath string, startLine, endLine int) {
	t.Helper()

	db := backend.DB()
	query := `?[id, name, signature, file_path, start_line, end_line, start_col, end_col] <- [[
		$id, $name, $signature, $file_path, $start_line, $end_line, 0, 0
	]]
	:put cie_function { id, name, signature, file_path, start_line, end_line, start_col, end_col }`

	_, err := db.Run(query, map[string]any{
		"id":         id,
		"name":       name,
		"signature":  signature,
		"file_path":  filePath,
		"start_line": startLine,
		"end_line":   endLine,
	})

	if err != nil {
		t.Fatalf("failed to insert test function with signature: %v", err)
	}
}

// InsertTestFile adds a test file to the database.
// This is a convenience helper for seeding test data.
//
// Example:
//
//	backend := testing.SetupTestBackend(t)
//	testing.InsertTestFile(t, backend, "file_123", "auth.go", "abc123", "go", 1234)
func InsertTestFile(t *testing.T, backend *storage.EmbeddedBackend, id, path, hash, language string, size int64) {
	t.Helper()

	db := backend.DB()
	query := `?[id, path, hash, language, size] <- [[
		$id, $path, $hash, $language, $size
	]]
	:put cie_file { id, path, hash, language, size }`

	_, err := db.Run(query, map[string]any{
		"id":       id,
		"path":     path,
		"hash":     hash,
		"language": language,
		"size":     size,
	})

	if err != nil {
		t.Fatalf("failed to insert test file: %v", err)
	}
}

// InsertTestType adds a test type (struct/interface/class) to the database.
// This is a convenience helper for seeding test data.
//
// Example:
//
//	backend := testing.SetupTestBackend(t)
//	testing.InsertTestType(t, backend, "type_123", "UserService", "struct", "user.go", 10, 50)
func InsertTestType(t *testing.T, backend *storage.EmbeddedBackend, id, name, kind, filePath string, startLine, endLine int) {
	t.Helper()

	db := backend.DB()
	query := `?[id, name, kind, file_path, start_line, end_line, start_col, end_col] <- [[
		$id, $name, $kind, $file_path, $start_line, $end_line, 0, 0
	]]
	:put cie_type { id, name, kind, file_path, start_line, end_line, start_col, end_col }`

	_, err := db.Run(query, map[string]any{
		"id":         id,
		"name":       name,
		"kind":       kind,
		"file_path":  filePath,
		"start_line": startLine,
		"end_line":   endLine,
	})

	if err != nil {
		t.Fatalf("failed to insert test type: %v", err)
	}
}

// InsertTestDefines adds a defines edge (file -> function) to the database.
// This links a file to a function it defines.
//
// Example:
//
//	testing.InsertTestDefines(t, backend, "def_123", "file_123", "func_123")
func InsertTestDefines(t *testing.T, backend *storage.EmbeddedBackend, id, fileID, functionID string) {
	t.Helper()

	db := backend.DB()
	query := `?[id, file_id, function_id] <- [[
		$id, $file_id, $function_id
	]]
	:put cie_defines { id, file_id, function_id }`

	_, err := db.Run(query, map[string]any{
		"id":          id,
		"file_id":     fileID,
		"function_id": functionID,
	})

	if err != nil {
		t.Fatalf("failed to insert defines edge: %v", err)
	}
}

// InsertTestCalls adds a calls edge (caller -> callee) to the database.
// This links a caller function to a callee function.
//
// Example:
//
//	testing.InsertTestCalls(t, backend, "call_123", "caller_func_id", "callee_func_id")
func InsertTestCalls(t *testing.T, backend *storage.EmbeddedBackend, id, callerID, calleeID string) {
	t.Helper()

	db := backend.DB()
	query := `?[id, caller_id, callee_id] <- [[
		$id, $caller_id, $callee_id
	]]
	:put cie_calls { id, caller_id, callee_id }`

	_, err := db.Run(query, map[string]any{
		"id":        id,
		"caller_id": callerID,
		"callee_id": calleeID,
	})

	if err != nil {
		t.Fatalf("failed to insert calls edge: %v", err)
	}
}

// InsertTestImport adds an import to the database.
// This records that a file imports a package.
//
// Example:
//
//	testing.InsertTestImport(t, backend, "import_123", "auth.go", "fmt", "", 1)
func InsertTestImport(t *testing.T, backend *storage.EmbeddedBackend, id, filePath, importPath, alias string, startLine int) {
	t.Helper()

	db := backend.DB()
	query := `?[id, file_path, import_path, alias, start_line] <- [[
		$id, $file_path, $import_path, $alias, $start_line
	]]
	:put cie_import { id, file_path, import_path, alias, start_line }`

	_, err := db.Run(query, map[string]any{
		"id":          id,
		"file_path":   filePath,
		"import_path": importPath,
		"alias":       alias,
		"start_line":  startLine,
	})

	if err != nil {
		t.Fatalf("failed to insert import: %v", err)
	}
}

// QueryFunctions is a helper to query all functions from the database.
// Returns rows with [id, name] columns.
//
// Example:
//
//	result := testing.QueryFunctions(t, backend)
//	require.Len(t, result.Rows, 2)
//	// Access: result.Rows[0][0] = id, result.Rows[0][1] = name
func QueryFunctions(t *testing.T, backend *storage.EmbeddedBackend) *storage.QueryResult {
	t.Helper()

	ctx := context.Background()
	result, err := backend.Query(ctx, "?[id, name] := *cie_function { id, name }")
	if err != nil {
		t.Fatalf("failed to query functions: %v", err)
	}

	return result
}

// QueryFiles is a helper to query all files from the database.
// Returns rows with [id, path] columns.
//
// Example:
//
//	result := testing.QueryFiles(t, backend)
//	require.Len(t, result.Rows, 1)
func QueryFiles(t *testing.T, backend *storage.EmbeddedBackend) *storage.QueryResult {
	t.Helper()

	ctx := context.Background()
	result, err := backend.Query(ctx, "?[id, path] := *cie_file { id, path }")
	if err != nil {
		t.Fatalf("failed to query files: %v", err)
	}

	return result
}

// QueryTypes is a helper to query all types from the database.
// Returns rows with [id, name, kind] columns.
//
// Example:
//
//	result := testing.QueryTypes(t, backend)
//	require.Len(t, result.Rows, 1)
func QueryTypes(t *testing.T, backend *storage.EmbeddedBackend) *storage.QueryResult {
	t.Helper()

	ctx := context.Background()
	result, err := backend.Query(ctx, "?[id, name, kind] := *cie_type { id, name, kind }")
	if err != nil {
		t.Fatalf("failed to query types: %v", err)
	}

	return result
}
