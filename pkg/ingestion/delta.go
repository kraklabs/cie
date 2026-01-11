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
	resolvedBase, resolvedHead, err := dd.resolveRefs(baseSHA, headSHA)
	if err != nil {
		return nil, err
	}

	delta := &GitDelta{
		BaseSHA: resolvedBase,
		HeadSHA: resolvedHead,
		Renamed: make(map[string]string),
	}

	output, err := dd.runGitDiff(resolvedBase, resolvedHead)
	if err != nil {
		return nil, err
	}

	if err := dd.parseDiffOutput(output, delta); err != nil {
		return nil, err
	}

	sortDeltaLists(delta)
	rebuildAllList(delta)
	dd.logDeltaComplete(resolvedBase, resolvedHead, delta)

	return delta, nil
}

// resolveRefs resolves base and head refs to commit SHAs.
func (dd *DeltaDetector) resolveRefs(baseSHA, headSHA string) (resolvedBase, resolvedHead string, err error) {
	if headSHA == "" {
		headSHA = "HEAD"
	}

	resolvedHead, err = dd.resolveRef(headSHA)
	if err != nil {
		return "", "", fmt.Errorf("resolve head SHA: %w", err)
	}

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
			return "", "", fmt.Errorf("resolve base SHA: %w", err)
		}
	}

	return resolvedBase, resolvedHead, nil
}

// runGitDiff executes git diff with rename detection.
func (dd *DeltaDetector) runGitDiff(resolvedBase, resolvedHead string) ([]byte, error) {
	cmd := exec.Command("git", "diff", "--name-status", "-M", resolvedBase, resolvedHead) //nolint:gosec // G204: args are SHA hashes from git rev-parse
	cmd.Dir = dd.repoPath

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git diff: %w", err)
	}
	return output, nil
}

// parseDiffOutput parses git diff output into delta struct.
func (dd *DeltaDetector) parseDiffOutput(output []byte, delta *GitDelta) error {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		dd.processDiffLine(line, delta)
	}
	return scanner.Err()
}

// processDiffLine handles a single line from git diff output.
func (dd *DeltaDetector) processDiffLine(line string, delta *GitDelta) {
	status, paths := parseGitDiffLine(line)
	if status == "" || len(paths) == 0 {
		return
	}

	switch status[0] {
	case 'A':
		delta.Added = append(delta.Added, paths[0])
	case 'M':
		delta.Modified = append(delta.Modified, paths[0])
	case 'D':
		delta.Deleted = append(delta.Deleted, paths[0])
	case 'R':
		if len(paths) >= 2 {
			delta.Renamed[paths[0]] = paths[1]
		}
	case 'C':
		if len(paths) >= 2 {
			delta.Added = append(delta.Added, paths[1])
		}
	}
}

// logDeltaComplete logs the completion of delta detection.
func (dd *DeltaDetector) logDeltaComplete(resolvedBase, resolvedHead string, delta *GitDelta) {
	dd.logger.Info("delta.detect.complete",
		"base_sha", resolvedBase[:minInt(8, len(resolvedBase))],
		"head_sha", resolvedHead[:minInt(8, len(resolvedHead))],
		"added", len(delta.Added),
		"modified", len(delta.Modified),
		"deleted", len(delta.Deleted),
		"renamed", len(delta.Renamed),
		"total_changed", len(delta.All),
	)
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
	fc := &filterContext{excludeGlobs: excludeGlobs, maxFileSize: maxFileSize, repoPath: repoPath}
	filtered := &GitDelta{
		BaseSHA: delta.BaseSHA,
		HeadSHA: delta.HeadSHA,
		Renamed: make(map[string]string),
	}

	filtered.Added = fc.filterPaths(delta.Added, true)
	filtered.Modified = fc.filterPaths(delta.Modified, true)
	filtered.Deleted = fc.filterPaths(delta.Deleted, false)
	fc.filterRenamed(delta.Renamed, filtered)

	sortDeltaLists(filtered)
	rebuildAllList(filtered)

	return filtered
}

// filterContext holds filtering configuration for delta operations.
type filterContext struct {
	excludeGlobs []string
	maxFileSize  int64
	repoPath     string
}

// shouldInclude checks if path matches exclude glob patterns.
func (fc *filterContext) shouldInclude(path string) bool {
	normalizedPath := filepath.ToSlash(path)
	for _, pattern := range fc.excludeGlobs {
		if matchesGlob(normalizedPath, pattern) {
			return false
		}
	}
	return true
}

// checkFileEligible validates basic constraints (exists, regular file, size, textual).
func (fc *filterContext) checkFileEligible(path string) bool {
	fullPath := filepath.Join(fc.repoPath, path)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return true // File doesn't exist - let later stages handle it
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
		return false
	}
	if fc.maxFileSize > 0 && info.Size() > fc.maxFileSize {
		return false
	}
	return !isBinaryFile(fullPath)
}

// isBinaryFile checks if file appears to be binary by scanning for NUL bytes.
func isBinaryFile(fullPath string) bool {
	f, err := os.Open(fullPath) //nolint:gosec // G304: path validated by caller
	if err != nil {
		return false // Can't open - let later stages handle it
	}
	defer func() { _ = f.Close() }()
	const sniff = 8192
	buf := make([]byte, sniff)
	n, _ := io.ReadFull(f, buf)
	if n <= 0 {
		return false
	}
	return bytes.IndexByte(buf[:n], 0x00) >= 0
}

// filterPaths filters a slice of paths using include/eligibility checks.
func (fc *filterContext) filterPaths(paths []string, checkEligible bool) []string {
	var result []string
	for _, p := range paths {
		if !fc.shouldInclude(p) {
			continue
		}
		if checkEligible && !fc.checkFileEligible(p) {
			continue
		}
		result = append(result, p)
	}
	return result
}

// filterRenamed processes renamed files, converting ineligible renames to deletions.
func (fc *filterContext) filterRenamed(renamed map[string]string, filtered *GitDelta) {
	for oldPath, newPath := range renamed {
		if fc.shouldInclude(newPath) && fc.checkFileEligible(newPath) {
			filtered.Renamed[oldPath] = newPath
			continue
		}
		if fc.shouldInclude(oldPath) {
			filtered.Deleted = append(filtered.Deleted, oldPath)
		}
	}
}

// sortDeltaLists ensures deterministic ordering of all lists.
func sortDeltaLists(d *GitDelta) {
	sort.Strings(d.Added)
	sort.Strings(d.Modified)
	sort.Strings(d.Deleted)
	if len(d.Renamed) > 1 {
		keys := make([]string, 0, len(d.Renamed))
		for k := range d.Renamed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make(map[string]string, len(d.Renamed))
		for _, k := range keys {
			ordered[k] = d.Renamed[k]
		}
		d.Renamed = ordered
	}
}

// rebuildAllList reconstructs the All list from all buckets.
func rebuildAllList(d *GitDelta) {
	allSet := make(map[string]bool)
	for _, p := range d.Added {
		allSet[p] = true
	}
	for _, p := range d.Modified {
		allSet[p] = true
	}
	for _, p := range d.Deleted {
		allSet[p] = true
	}
	for oldPath, newPath := range d.Renamed {
		allSet[oldPath] = true
		allSet[newPath] = true
	}
	d.All = make([]string, 0, len(allSet))
	for p := range allSet {
		d.All = append(d.All, p)
	}
	sort.Strings(d.All)
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
