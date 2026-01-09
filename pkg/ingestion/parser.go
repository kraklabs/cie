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

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"log/slog"
)

// Parser extracts functions and relationships from source code files.
//
// Current Implementation:
// This parser uses simplified pattern matching (regex/string matching) for function extraction.
// It handles basic cases but has limitations:
//   - Functions nested inside structs/interfaces may not be extracted correctly
//   - Complex signatures with generics may be incomplete
//   - Call graph extraction is not implemented (calls edges are empty)
//
// Future Improvement:
// Tree-sitter integration is planned for more accurate AST-based parsing.
// This would provide:
//   - Precise function extraction with correct ranges
//   - Complete signature extraction including generics
//   - Call graph extraction (same-file and cross-file)
//   - Better handling of edge cases (nested functions, closures, etc.)
//
// Note: Tree-sitter requires CGO and additional setup, so it's deferred to a future iteration.
type Parser struct {
	logger          *slog.Logger
	maxCodeTextSize int64
	truncatedCount  int // Count of truncated CodeTexts (for summary)
}

// NewParser creates a new code parser.
func NewParser(logger *slog.Logger) *Parser {
	if logger == nil {
		logger = slog.Default()
	}
	return &Parser{
		logger:          logger,
		maxCodeTextSize: 102400, // Default 100KB
		truncatedCount:  0,
	}
}

// SetMaxCodeTextSize sets the maximum size for CodeText (in bytes).
func (p *Parser) SetMaxCodeTextSize(size int64) {
	p.maxCodeTextSize = size
}

// GetTruncatedCount returns the number of CodeTexts that were truncated.
func (p *Parser) GetTruncatedCount() int {
	return p.truncatedCount
}

// ResetTruncatedCount resets the truncation counter.
func (p *Parser) ResetTruncatedCount() {
	p.truncatedCount = 0
}

// truncateCodeText truncates CodeText if it exceeds the limit and increments counter.
func (p *Parser) truncateCodeText(codeText string) string {
	if p.maxCodeTextSize > 0 && int64(len(codeText)) > p.maxCodeTextSize {
		p.truncatedCount++
		return codeText[:p.maxCodeTextSize]
	}
	return codeText
}

// ParseResult contains extracted entities from a file.
type ParseResult struct {
	File            FileEntity
	Functions       []FunctionEntity
	Types           []TypeEntity // Types/interfaces/classes/structs
	Defines         []DefinesEdge
	DefinesTypes    []DefinesTypeEdge // File -> Type edges
	Calls           []CallsEdge
	Imports         []ImportEntity   // Go imports for cross-package resolution
	UnresolvedCalls []UnresolvedCall // Calls that couldn't be resolved in same file
	PackageName     string           // Go package name (e.g., "handlers")
}

// ParseFile parses a source file and extracts functions.
// Uses simplified pattern matching with fallback for unsupported languages.
// See Parser struct documentation for limitations and future improvements.
func (p *Parser) ParseFile(fileInfo FileInfo) (*ParseResult, error) {
	// Read file content
	content, err := os.ReadFile(fileInfo.FullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	// Compute content hash
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Create file entity
	fileID := GenerateFileID(fileInfo.Path)
	fileEntity := FileEntity{
		ID:       fileID,
		Path:     fileInfo.Path,
		Hash:     hashStr,
		Language: fileInfo.Language,
		Size:     fileInfo.Size,
	}

	// Extract functions based on language
	var functions []FunctionEntity
	var calls []CallsEdge

	switch fileInfo.Language {
	case "go":
		functions, calls = p.parseGoFile(string(content), fileInfo.Path)
	case "python":
		functions, calls = p.parsePythonFile(string(content), fileInfo.Path)
	case "javascript", "typescript":
		functions, calls = p.parseJSFile(string(content), fileInfo.Path)
	case "protobuf":
		functions, calls = parseProtobufContent(string(content), fileInfo.Path, p.truncateCodeText)
	default:
		// For unsupported languages, return empty result
		p.logger.Debug("parser.skip_unsupported_language",
			"path", fileInfo.Path,
			"language", fileInfo.Language,
		)
	}

	// Create defines edges
	defines := make([]DefinesEdge, len(functions))
	for i, fn := range functions {
		defines[i] = DefinesEdge{
			FileID:     fileID,
			FunctionID: fn.ID,
		}
	}

	return &ParseResult{
		File:      fileEntity,
		Functions: functions,
		Defines:   defines,
		Calls:     calls,
	}, nil
}
