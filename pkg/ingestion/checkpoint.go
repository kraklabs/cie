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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Checkpoint tracks ingestion progress for restartability.
//
// If the ingestion process is interrupted, the checkpoint allows resuming
// from where it left off without re-processing all files.
type Checkpoint struct {
	// ProjectID is the unique identifier for the project being indexed.
	ProjectID string `json:"project_id"`

	// LastProcessedFile is the path of the last file successfully processed.
	// Used to resume from the next file on restart.
	LastProcessedFile string `json:"last_processed_file,omitempty"`

	// LastCommittedIndex is the replication log index of the last committed write.
	// Used to avoid duplicate writes on resume.
	LastCommittedIndex uint64 `json:"last_committed_index"`

	// FilesProcessed is the count of files successfully processed so far.
	FilesProcessed int `json:"files_processed"`

	// FunctionsExtracted is the count of functions discovered so far.
	FunctionsExtracted int `json:"functions_extracted"`

	// BatchesSent is the number of Datalog batches successfully written.
	BatchesSent int `json:"batches_sent"`

	// EntitiesSent maps entity types to counts (e.g., "function": 150, "type": 45).
	EntitiesSent map[string]int `json:"entities_sent"`

	// SentBatchRequestIDs maps batch indices to request IDs for idempotent resumption.
	// This ensures the same batch isn't sent twice if resuming mid-write.
	SentBatchRequestIDs map[int]string `json:"sent_batch_request_ids,omitempty"`

	// FileHashes maps file paths to content hashes for detecting modified files.
	// Used for incremental updates to skip unchanged files.
	FileHashes map[string]string `json:"file_hashes,omitempty"`

	// DatalogScript is the cached Datalog mutation script.
	// Cached to avoid regenerating on resume (performance optimization).
	DatalogScript string `json:"datalog_script,omitempty"`

	// Batches is the list of cached Datalog batch scripts.
	// Cached to avoid re-batching on resume (performance optimization).
	Batches []string `json:"batches,omitempty"`

	// StartTime is the ISO 8601 timestamp when ingestion started.
	StartTime string `json:"start_time"`

	// LastUpdateTime is the ISO 8601 timestamp of the last checkpoint save.
	LastUpdateTime string `json:"last_update_time"`
}

// CheckpointManager manages checkpoint persistence.
type CheckpointManager struct {
	checkpointPath string
}

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager(checkpointPath string) *CheckpointManager {
	return &CheckpointManager{
		checkpointPath: checkpointPath,
	}
}

// LoadCheckpoint loads a checkpoint from disk.
func (cm *CheckpointManager) LoadCheckpoint(projectID string) (*Checkpoint, error) {
	path := cm.getCheckpointPath(projectID)

	data, err := os.ReadFile(path) //nolint:gosec // G304: path from checkpoint manager
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No checkpoint exists
		}
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}

	// Initialize SentBatchRequestIDs if nil (for backward compatibility)
	if checkpoint.SentBatchRequestIDs == nil {
		checkpoint.SentBatchRequestIDs = make(map[int]string)
	}

	// Initialize FileHashes if nil (for backward compatibility)
	if checkpoint.FileHashes == nil {
		checkpoint.FileHashes = make(map[string]string)
	}

	// DatalogScript field was added later - if missing, it will be regenerated
	// This is fine for backward compatibility

	// Batches field was added later - if missing, it will be regenerated
	// This is fine for backward compatibility
	if checkpoint.Batches == nil {
		checkpoint.Batches = nil // Will be regenerated if needed
	}

	return &checkpoint, nil
}

// SaveCheckpoint saves a checkpoint to disk.
func (cm *CheckpointManager) SaveCheckpoint(checkpoint *Checkpoint) error {
	path := cm.getCheckpointPath(checkpoint.ProjectID)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}

	// Write atomically (temp file + rename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write checkpoint temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Cleanup on error (ignore error as rename already failed)
		return fmt.Errorf("rename checkpoint: %w", err)
	}

	return nil
}

// ClearCheckpoint removes a checkpoint file.
func (cm *CheckpointManager) ClearCheckpoint(projectID string) error {
	path := cm.getCheckpointPath(projectID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove checkpoint: %w", err)
	}
	return nil
}

// getCheckpointPath returns the checkpoint file path for a project.
func (cm *CheckpointManager) getCheckpointPath(projectID string) string {
	if cm.checkpointPath != "" {
		return filepath.Join(cm.checkpointPath, fmt.Sprintf("checkpoint-%s.json", projectID))
	}
	// Default: current directory
	return fmt.Sprintf("checkpoint-%s.json", projectID)
}
