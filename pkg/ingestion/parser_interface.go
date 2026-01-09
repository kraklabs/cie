// Copyright 2025 KrakLabs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ingestion

// CodeParser defines the interface for code parsing implementations.
// This allows switching between Tree-sitter and simplified parsers.
type CodeParser interface {
	// ParseFile parses a source file and extracts functions, defines edges, and calls edges.
	ParseFile(fileInfo FileInfo) (*ParseResult, error)

	// SetMaxCodeTextSize sets the maximum size for CodeText (in bytes).
	SetMaxCodeTextSize(size int64)

	// GetTruncatedCount returns the number of CodeTexts that were truncated.
	GetTruncatedCount() int

	// ResetTruncatedCount resets the truncation counter.
	ResetTruncatedCount()
}

// Ensure implementations satisfy the interface
var _ CodeParser = (*TreeSitterParser)(nil)
var _ CodeParser = (*Parser)(nil)

// ParserMode determines which parser implementation to use.
type ParserMode string

const (
	// ParserModeTreeSitter uses Tree-sitter for accurate AST-based parsing.
	// Requires CGO and tree-sitter libraries.
	ParserModeTreeSitter ParserMode = "treesitter"

	// ParserModeSimplified uses regex/string matching (fallback).
	// Does not require CGO, but has limitations with complex code.
	ParserModeSimplified ParserMode = "simplified"

	// ParserModeAuto automatically selects the best available parser.
	// Uses Tree-sitter if available, falls back to simplified.
	ParserModeAuto ParserMode = "auto"
)

// DefaultParserMode is the default parser mode.
// Set to Auto to prefer Tree-sitter when available.
const DefaultParserMode = ParserModeAuto
