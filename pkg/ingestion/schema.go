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
package ingestion

// Schema defines the Datalog schema for CIE ingestion entities.
//
// Tables (v3 - vertically partitioned for performance):
//   - cie_file: File entities
//   - cie_function: Function metadata (lightweight, ~500 bytes/row)
//   - cie_function_code: Function code text (lazy loaded)
//   - cie_function_embedding: Function embeddings (for HNSW only)
//   - cie_type: Type metadata (lightweight)
//   - cie_type_code: Type code text (lazy loaded)
//   - cie_type_embedding: Type embeddings (for HNSW only)
//   - cie_defines: Edge from file to function
//   - cie_defines_type: Edge from file to type
//   - cie_calls: Edge from caller function to callee function
//   - cie_import: Import statements for cross-package call resolution
//
// All IDs are deterministic and stable across re-runs for idempotency.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// FileEntity represents a source file in the repository.
type FileEntity struct {
	ID       string // Deterministic: hash(file_path) or file_path itself
	Path     string // Relative path from repo root
	Hash     string // Content hash (SHA256) for change detection
	Language string // Detected language (go, python, javascript, etc.)
	Size     int64  // File size in bytes
}

// FunctionEntity represents a function/method extracted from code.
// Note: In the database, CodeText and Embedding are stored in separate tables
// (cie_function_code, cie_function_embedding) for query performance.
// The struct keeps all fields for use in the ingestion pipeline.
type FunctionEntity struct {
	ID        string    // Deterministic: hash(file_path + name + range) - signature excluded for stability
	Name      string    // Function name
	Signature string    // Full signature if available, else empty (metadata only, not used in ID)
	FilePath  string    // Path to containing file
	CodeText  string    // Raw code snippet (stored in cie_function_code)
	Embedding []float32 // Embedding vector (stored in cie_function_embedding)
	StartLine int       // Start line (1-indexed)
	EndLine   int       // End line (1-indexed)
	StartCol  int       // Start column (1-indexed)
	EndCol    int       // End column (1-indexed)
}

// DefinesEdge represents a "file defines function" relationship.
type DefinesEdge struct {
	FileID     string // Reference to FileEntity.ID
	FunctionID string // Reference to FunctionEntity.ID
}

// TypeEntity represents a type/interface/class/struct definition.
// This is language-agnostic and normalizes across:
//   - Go: struct, interface, type_alias
//   - Python: class
//   - TypeScript: interface, class, type_alias
//   - JavaScript: class
//
// Note: In the database, CodeText and Embedding are stored in separate tables
// (cie_type_code, cie_type_embedding) for query performance.
type TypeEntity struct {
	ID        string    // Deterministic: hash(file_path + name + range)
	Name      string    // Type name (e.g., "UserService", "Handler")
	Kind      string    // "struct", "interface", "class", "type_alias"
	FilePath  string    // Path to containing file
	CodeText  string    // Raw code snippet (stored in cie_type_code)
	Embedding []float32 // Embedding vector (stored in cie_type_embedding)
	StartLine int       // Start line (1-indexed)
	EndLine   int       // End line (1-indexed)
	StartCol  int       // Start column (1-indexed)
	EndCol    int       // End column (1-indexed)
}

// DefinesTypeEdge represents a "file defines type" relationship.
type DefinesTypeEdge struct {
	FileID string // Reference to FileEntity.ID
	TypeID string // Reference to TypeEntity.ID
}

// CallsEdge represents a "function calls function" relationship.
// Includes both same-file calls and cross-package calls (resolved via imports).
type CallsEdge struct {
	CallerID string // Reference to FunctionEntity.ID (caller)
	CalleeID string // Reference to FunctionEntity.ID (callee)
}

// ImportEntity represents an import statement in a source file.
type ImportEntity struct {
	ID         string // Deterministic: hash(file_path + import_path)
	FilePath   string // File that contains the import
	ImportPath string // Module/package being imported (e.g., "fmt", "github.com/org/pkg")
	Alias      string // Import alias: "" (default), "alias", "." (dot import), "_" (blank import)
	StartLine  int    // Line number of the import statement
}

// UnresolvedCall represents a function call that couldn't be resolved locally.
// These are collected during parsing and resolved later using import information.
type UnresolvedCall struct {
	CallerID   string // Reference to FunctionEntity.ID of the caller
	CalleeName string // Name of the called function (e.g., "foo" or "pkg.Foo")
	FilePath   string // File where the call occurs (for import resolution)
	Line       int    // Line number of the call
}

// PackageInfo represents a Go package with its files.
type PackageInfo struct {
	PackagePath string   // Directory path (e.g., "internal/http/handlers")
	PackageName string   // Package name from `package X` declaration
	Files       []string // Files that belong to this package
}

// DatalogSchema returns the Datalog schema definition for all ingestion tables.
// Schema v3: Vertically partitioned for performance on large datasets.
func DatalogSchema() string {
	return `
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
// code_text and embedding are stored in separate tables for performance
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
// Note: embedding uses CozoDB vector type <F32; 1536> for HNSW index support
// 1536 dimensions for Qodo-Embed-1-1.5B (768 for nomic-embed-text)
:create cie_function_embedding {
	function_id: String =>
	embedding: <F32; 1536>
}

// Defines edges: file -> function (file defines function)
:create cie_defines {
	file_id: String,
	function_id: String =>
}

// Calls edges: function -> function (caller calls callee)
:create cie_calls {
	caller_id: String,
	callee_id: String =>
}

// Import entities: represents import statements in source files
:create cie_import {
	id: String =>
	file_path: String,
	import_path: String,
	alias: String,
	start_line: Int
}

// Type entities: lightweight metadata
// code_text and embedding are stored in separate tables for performance
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

// Type code text: lazy loaded only when displaying source
:create cie_type_code {
	type_id: String =>
	code_text: String
}

// Type embeddings: used only for HNSW semantic search
// 1536 dimensions for Qodo-Embed-1-1.5B (768 for nomic-embed-text)
:create cie_type_embedding {
	type_id: String =>
	embedding: <F32; 1536>
}

// Defines type edges: file -> type (file defines type)
:create cie_defines_type {
	file_id: String,
	type_id: String =>
}
`
}

// GenerateImportID generates a deterministic ID for an import entity.
func GenerateImportID(filePath, importPath string) string {
	h := sha256.New()
	h.Write([]byte(filePath))
	h.Write([]byte("|"))
	h.Write([]byte(importPath))
	return "imp:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// GenerateTypeID generates a deterministic ID for a type entity.
func GenerateTypeID(filePath, name string, startLine, endLine int) string {
	h := sha256.New()
	h.Write([]byte(filePath))
	h.Write([]byte("|"))
	h.Write([]byte(name))
	h.Write([]byte("|"))
	h.Write([]byte(fmt.Sprintf("%d-%d", startLine, endLine)))
	return "typ:" + hex.EncodeToString(h.Sum(nil))[:16]
}
