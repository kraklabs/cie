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

import (
	"fmt"
	"strings"
)

// Batcher splits Datalog mutations into batches targeting a specific mutation count.
type Batcher struct {
	targetMutations int
	maxScriptSize   int // Maximum script size in bytes (soft limit: 2MB, hard: 4MB)
}

// NewBatcher creates a new batcher.
func NewBatcher(targetMutations int, maxScriptSize int) *Batcher {
	return &Batcher{
		targetMutations: targetMutations,
		maxScriptSize:   maxScriptSize,
	}
}

// Batch splits a Datalog script into multiple batches.
// Each batch targets targetMutations mutations and stays under maxScriptSize bytes.
func (b *Batcher) Batch(script string) ([]string, error) {
	if script == "" {
		return nil, nil
	}

	// Split script into individual mutation statements
	statements := b.splitStatements(script)
	if len(statements) == 0 {
		return nil, nil
	}

	var batches []string
	var currentBatch []string
	currentSize := 0
	currentMutations := 0

	separatorSize := len("\n\n") // Size of separator between statements

	for _, stmt := range statements {
		stmtSize := len(stmt)

		// Calculate actual size including separator
		additionalSize := stmtSize
		if len(currentBatch) > 0 {
			additionalSize += separatorSize
		}

		// Check if adding this statement would exceed limits
		wouldExceedSize := currentSize+additionalSize > b.maxScriptSize
		wouldExceedTarget := currentMutations >= b.targetMutations

		// If current batch is full, start a new one
		if len(currentBatch) > 0 && (wouldExceedSize || wouldExceedTarget) {
			// Use blank line between statements to help Cozo parser separate them
			batch := strings.Join(currentBatch, "\n\n")
			if !strings.HasSuffix(batch, "\n") {
				batch += "\n"
			}
			batches = append(batches, batch)
			currentBatch = nil
			currentSize = 0
			currentMutations = 0
		}

		// If a single statement exceeds max size, that's an error
		// Include statement preview in error for debugging
		if stmtSize > b.maxScriptSize {
			stmtPreview := stmt
			if len(stmtPreview) > 200 {
				stmtPreview = stmtPreview[:200] + "..."
			}
			return nil, fmt.Errorf("mutation statement exceeds max size: %d bytes (limit: %d). Statement preview: %s", stmtSize, b.maxScriptSize, stmtPreview)
		}

		currentBatch = append(currentBatch, stmt)
		if len(currentBatch) == 1 {
			currentSize = stmtSize
		} else {
			currentSize += separatorSize + stmtSize
		}
		currentMutations++
	}

	// Add final batch if any
	if len(currentBatch) > 0 {
		batch := strings.Join(currentBatch, "\n\n")
		if !strings.HasSuffix(batch, "\n") {
			batch += "\n"
		}
		batches = append(batches, batch)
	}

	return batches, nil
}

// splitStatements splits a Datalog script into individual mutation statements.
// Handles CozoDB's batch syntax where each query is wrapped in { }.
// See: https://docs.cozodb.org/en/latest/stored.html
func (b *Batcher) splitStatements(script string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(script, "\n")
	braceDepth := 0
	bracketDepth := 0
	inString := false
	stringChar := rune(0) // Use rune to properly handle Unicode
	escapeNext := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Track depth of braces, brackets, and strings
		// NOTE: We use rune comparison directly, not byte, to avoid
		// false matches with Unicode characters like Ч (U+0427) which
		// would truncate to 0x27 (single quote) when cast to byte.
		for _, char := range line {
			// Handle escape sequences
			if escapeNext {
				escapeNext = false
				continue
			}

			// Handle string literals (only ASCII quotes)
			if !inString && (char == '"' || char == '\'') {
				inString = true
				stringChar = char
			} else if inString && char == stringChar {
				inString = false
				stringChar = 0
			} else if char == '\\' {
				escapeNext = true
				continue
			}

			// Count braces and brackets only outside strings (ASCII only)
			if !inString {
				switch char {
				case '{':
					braceDepth++
				case '}':
					braceDepth--
				case '[':
					bracketDepth++
				case ']':
					bracketDepth--
				}
			}
		}

		// Add line to current statement
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)

		// Statement is complete when all braces are closed
		if braceDepth == 0 && bracketDepth == 0 && !inString && current.Len() > 0 {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" && stmt != "//" && !strings.HasPrefix(stmt, "//") {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}

	// Add any remaining statement
	if current.Len() > 0 {
		stmt := strings.TrimSpace(current.String())
		if stmt != "" && stmt != "//" && !strings.HasPrefix(stmt, "//") {
			statements = append(statements, stmt)
		}
	}

	return statements
}
