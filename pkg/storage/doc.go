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

// Package storage provides storage backend abstractions for CIE.
//
// This package defines the Backend interface that allows CIE tools to work
// with different storage implementations. The abstraction enables the same
// MCP tools to operate against either a local embedded database or a remote
// CIE Enterprise service.
//
// # Available Backends
//
// The package provides these backend implementations:
//
//   - EmbeddedBackend: Local CozoDB instance for standalone/open-source use
//   - Remote backends: Available in CIE Enterprise (not included in this package)
//
// # Quick Start
//
// Create an embedded backend and execute queries:
//
//	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
//	    DataDir:   "/path/to/data",
//	    Engine:    "rocksdb",
//	    ProjectID: "myproject",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer backend.Close()
//
//	// Initialize schema
//	if err := backend.EnsureSchema(); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Execute a query
//	result, err := backend.Query(ctx, `
//	    ?[name, file_path] := *cie_function{name, file_path}
//	    :limit 10
//	`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, row := range result.Rows {
//	    fmt.Printf("%s in %s\n", row[0], row[1])
//	}
//
// # Schema Initialization
//
// Before indexing code, initialize the CIE schema:
//
//	// Create all CIE tables (idempotent)
//	err := backend.EnsureSchema()
//
//	// Create HNSW indexes for semantic search
//	err := backend.CreateHNSWIndex()
//
// The schema includes tables for:
//   - Files and their metadata
//   - Functions with code and embeddings
//   - Types with code and embeddings
//   - Call graph relationships
//   - Import statements
//
// # Query vs Execute
//
// Use Query for read operations and Execute for mutations:
//
//	// Read-only query (uses RunReadOnly internally)
//	result, err := backend.Query(ctx, `?[count(f)] := *cie_function{id: f}`)
//
//	// Mutation (uses Run internally)
//	err := backend.Execute(ctx, `:rm cie_function { id: "fn123" }`)
//
// # Configuration
//
// EmbeddedConfig controls the backend behavior:
//
//	config := storage.EmbeddedConfig{
//	    DataDir:   "/path/to/data",  // Where to store CozoDB data
//	    Engine:    "rocksdb",        // Storage engine: mem, sqlite, rocksdb
//	    ProjectID: "myproject",      // Namespaces data directory
//	}
//
// Default values if not specified:
//   - DataDir: ~/.cie/data/<project_id>
//   - Engine: "rocksdb" (recommended for production)
//
// # Thread Safety
//
// EmbeddedBackend is safe for concurrent use. Read operations use a read
// lock while write operations use an exclusive lock, allowing concurrent
// reads but exclusive writes.
//
// # Direct Database Access
//
// For advanced operations, access the underlying CozoDB instance:
//
//	db := backend.DB()
//	result, err := db.Run(`::relations`, nil)  // List all relations
//
// Use with caution - prefer the Backend interface methods for normal operations.
package storage
