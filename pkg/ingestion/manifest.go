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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Note: CallsEdge is defined in entities.go

// =============================================================================
// INCREMENTAL INGESTION MANIFEST (F1.M2)
// =============================================================================
//
// The manifest tracks the state of the project DB as seen by the ingestion worker.
// It stores per-file information to enable:
//   - Detecting which files changed between runs
//   - Detecting which functions were removed from a file
//   - Generating accurate :rm mutations for stale entities
//
// Design Choice: Ingestion-side manifest (Option B from spec)
// Reason: Primary Hub does not serve reasoning queries, so we cannot query
// it for the previous function set. The manifest is persisted per project
// and updated transactionally with ingestion progress.

// FunctionManifestEntry tracks a single function's identity for diffing.
type FunctionManifestEntry struct {
	// ID is the deterministic function ID (stable across runs if unchanged)
	ID string `json:"id"`

	// Name is the function name (for debugging/logging)
	Name string `json:"name"`

	// BodyHash is SHA256 of the function's code_text
	// Used to detect content changes even if line numbers shift
	BodyHash string `json:"body_hash"`

	// StartLine/EndLine for reference (may change on edits)
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`

	// Embedding stores the function's embedding vector
	// This allows skipping re-embedding for unchanged functions
	Embedding []float32 `json:"embedding,omitempty"`

	// EmbeddingHash is a hash of the embedding for quick comparison
	// Format: "provider:dimension:hash" to detect provider/dimension changes
	EmbeddingHash string `json:"embedding_hash,omitempty"`
}

// CallEdgeManifestEntry tracks a calls edge for cleanup.
type CallEdgeManifestEntry struct {
	CallerID string `json:"caller_id"`
	CalleeID string `json:"callee_id"`
}

// FileManifestEntry tracks a single file's state for diffing.
type FileManifestEntry struct {
	// ID is the deterministic file ID
	ID string `json:"id"`

	// Path is the relative file path from repo root
	Path string `json:"path"`

	// Hash is SHA256 of the file content
	// Used to detect file changes quickly
	Hash string `json:"hash"`

	// Language detected from file extension
	Language string `json:"language"`

	// Functions is the list of functions in this file
	Functions []FunctionManifestEntry `json:"functions"`

	// CallsEdges tracks calls edges where the caller is in this file
	// Used for cleanup when file is modified or deleted
	CallsEdges []CallEdgeManifestEntry `json:"calls_edges,omitempty"`

	// LastUpdated is when this file was last processed
	LastUpdated time.Time `json:"last_updated"`
}

// ProjectManifest is the complete manifest for a project.
// It stores the state of all files and functions as last ingested.
type ProjectManifest struct {
	// ProjectID is the project identifier
	ProjectID string `json:"project_id"`

	// Version is the manifest schema version
	Version int `json:"version"`

	// Files maps file_path -> FileManifestEntry
	Files map[string]*FileManifestEntry `json:"files"`

	// LastCommittedIndex is the replication log index when this manifest was last updated
	LastCommittedIndex uint64 `json:"last_committed_index"`

	// BaseSHA is the git commit SHA this manifest represents (if git-based)
	BaseSHA string `json:"base_sha,omitempty"`

	// CreatedAt is when the manifest was first created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the manifest was last modified
	UpdatedAt time.Time `json:"updated_at"`

	mu sync.RWMutex `json:"-"` // Protects concurrent access
}

// ManifestVersion is the current schema version
const ManifestVersion = 1

// NewProjectManifest creates a new empty manifest.
func NewProjectManifest(projectID string) *ProjectManifest {
	now := time.Now()
	return &ProjectManifest{
		ProjectID: projectID,
		Version:   ManifestVersion,
		Files:     make(map[string]*FileManifestEntry),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// GetFile returns the manifest entry for a file, or nil if not found.
func (m *ProjectManifest) GetFile(path string) *FileManifestEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Files[path]
}

// SetFile updates or adds a file entry in the manifest.
func (m *ProjectManifest) SetFile(entry *FileManifestEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Files[entry.Path] = entry
	m.UpdatedAt = time.Now()
}

// RemoveFile removes a file from the manifest.
func (m *ProjectManifest) RemoveFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Files, path)
	m.UpdatedAt = time.Now()
}

// GetAllFilePaths returns all file paths in the manifest (sorted).
func (m *ProjectManifest) GetAllFilePaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	paths := make([]string, 0, len(m.Files))
	for path := range m.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// GetFunctionIDs returns all function IDs for a file.
func (m *ProjectManifest) GetFunctionIDs(filePath string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, exists := m.Files[filePath]
	if !exists {
		return nil
	}
	ids := make([]string, len(entry.Functions))
	for i, fn := range entry.Functions {
		ids[i] = fn.ID
	}
	return ids
}

// GetAllFunctionIDs returns all function IDs across all files.
func (m *ProjectManifest) GetAllFunctionIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for _, entry := range m.Files {
		for _, fn := range entry.Functions {
			ids = append(ids, fn.ID)
		}
	}
	return ids
}

// CreateFileManifestEntry creates a manifest entry from parsed file data.
func CreateFileManifestEntry(file FileEntity, functions []FunctionEntity) *FileManifestEntry {
	funcEntries := make([]FunctionManifestEntry, len(functions))
	for i, fn := range functions {
		funcEntries[i] = FunctionManifestEntry{
			ID:            fn.ID,
			Name:          fn.Name,
			BodyHash:      computeBodyHash(fn.CodeText),
			StartLine:     fn.StartLine,
			EndLine:       fn.EndLine,
			Embedding:     fn.Embedding,
			EmbeddingHash: computeEmbeddingHash(fn.Embedding),
		}
	}

	return &FileManifestEntry{
		ID:          file.ID,
		Path:        file.Path,
		Hash:        file.Hash,
		Language:    file.Language,
		Functions:   funcEntries,
		LastUpdated: time.Now(),
	}
}

// CreateFileManifestEntryWithCalls creates a manifest entry including calls edges.
func CreateFileManifestEntryWithCalls(file FileEntity, functions []FunctionEntity, calls []CallsEdge) *FileManifestEntry {
	entry := CreateFileManifestEntry(file, functions)

	// Filter calls edges where caller is in this file
	for _, call := range calls {
		// Check if caller belongs to this file
		for _, fn := range functions {
			if call.CallerID == fn.ID {
				entry.CallsEdges = append(entry.CallsEdges, CallEdgeManifestEntry(call))
				break
			}
		}
	}

	return entry
}

// computeEmbeddingHash creates a hash string for an embedding vector.
// Returns empty string for nil/empty embeddings.
func computeEmbeddingHash(embedding []float32) string {
	if len(embedding) == 0 {
		return ""
	}

	// Create a simple hash based on first/last values and dimension
	// This is fast and sufficient for detecting changes
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "dim:%d:", len(embedding))

	// Sample a few values for the hash (first, middle, last)
	_, _ = fmt.Fprintf(h, "%.6f:", embedding[0])
	if len(embedding) > 2 {
		mid := len(embedding) / 2
		_, _ = fmt.Fprintf(h, "%.6f:", embedding[mid])
	}
	_, _ = fmt.Fprintf(h, "%.6f", embedding[len(embedding)-1])

	return hex.EncodeToString(h.Sum(nil)[:8]) // First 8 bytes = 16 hex chars
}

// GetFunctionEmbedding returns the stored embedding for a function by ID.
// Returns nil if function not found or has no embedding.
func (m *ProjectManifest) GetFunctionEmbedding(filePath, functionID string) []float32 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.Files[filePath]
	if !exists {
		return nil
	}

	for _, fn := range entry.Functions {
		if fn.ID == functionID {
			return fn.Embedding
		}
	}

	return nil
}

// GetCallsEdgesForFile returns all calls edges where caller is in the specified file.
func (m *ProjectManifest) GetCallsEdgesForFile(filePath string) []CallEdgeManifestEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.Files[filePath]
	if !exists {
		return nil
	}

	return entry.CallsEdges
}

// computeBodyHash computes SHA256 hash of function body text.
func computeBodyHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// =============================================================================
// DIFF COMPUTATION
// =============================================================================

// FileDiff represents changes to a single file.
type FileDiff struct {
	// Path is the file path
	Path string

	// ChangeType indicates what happened to the file
	ChangeType FileChangeType

	// OldPath is the previous path (for renames)
	OldPath string

	// OldEntry is the previous manifest entry (nil for added files)
	OldEntry *FileManifestEntry

	// NewEntry is the new manifest entry (nil for deleted files)
	NewEntry *FileManifestEntry

	// AddedFunctions are functions that were added
	AddedFunctions []FunctionManifestEntry

	// ModifiedFunctions are functions whose body changed
	ModifiedFunctions []FunctionManifestEntry

	// RemovedFunctions are functions that no longer exist
	RemovedFunctions []FunctionManifestEntry

	// UnchangedFunctions are functions that didn't change
	UnchangedFunctions []FunctionManifestEntry
}

// FileChangeType indicates the type of change to a file.
type FileChangeType string

const (
	FileAdded    FileChangeType = "added"
	FileModified FileChangeType = "modified"
	FileDeleted  FileChangeType = "deleted"
	FileRenamed  FileChangeType = "renamed"
)

// ComputeFileDiff computes the difference between old and new state for a file.
// oldEntry may be nil (file added), newEntry may be nil (file deleted).
func ComputeFileDiff(path string, oldEntry, newEntry *FileManifestEntry) *FileDiff {
	diff := &FileDiff{
		Path:     path,
		OldEntry: oldEntry,
		NewEntry: newEntry,
	}

	// Determine change type
	switch {
	case oldEntry == nil && newEntry != nil:
		diff.ChangeType = FileAdded
		// All functions are new
		diff.AddedFunctions = append(diff.AddedFunctions, newEntry.Functions...)

	case oldEntry != nil && newEntry == nil:
		diff.ChangeType = FileDeleted
		// All functions are removed
		diff.RemovedFunctions = append(diff.RemovedFunctions, oldEntry.Functions...)

	case oldEntry != nil && newEntry != nil:
		diff.ChangeType = FileModified
		// Compare function sets
		diff.AddedFunctions, diff.ModifiedFunctions, diff.RemovedFunctions, diff.UnchangedFunctions =
			computeFunctionDiff(oldEntry.Functions, newEntry.Functions)
	}

	return diff
}

// computeFunctionDiff computes which functions were added/modified/removed.
func computeFunctionDiff(oldFuncs, newFuncs []FunctionManifestEntry) (
	added, modified, removed, unchanged []FunctionManifestEntry,
) {
	// Build map of old functions by ID
	oldByID := make(map[string]FunctionManifestEntry)
	for _, fn := range oldFuncs {
		oldByID[fn.ID] = fn
	}

	// Build map of new functions by ID
	newByID := make(map[string]FunctionManifestEntry)
	for _, fn := range newFuncs {
		newByID[fn.ID] = fn
	}

	// Check each new function
	for id, newFn := range newByID {
		if oldFn, exists := oldByID[id]; exists {
			// Function exists in both - check if modified
			if oldFn.BodyHash != newFn.BodyHash {
				modified = append(modified, newFn)
			} else {
				unchanged = append(unchanged, newFn)
			}
		} else {
			// Function only in new - added
			added = append(added, newFn)
		}
	}

	// Check for removed functions
	for id, oldFn := range oldByID {
		if _, exists := newByID[id]; !exists {
			// Function only in old - removed
			removed = append(removed, oldFn)
		}
	}

	return added, modified, removed, unchanged
}

// =============================================================================
// MANIFEST PERSISTENCE
// =============================================================================

// ManifestManager handles manifest persistence.
type ManifestManager struct {
	basePath string
}

// NewManifestManager creates a new manifest manager.
func NewManifestManager(basePath string) *ManifestManager {
	return &ManifestManager{
		basePath: basePath,
	}
}

// LoadManifest loads a project manifest from disk.
// Returns nil, nil if manifest doesn't exist (first run).
func (mm *ManifestManager) LoadManifest(projectID string) (*ProjectManifest, error) {
	path := mm.getManifestPath(projectID)

	data, err := os.ReadFile(path) //nolint:gosec // G304: path from manifest manager
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // First run - no manifest yet
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest ProjectManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Initialize mutex (not serialized)
	manifest.mu = sync.RWMutex{}

	// Version migration if needed
	if manifest.Version < ManifestVersion {
		// Future: handle version migrations here
		manifest.Version = ManifestVersion
	}

	// Initialize Files map if nil (backward compatibility)
	if manifest.Files == nil {
		manifest.Files = make(map[string]*FileManifestEntry)
	}

	return &manifest, nil
}

// SaveManifest saves a project manifest to disk atomically.
func (mm *ManifestManager) SaveManifest(manifest *ProjectManifest) error {
	path := mm.getManifestPath(manifest.ProjectID)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	// Update timestamp
	manifest.mu.Lock()
	manifest.UpdatedAt = time.Now()
	manifest.mu.Unlock()

	// Marshal
	manifest.mu.RLock()
	data, err := json.MarshalIndent(manifest, "", "  ")
	manifest.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Write atomically (temp file + rename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write manifest temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Cleanup on failure
		return fmt.Errorf("rename manifest: %w", err)
	}

	return nil
}

// DeleteManifest removes a project manifest from disk.
func (mm *ManifestManager) DeleteManifest(projectID string) error {
	path := mm.getManifestPath(projectID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove manifest: %w", err)
	}
	return nil
}

// getManifestPath returns the path to a project's manifest file.
func (mm *ManifestManager) getManifestPath(projectID string) string {
	if mm.basePath != "" {
		return filepath.Join(mm.basePath, fmt.Sprintf("manifest-%s.json", projectID))
	}
	return fmt.Sprintf("manifest-%s.json", projectID)
}

// =============================================================================
// STATISTICS
// =============================================================================

// ManifestStats provides summary statistics for a manifest.
type ManifestStats struct {
	FileCount      int
	FunctionCount  int
	TotalCodeBytes int64
	LanguageCounts map[string]int
	LastUpdated    time.Time
	BaseSHA        string
	CommittedIndex uint64
}

// GetStats computes summary statistics for the manifest.
func (m *ProjectManifest) GetStats() ManifestStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ManifestStats{
		FileCount:      len(m.Files),
		LanguageCounts: make(map[string]int),
		LastUpdated:    m.UpdatedAt,
		BaseSHA:        m.BaseSHA,
		CommittedIndex: m.LastCommittedIndex,
	}

	for _, file := range m.Files {
		stats.FunctionCount += len(file.Functions)
		if file.Language != "" {
			stats.LanguageCounts[file.Language]++
		}
	}

	return stats
}
