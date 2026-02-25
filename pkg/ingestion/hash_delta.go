// Copyright 2025 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kraklabs/cie/pkg/storage"
)

// HashDeltaDetector detects file changes by comparing content hashes.
// Works without Git - suitable for any VCS or no VCS at all.
type HashDeltaDetector struct {
	logger   *slog.Logger
	repoPath string
	backend  *storage.EmbeddedBackend
}

// NewHashDeltaDetector creates a hash-based delta detector.
func NewHashDeltaDetector(repoPath string, backend *storage.EmbeddedBackend, logger *slog.Logger) *HashDeltaDetector {
	if logger == nil {
		logger = slog.Default()
	}
	return &HashDeltaDetector{
		logger:   logger,
		repoPath: repoPath,
		backend:  backend,
	}
}

// FileHashState represents the stored hash for a file.
type FileHashState struct {
	Path string
	Hash string
}

// DetectChanges compares current files with stored hashes and returns delta.
// - currentFiles: files discovered on disk (from LoadRepository)
// Returns GitDelta-style result with Added, Modified, Deleted lists.
func (hd *HashDeltaDetector) DetectChanges(ctx context.Context, currentFiles []FileInfo) (*GitDelta, error) {
	delta := &GitDelta{
		Renamed: make(map[string]string),
	}

	// Load stored file hashes from database
	storedHashes, err := hd.loadStoredHashes(ctx)
	if err != nil {
		return nil, fmt.Errorf("load stored hashes: %w", err)
	}

	// Build map of current files by path for quick lookup
	currentMap := make(map[string]FileInfo, len(currentFiles))
	for _, f := range currentFiles {
		currentMap[f.Path] = f
	}

	// Build map of stored files by path
	storedMap := make(map[string]string, len(storedHashes))
	for _, s := range storedHashes {
		storedMap[s.Path] = s.Hash
	}

	hd.logger.Info("hash_delta.compare",
		"stored_files", len(storedMap),
		"current_files", len(currentFiles),
	)

	// Find added and modified files
	for _, current := range currentFiles {
		storedHash, exists := storedMap[current.Path]
		if !exists {
			// New file (not in database)
			delta.Added = append(delta.Added, current.Path)
			AppendIndexLog(filepath.Join(hd.repoPath, ".cie"),
				fmt.Sprintf("added %s", current.Path))
		} else {
			// Existing file - need to compare hash
			hash, err := hd.computeFileHash(current.FullPath)
			if err != nil {
				hd.logger.Warn("hash_delta.hash_failed", "path", current.Path, "err", err)
				AppendIndexLog(filepath.Join(hd.repoPath, ".cie"),
					fmt.Sprintf("hash_failed %s: %v", current.Path, err))
				continue
			}
			if hash != storedHash {
				delta.Modified = append(delta.Modified, current.Path)
				AppendIndexLog(filepath.Join(hd.repoPath, ".cie"),
					fmt.Sprintf("modified %s", current.Path))
			}
		}
	}

	// Find deleted files (in database but not on disk)
	for _, stored := range storedHashes {
		if _, exists := currentMap[stored.Path]; !exists {
			delta.Deleted = append(delta.Deleted, stored.Path)
			AppendIndexLog(filepath.Join(hd.repoPath, ".cie"),
				fmt.Sprintf("deleted %s", stored.Path))
		}
	}

	rebuildAllList(delta)
	hd.logger.Info("hash_delta.complete",
		"added", len(delta.Added),
		"modified", len(delta.Modified),
		"deleted", len(delta.Deleted),
	)

	return delta, nil
}

// loadStoredHashes retrieves all file hashes from the database.
func (hd *HashDeltaDetector) loadStoredHashes(ctx context.Context) ([]FileHashState, error) {
	query := `?[path, hash] := *cie_file { path, hash }`

	result, err := hd.backend.Query(ctx, query)
	if err != nil {
		hd.logger.Warn("hash_delta.load_hashes_error", "err", err)
		return nil, fmt.Errorf("query file hashes: %w", err)
	}

	var states []FileHashState
	for _, row := range result.Rows {
		if len(row) >= 2 {
			path, _ := row[0].(string)
			hash, _ := row[1].(string)
			if path != "" && hash != "" {
				states = append(states, FileHashState{Path: path, Hash: hash})
			}
		}
	}

	return states, nil
}

// computeFileHash computes SHA256 hash of file content.
func (hd *HashDeltaDetector) computeFileHash(fullPath string) (string, error) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// IsAvailable checks if the backend is available for hash-based detection.
func (hd *HashDeltaDetector) IsAvailable() bool {
	return hd.backend != nil
}
