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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupTestBackend verifies the test backend is created correctly.
func TestSetupTestBackend(t *testing.T) {
	backend := SetupTestBackend(t)

	// Backend should not be nil
	require.NotNil(t, backend)

	// Should be able to query (schema should exist)
	result := QueryFunctions(t, backend)
	require.NotNil(t, result)
	assert.Empty(t, result.Rows, "Should start with no functions")
}

// TestInsertTestFunction verifies function insertion.
func TestInsertTestFunction(t *testing.T) {
	backend := SetupTestBackend(t)

	// Insert a test function
	InsertTestFunction(t, backend, "func_123", "HandleAuth", "auth.go", 10, 25)

	// Verify it was inserted
	result := QueryFunctions(t, backend)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "func_123", result.Rows[0][0])
	assert.Equal(t, "HandleAuth", result.Rows[0][1])
}

// TestInsertTestFile verifies file insertion.
func TestInsertTestFile(t *testing.T) {
	backend := SetupTestBackend(t)

	// Insert a test file
	InsertTestFile(t, backend, "file_123", "auth.go", "abc123", "go", 1234)

	// Verify it was inserted
	result := QueryFiles(t, backend)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "file_123", result.Rows[0][0])
	assert.Equal(t, "auth.go", result.Rows[0][1])
}

// TestInsertTestType verifies type insertion.
func TestInsertTestType(t *testing.T) {
	backend := SetupTestBackend(t)

	// Insert a test type
	InsertTestType(t, backend, "type_123", "UserService", "struct", "user.go", 10, 50)

	// Verify it was inserted
	result := QueryTypes(t, backend)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "type_123", result.Rows[0][0])
	assert.Equal(t, "UserService", result.Rows[0][1])
	assert.Equal(t, "struct", result.Rows[0][2])
}

// TestMultipleInserts verifies multiple entities can be inserted.
func TestMultipleInserts(t *testing.T) {
	backend := SetupTestBackend(t)

	// Insert multiple functions
	InsertTestFunction(t, backend, "func1", "Main", "main.go", 5, 10)
	InsertTestFunction(t, backend, "func2", "Helper", "util.go", 15, 20)
	InsertTestFunction(t, backend, "func3", "Process", "processor.go", 25, 35)

	// Verify all were inserted
	result := QueryFunctions(t, backend)
	require.Len(t, result.Rows, 3)
}

// TestEdgeInsertion verifies relationship edges can be inserted.
func TestEdgeInsertion(t *testing.T) {
	backend := SetupTestBackend(t)

	// Setup entities
	InsertTestFile(t, backend, "file1", "main.go", "hash1", "go", 100)
	InsertTestFunction(t, backend, "func1", "main", "main.go", 1, 10)
	InsertTestFunction(t, backend, "func2", "helper", "main.go", 12, 15)

	// Insert edges
	InsertTestDefines(t, backend, "def1", "file1", "func1")
	InsertTestCalls(t, backend, "call1", "func1", "func2")

	// No direct query for edges yet, but should not error
	// Future: Add QueryDefines/QueryCalls helpers
}

// TestBackendIsolation verifies each test gets isolated backend.
func TestBackendIsolation(t *testing.T) {
	// Create first backend and add data
	backend1 := SetupTestBackend(t)
	InsertTestFunction(t, backend1, "func1", "Test1", "file1.go", 1, 10)

	// Create second backend - should be empty
	backend2 := SetupTestBackend(t)
	result := QueryFunctions(t, backend2)
	assert.Empty(t, result.Rows, "Second backend should be isolated from first")

	// Verify first backend still has data
	result1 := QueryFunctions(t, backend1)
	assert.Len(t, result1.Rows, 1)
}
