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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"log/slog"
)

// =============================================================================
// GIT DELTA DETECTION (F1.M2)
// =============================================================================
//
// This module detects which files changed between two git commits.
// It uses `git diff --name-status` to get added/modified/deleted/renamed files.
//
// The delta is used to:
// - Re-parse only changed files (not the whole repo)
// - Identify deleted files for cleanup
// - Track renames (treated as delete + add in v1)

// DeltaDetector detects changed files using git.
type DeltaDetector struct {
	logger   *slog.Logger
	repoPath string
}

// NewDeltaDetector creates a new delta detector for a git repository.
func NewDeltaDetector(repoPath string, logger *slog.Logger) *DeltaDetector {
	if logger == nil {
		logger = slog.Default()
	}
	return &DeltaDetector{
		logger:   logger,
		repoPath: repoPath,
	}
}

// GitDelta represents the changes between two commits.
type GitDelta struct {
	// BaseSHA is the starting commit (older)
	BaseSHA string

	// HeadSHA is the ending commit (newer)
	HeadSHA string

	// Added are files that were added
	Added []string

	// Modified are files that were modified
	Modified []string

	// Deleted are files that were deleted
	Deleted []string

	// Renamed maps old_path -> new_path for renamed files
	Renamed map[string]string

	// All is the union of all changed files (sorted, deduplicated)
	// For renamed files, includes both old and new paths
	All []string
}

// ChangeType returns the type of change for a file path.
func (d *GitDelta) ChangeType(path string) FileChangeType {
	for _, p := range d.Added {
		if p == path {
			return FileAdded
		}
	}
	for _, p := range d.Modified {
		if p == path {
			return FileModified
		}
	}
	for _, p := range d.Deleted {
		if p == path {
			return FileDeleted
		}
	}
	// Check if this is the new path of a rename
	for oldPath, newPath := range d.Renamed {
		if newPath == path {
			return FileRenamed
		}
		if oldPath == path {
			return FileDeleted // Old path of rename is effectively deleted
		}
	}
	return "" // Not in delta
}

// GetOldPath returns the old path for a renamed file, or "" if not renamed.
func (d *GitDelta) GetOldPath(newPath string) string {
	for oldPath, np := range d.Renamed {
		if np == newPath {
			return oldPath
		}
	}
	return ""
}

