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

// Package testing provides test helpers for CIE integration tests.
//
// This package wraps the root-level testcontainer infrastructure
// (located at /internal/testing/cozodb/) with CIE-specific schema
// setup and data seeding utilities.
//
// # Quick Start
//
// Use SetupTestBackend to create an in-memory CIE backend with schema:
//
//	func TestMyFeature(t *testing.T) {
//	    backend := testing.SetupTestBackend(t)
//
//	    // Backend is ready with CIE schema initialized
//	    testing.InsertTestFunction(t, backend, "func1", "TestFunc", "test.go", 10, 20)
//
//	    // Query and verify
//	    funcs := testing.QueryFunctions(t, backend)
//	    require.Len(t, funcs, 1)
//	}
//
// # Seeding Test Data
//
// The package provides helpers for inserting common test entities:
//   - InsertTestFunction: Add a function to the database
//   - InsertTestFile: Add a file to the database
//   - InsertTestType: Add a type (struct/interface) to the database
//   - InsertTestDefines: Link a file to a function
//   - InsertTestCalls: Link caller to callee
//   - InsertTestImport: Record an import statement
//
// # Querying Test Data
//
// Helper functions for common queries:
//   - QueryFunctions: Get all functions
//   - QueryFiles: Get all files
//   - QueryTypes: Get all types
//
// # Integration with Root Testcontainers
//
// For tests that require Docker/testcontainers, use the root-level
// infrastructure at /internal/testing/cozodb/:
//
//	//go:build cozodb
//	// +build cozodb
//
//	package mypackage
//
//	import (
//	    cozodbtest "github.com/kraklabs/kraken/internal/testing/cozodb"
//	    cietest "github.com/kraklabs/cie/internal/testing"
//	)
//
//	func TestIntegration(t *testing.T) {
//	    cozodbtest.RequireCozoDB(t) // Skip if CozoDB unavailable
//
//	    backend := cietest.SetupTestBackend(t)
//	    // Test with real CozoDB...
//	}
package testing
