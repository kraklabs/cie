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

// Package cozodb provides a Go binding for CozoDB v0.7.6+.
//
// CozoDB is a Datalog-based embedded database designed for graph queries
// and complex data relationships. CIE uses it to store code intelligence
// data including functions, types, call graphs, and vector embeddings
// for semantic search.
//
// # Requirements
//
// This package requires CGO and the CozoDB C library (libcozo_c). Build with:
//
//	CGO_ENABLED=1 go build
//
// The CozoDB library must be installed on your system:
//
//	# macOS (Homebrew)
//	brew install cozodb
//
//	# Linux (from source or package manager)
//	# See https://github.com/cozodb/cozo for installation
//
// You may need to set library paths:
//
//	export CGO_LDFLAGS="-L/path/to/libcozo_c"
//	export CGO_CFLAGS="-I/path/to/cozo_c.h"
//
// # Storage Engines
//
// CozoDB supports multiple storage backends:
//   - "mem": In-memory, fast but not persisted (good for testing)
//   - "sqlite": SQLite-backed, single-file persistence
//   - "rocksdb": RocksDB-backed, best performance for production
//
// # Quick Start
//
// Open a database and run queries:
//
//	// Open with RocksDB storage
//	db, err := cozodb.New("rocksdb", "/path/to/data", nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
//	// Run a simple query
//	result, err := db.Run(`?[x] := x = 1 + 1`, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("1 + 1 = %v\n", result.Rows[0][0])
//
// # Read-Only Queries
//
// Use RunReadOnly for queries that should not modify data:
//
//	// This enforces read-only semantics at the database level
//	result, err := db.RunReadOnly(`?[name] := *cie_function{name}`, nil)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Parameterized Queries
//
// Pass parameters to prevent injection and improve readability:
//
//	params := map[string]any{
//	    "name": "main",
//	}
//	result, err := db.Run(`
//	    ?[signature, file_path] :=
//	        *cie_function{name, signature, file_path},
//	        name == $name
//	`, params)
//
// # Backup and Restore
//
// Create and restore database backups:
//
//	// Create backup
//	err := db.Backup("/path/to/backup.db")
//
//	// Restore from backup
//	err := db.Restore("/path/to/backup.db")
//
// # CIE Data Model
//
// CIE uses these main relations (tables) for code intelligence:
//
//	cie_file            - Indexed source files with metadata
//	cie_function        - Functions and methods (name, signature, location)
//	cie_function_code   - Function source code (separate for lazy loading)
//	cie_function_embedding - Vector embeddings for semantic search
//	cie_type            - Types, interfaces, classes
//	cie_type_code       - Type definitions source code
//	cie_type_embedding  - Type embeddings for semantic search
//	cie_calls           - Function call graph edges
//	cie_defines         - File defines function relationships
//	cie_import          - Import statements
//
// # Version Compatibility
//
// This binding targets CozoDB v0.7.6+ which includes the immutable_query
// parameter in the C API. Earlier versions may not work correctly with
// the RunReadOnly method.
package cozodb
