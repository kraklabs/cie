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
	"path/filepath"
)

// GenerateFileID generates a deterministic file ID from the file path.
// Strategy: Use normalized path as ID (or hash if path is too long).
func GenerateFileID(filePath string) string {
	// Normalize path: use forward slashes, remove leading ./
	normalized := normalizePath(filePath)

	// If path is reasonable length, use it directly
	// Otherwise hash it to keep IDs manageable
	if len(normalized) <= 256 {
		return fmt.Sprintf("file:%s", normalized)
	}

	// Hash long paths
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("file:%s", hex.EncodeToString(hash[:16])) // Use first 16 bytes
}

// GenerateFunctionID generates a deterministic function ID.
// Strategy: hash(file_path + name + start_line + end_line + start_col + end_col).
// NOTE: Signature is NOT included to ensure IDs remain stable when parser improvements
// change signature extraction. Signature is stored as metadata in the function entity.
// Including start_col and end_col reduces collision risk for functions with same name
// at same line range (e.g., overloads, nested functions).
func GenerateFunctionID(filePath, name, signature string, startLine, endLine int, startCol, endCol int) string {
	normalizedPath := normalizePath(filePath)

	// Build a stable identifier string using path, name, and full range (line + column)
	// Signature is excluded to maintain idempotency across parser improvements
	// Columns are included to prevent collisions when functions share line ranges
	idStr := fmt.Sprintf("%s|%s|%d|%d|%d|%d", normalizedPath, name, startLine, endLine, startCol, endCol)

	// Hash to get fixed-length ID
	hash := sha256.Sum256([]byte(idStr))
	return fmt.Sprintf("func:%s", hex.EncodeToString(hash[:]))
}

// normalizePath normalizes a file path for consistent ID generation.
// Ensures cross-platform consistency by:
//   - Removing leading ./
//   - Normalizing path separators to forward slashes (cross-platform)
//   - Cleaning the path (removing redundant separators, etc.)
//   - Converting absolute paths to relative (if they start with common prefixes)
func normalizePath(path string) string {
	// Remove leading ./
	if len(path) >= 2 && path[0:2] == "./" {
		path = path[2:]
	}
	// Clean the path (removes redundant separators, etc.)
	path = filepath.Clean(path)
	// Normalize separators to forward slashes for cross-platform consistency
	// This ensures IDs are the same on Windows and Unix systems
	path = filepath.ToSlash(path)
	// Remove leading slash to ensure relative paths are consistent
	// This handles cases where paths might be absolute on some systems
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return path
}
