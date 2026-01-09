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
	"sync"

	"log/slog"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterParser uses Tree-sitter for accurate AST-based code parsing.
// This provides:
//   - Precise function extraction with correct ranges
//   - Complete signature extraction including generics
//   - Call graph extraction (same-file)
//   - Proper handling of nested functions, closures, methods
//
// Supported languages: Go, Python, JavaScript, TypeScript
type TreeSitterParser struct {
	logger          *slog.Logger
	maxCodeTextSize int64
	truncatedCount  int
	mu              sync.Mutex // Protects truncatedCount

	// Language parsers (lazily initialized, thread-safe via sitter)
	goParser   *sitter.Parser
	pyParser   *sitter.Parser
	jsParser   *sitter.Parser
	tsParser   *sitter.Parser
	parserInit sync.Once
}

// NewTreeSitterParser creates a new Tree-sitter based parser.
func NewTreeSitterParser(logger *slog.Logger) *TreeSitterParser {
	if logger == nil {
		logger = slog.Default()
	}
	return &TreeSitterParser{
		logger:          logger,
		maxCodeTextSize: 102400, // Default 100KB
	}
}

// initParsers initializes all language parsers (called once, lazily).
func (p *TreeSitterParser) initParsers() {
	p.parserInit.Do(func() {
		// Go parser
		p.goParser = sitter.NewParser()
		p.goParser.SetLanguage(golang.GetLanguage())

		// Python parser
		p.pyParser = sitter.NewParser()
		p.pyParser.SetLanguage(python.GetLanguage())

		// JavaScript parser
		p.jsParser = sitter.NewParser()
		p.jsParser.SetLanguage(javascript.GetLanguage())

		// TypeScript parser
		p.tsParser = sitter.NewParser()
		p.tsParser.SetLanguage(typescript.GetLanguage())
	})
}

// SetMaxCodeTextSize sets the maximum size for CodeText (in bytes).
func (p *TreeSitterParser) SetMaxCodeTextSize(size int64) {
	p.maxCodeTextSize = size
}

// GetTruncatedCount returns the number of CodeTexts that were truncated.
func (p *TreeSitterParser) GetTruncatedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.truncatedCount
}

// ResetTruncatedCount resets the truncation counter.
func (p *TreeSitterParser) ResetTruncatedCount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.truncatedCount = 0
}

// truncateCodeText truncates CodeText if it exceeds the limit.
func (p *TreeSitterParser) truncateCodeText(codeText string) string {
	if p.maxCodeTextSize > 0 && int64(len(codeText)) > p.maxCodeTextSize {
		p.mu.Lock()
		p.truncatedCount++
		p.mu.Unlock()
		return codeText[:p.maxCodeTextSize]
	}
	return codeText
}

// ParseFile parses a source file and extracts functions using Tree-sitter.
func (p *TreeSitterParser) ParseFile(fileInfo FileInfo) (*ParseResult, error) {
	p.initParsers()

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

	// Parse with appropriate language parser
	var functions []FunctionEntity
	var types []TypeEntity
	var calls []CallsEdge
	var imports []ImportEntity
	var unresolvedCalls []UnresolvedCall
	var packageName string

	switch fileInfo.Language {
	case "go":
		goResult, goErr := p.parseGoAST(content, fileInfo.Path)
		if goErr != nil {
			return nil, fmt.Errorf("parse go AST: %w", goErr)
		}
		functions = goResult.Functions
		types = goResult.Types
		calls = goResult.Calls
		imports = goResult.Imports
		unresolvedCalls = goResult.UnresolvedCalls
		packageName = goResult.PackageName
	case "python":
		functions, types, calls, err = p.parsePythonAST(content, fileInfo.Path)
	case "javascript":
		functions, types, calls, err = p.parseJavaScriptAST(content, fileInfo.Path)
	case "typescript":
		functions, types, calls, err = p.parseTypeScriptAST(content, fileInfo.Path)
	case "protobuf":
		// Use regex-based parsing for protobuf (no tree-sitter grammar bundled)
		functions, calls = parseProtobufSimplified(content, fileInfo.Path, p)
	default:
		// Unsupported language - return empty result without error
		p.logger.Debug("parser.treesitter.skip_unsupported",
			"path", fileInfo.Path,
			"language", fileInfo.Language,
		)
		return &ParseResult{
			File:      fileEntity,
			Functions: nil,
			Defines:   nil,
			Calls:     nil,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("parse %s AST: %w", fileInfo.Language, err)
	}

	// Create defines edges for functions
	defines := make([]DefinesEdge, len(functions))
	for i, fn := range functions {
		defines[i] = DefinesEdge{
			FileID:     fileID,
			FunctionID: fn.ID,
		}
	}

	// Create defines edges for types
	definesTypes := make([]DefinesTypeEdge, len(types))
	for i, t := range types {
		definesTypes[i] = DefinesTypeEdge{
			FileID: fileID,
			TypeID: t.ID,
		}
	}

	return &ParseResult{
		File:            fileEntity,
		Functions:       functions,
		Types:           types,
		Defines:         defines,
		DefinesTypes:    definesTypes,
		Calls:           calls,
		Imports:         imports,
		UnresolvedCalls: unresolvedCalls,
		PackageName:     packageName,
	}, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// countErrors counts ERROR nodes in the AST.
func countErrors(node *sitter.Node) int {
	count := 0
	if node.Type() == "ERROR" {
		count++
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		count += countErrors(node.Child(i))
	}
	return count
}

// findNodeAtPosition finds the deepest node at the given position.
// Used for Python/JS/TS call extraction (Go uses direct node references).
func findNodeAtPosition(node *sitter.Node, row, col uint32) *sitter.Node {
	if node == nil {
		return nil
	}

	startRow := node.StartPoint().Row
	startCol := node.StartPoint().Column
	endRow := node.EndPoint().Row
	endCol := node.EndPoint().Column

	// Check if position is within this node
	inNode := false
	if row > startRow && row < endRow {
		inNode = true
	} else if row == startRow && row == endRow {
		inNode = col >= startCol && col <= endCol
	} else if row == startRow {
		inNode = col >= startCol
	} else if row == endRow {
		inNode = col <= endCol
	}

	if !inNode {
		return nil
	}

	// Try to find a more specific child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		found := findNodeAtPosition(child, row, col)
		if found != nil {
			return found
		}
	}

	return node
}
