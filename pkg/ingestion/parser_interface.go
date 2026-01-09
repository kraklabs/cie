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