// DetectDelta detects changed files between two commits.
// If baseSHA is empty, compares headSHA against an empty tree (all files are "added").
// If headSHA is empty, uses HEAD.
func (dd *DeltaDetector) DetectDelta(baseSHA, headSHA string) (*GitDelta, error) {
	// Validate inputs
	if headSHA == "" {
		headSHA = "HEAD"
	}

	// Resolve HEADs to actual SHAs for logging
	resolvedHead, err := dd.resolveRef(headSHA)
	if err != nil {
		return nil, fmt.Errorf("resolve head SHA: %w", err)
	}

	var resolvedBase string
	if baseSHA == "" {
		// Use empty tree SHA for initial commit comparison (all files are "added")
		resolvedBase = "4b825dc642cb6eb9a060e54bf8d69288fbee4904" // Git's empty tree SHA
		dd.logger.Info("delta.detect.initial",
			"head_sha", resolvedHead[:minInt(8, len(resolvedHead))],
			"msg", "comparing against empty tree (initial ingestion)",
		)
	} else {
		resolvedBase, err = dd.resolveRef(baseSHA)
		if err != nil {
			return nil, fmt.Errorf("resolve base SHA: %w", err)
		}
	}

	delta := &GitDelta{
		BaseSHA: resolvedBase,
		HeadSHA: resolvedHead,
		Renamed: make(map[string]string),
	}

	// Get diff with rename detection
	// --name-status: shows A/M/D/R status
	// -M: detect renames
	// --no-renames is NOT used so we get rename info
	cmd := exec.Command("git", "diff", "--name-status", "-M", resolvedBase, resolvedHead)
	cmd.Dir = dd.repoPath

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}

	// Parse output
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		status, paths := parseGitDiffLine(line)
		if status == "" {
			continue
		}

		switch status[0] {
		case 'A':
			delta.Added = append(delta.Added, paths[0])
		case 'M':
			delta.Modified = append(delta.Modified, paths[0])
		case 'D':
			delta.Deleted = append(delta.Deleted, paths[0])
		case 'R':
			// Rename: status is "R100" or "R95" (percentage), paths[0] = old, paths[1] = new
			if len(paths) >= 2 {
				delta.Renamed[paths[0]] = paths[1]
			}
		case 'C':
			// Copy: treat as add
			if len(paths) >= 2 {
				delta.Added = append(delta.Added, paths[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}

	// Ensure deterministic ordering per bucket before building All
	sort.Strings(delta.Added)
	sort.Strings(delta.Modified)
	sort.Strings(delta.Deleted)
	// Renames: build a sorted view of keys for determinism in logs/debug
	if len(delta.Renamed) > 1 {
		keys := make([]string, 0, len(delta.Renamed))
		for k := range delta.Renamed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Reinsert in a stable map order by recreating the map (Go maps are unordered,
		// but this ensures any iteration we do locally over keys is stable).
		ordered := make(map[string]string, len(delta.Renamed))
		for _, k := range keys {
			ordered[k] = delta.Renamed[k]
		}
		delta.Renamed = ordered
	}

	// Build All list (sorted, deduplicated)
	allSet := make(map[string]bool)
	for _, p := range delta.Added {
		allSet[p] = true
	}
	for _, p := range delta.Modified {
		allSet[p] = true
	}
	for _, p := range delta.Deleted {
		allSet[p] = true
	}
	for oldPath, newPath := range delta.Renamed {
		allSet[oldPath] = true
		allSet[newPath] = true
	}

	delta.All = make([]string, 0, len(allSet))
	for p := range allSet {
		delta.All = append(delta.All, p)
	}
	sort.Strings(delta.All)

	dd.logger.Info("delta.detect.complete",
		"base_sha", resolvedBase[:minInt(8, len(resolvedBase))],
		"head_sha", resolvedHead[:minInt(8, len(resolvedHead))],
		"added", len(delta.Added),
		"modified", len(delta.Modified),
		"deleted", len(delta.Deleted),
		"renamed", len(delta.Renamed),
		"total_changed", len(delta.All),
	)

	return delta, nil
}

// parseGitDiffLine parses a line from git diff --name-status output.
// Returns status (A/M/D/R###/C###) and paths.
func parseGitDiffLine(line string) (status string, paths []string) {
	// Format: "STATUS\tpath" or "STATUS\told_path\tnew_path" for renames
	parts := strings.Split(line, "\t")
	if len(parts) < 2 {
		return "", nil
	}

	status = parts[0]
	paths = parts[1:]

	// Normalize paths (remove quotes if present)
	for i, p := range paths {
		paths[i] = unquoteGitPath(p)
	}

	return status, paths
}

// unquoteGitPath removes quotes and handles escape sequences from git paths.
func unquoteGitPath(path string) string {
	// Git quotes paths with special characters
	if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
		// Remove quotes and unescape
		unquoted := path[1 : len(path)-1]
		// Handle common escapes
		unquoted = strings.ReplaceAll(unquoted, "\\n", "\n")
		unquoted = strings.ReplaceAll(unquoted, "\\t", "\t")
		unquoted = strings.ReplaceAll(unquoted, "\\\\", "\\")
		unquoted = strings.ReplaceAll(unquoted, "\\\"", "\"")
		return unquoted
	}
	return path
}

// resolveRef resolves a git ref (branch, tag, HEAD) to a commit SHA.
func (dd *DeltaDetector) resolveRef(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dd.repoPath

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git rev-parse %s failed: %s", ref, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git rev-parse: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetHeadSHA returns the current HEAD SHA.
func (dd *DeltaDetector) GetHeadSHA() (string, error) {
	return dd.resolveRef("HEAD")
}

// IsGitRepository checks if the repo path is a valid git repository.
func (dd *DeltaDetector) IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dd.repoPath
	err := cmd.Run()
	return err == nil
}

// =============================================================================
// DELTA FILTERING
// =============================================================================

// FilterDelta filters a GitDelta to only include files matching criteria.
// - excludeGlobs: patterns to exclude (e.g., "vendor/**", "node_modules/**")
// - maxFileSize: maximum file size in bytes (0 = no limit)
// - repoPath: path to repository root (for checking file sizes)
func FilterDelta(delta *GitDelta, excludeGlobs []string, maxFileSize int64, repoPath string) *GitDelta {
	filtered := &GitDelta{
		BaseSHA: delta.BaseSHA,
		HeadSHA: delta.HeadSHA,
		Renamed: make(map[string]string),
	}

	// shouldInclude checks glob patterns
	shouldInclude := func(path string) bool {
		// Check exclude globs
		normalizedPath := filepath.ToSlash(path)
		for _, pattern := range excludeGlobs {
			if matchesGlob(normalizedPath, pattern) {
				return false
			}
		}
		return true
	}

	// checkFileEligible validates basic constraints (exists, regular file, size, textual)
	checkFileEligible := func(path string) bool {
		if maxFileSize <= 0 {
			// still check for symlinks/dirs and binary
		}
		fullPath := filepath.Join(repoPath, path)
		info, err := os.Lstat(fullPath)
		if err != nil {
			// File doesn't exist or can't be read - for deleted files this is expected
			// For others, include and let later stages handle it
			return true
		}
		// Skip directories and symlinks (potential submodules appear as gitlinks/symlinks)
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return false
		}
		if maxFileSize > 0 && info.Size() > maxFileSize {
			return false
		}
		// Heuristic binary detection: scan first 8KB for NUL byte
		f, err := os.Open(fullPath)
		if err != nil {
			// If we cannot open, let later stages handle it
			return true
		}
		defer f.Close()
		const sniff = 8192
		buf := make([]byte, sniff)
		n, _ := io.ReadFull(f, buf)
		if n <= 0 {
			return true
		}
		// If we find a NUL byte, treat as binary
		if bytes.IndexByte(buf[:n], 0x00) >= 0 {
			return false
		}
		return true
	}

	for _, p := range delta.Added {
		if shouldInclude(p) && checkFileEligible(p) {
			filtered.Added = append(filtered.Added, p)
		}
	}

	for _, p := range delta.Modified {
		if shouldInclude(p) && checkFileEligible(p) {
			filtered.Modified = append(filtered.Modified, p)
		}
	}

	// For deleted files, we always include them (no size check needed - file doesn't exist)
	for _, p := range delta.Deleted {
		if shouldInclude(p) {
			filtered.Deleted = append(filtered.Deleted, p)
		}
	}

	for oldPath, newPath := range delta.Renamed {
		// If the new path is included and eligible, keep it as a rename
		if shouldInclude(newPath) && checkFileEligible(newPath) {
			filtered.Renamed[oldPath] = newPath
			continue
		}
		// Otherwise, treat this as an effective deletion of the old path.
		// Rationale: the file moved to an excluded/unsupported location; we
		// must still clean the old indexed state to avoid stale entities.
		if shouldInclude(oldPath) { // respect exclusions on the old path, too
			filtered.Deleted = append(filtered.Deleted, oldPath)
		}
	}

	// Ensure deterministic ordering of each bucket after filtering
	sort.Strings(filtered.Added)
	sort.Strings(filtered.Modified)
	sort.Strings(filtered.Deleted)
	if len(filtered.Renamed) > 1 {
		keys := make([]string, 0, len(filtered.Renamed))
		for k := range filtered.Renamed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make(map[string]string, len(filtered.Renamed))
		for _, k := range keys {
			ordered[k] = filtered.Renamed[k]
		}
		filtered.Renamed = ordered
	}

	// Rebuild All list
	allSet := make(map[string]bool)
	for _, p := range filtered.Added {
		allSet[p] = true
	}
	for _, p := range filtered.Modified {
		allSet[p] = true
	}
	for _, p := range filtered.Deleted {
		allSet[p] = true
	}
	for oldPath, newPath := range filtered.Renamed {
		allSet[oldPath] = true
		allSet[newPath] = true
	}

	filtered.All = make([]string, 0, len(allSet))
	for p := range allSet {
		filtered.All = append(filtered.All, p)
	}
	sort.Strings(filtered.All)

	return filtered
}

// =============================================================================
// DELTA STATISTICS
// =============================================================================

// DeltaStats provides summary statistics for a delta.
type DeltaStats struct {
	AddedCount    int
	ModifiedCount int
	DeletedCount  int
	RenamedCount  int
	TotalChanged  int
}

// GetStats computes summary statistics for the delta.
func (d *GitDelta) GetStats() DeltaStats {
	return DeltaStats{
		AddedCount:    len(d.Added),
		ModifiedCount: len(d.Modified),
		DeletedCount:  len(d.Deleted),
		RenamedCount:  len(d.Renamed),
		TotalChanged:  len(d.All),
	}
}

// HasChanges returns true if there are any changes in the delta.
func (d *GitDelta) HasChanges() bool {
	return len(d.All) > 0
}

// minInt returns the minimum of two ints.
// Note: Using minInt to avoid conflict with Go 1.21+ builtin min.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
