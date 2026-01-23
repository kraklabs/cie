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

package bootstrap

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kraklabs/cie/pkg/storage"
)

// ProjectConfig holds configuration for initializing a project.
type ProjectConfig struct {
	// ProjectID is the logical project identifier.
	ProjectID string

	// DataDir is the directory where CozoDB stores its data.
	// Defaults to ~/.cie/data/<project_id>
	DataDir string

	// Engine is the CozoDB storage engine: "rocksdb", "sqlite", or "mem".
	// Defaults to "rocksdb" for persistence.
	Engine string

	// EmbeddingDimensions is the vector size for embeddings.
	// Defaults to 768 (nomic-embed-text). Use 1536 for OpenAI.
	EmbeddingDimensions int
}

// ProjectInfo holds information about an initialized project.
type ProjectInfo struct {
	ProjectID string
	DataDir   string
	Engine    string
}

// InitProject initializes a new CIE project with local CozoDB.
// This function is idempotent: calling it multiple times is safe.
//
// The function:
//  1. Creates the data directory if it doesn't exist
//  2. Opens CozoDB with the specified engine
//  3. Creates schema tables if they don't exist
//  4. Creates HNSW indexes for semantic search
//
// After successful initialization:
//   - CozoDB database exists at DataDir
//   - All required schema tables are created
//   - HNSW indexes are ready for semantic search
//
// Parameters:
//   - config: project configuration
//   - logger: optional logger (nil uses default)
//
// Returns:
//   - ProjectInfo: information about the initialized project
//   - error: if initialization fails
func InitProject(config ProjectConfig, logger *slog.Logger) (*ProjectInfo, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Validate project ID
	if config.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	// Set defaults
	if config.Engine == "" {
		config.Engine = "rocksdb"
	}
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		config.DataDir = filepath.Join(homeDir, ".cie", "data", config.ProjectID)
	}

	logger.Info("bootstrap.project.init.start",
		"project_id", config.ProjectID,
		"data_dir", config.DataDir,
		"engine", config.Engine,
	)

	// Create embedded backend (handles directory creation and schema)
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		DataDir:             config.DataDir,
		Engine:              config.Engine,
		ProjectID:           config.ProjectID,
		EmbeddingDimensions: config.EmbeddingDimensions,
	})
	if err != nil {
		return nil, fmt.Errorf("create backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Ensure schema exists
	if err := backend.EnsureSchema(); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Create HNSW indexes for semantic search
	if err := backend.CreateHNSWIndex(config.EmbeddingDimensions); err != nil {
		logger.Warn("bootstrap.hnsw.warning", "err", err)
		// Don't fail - HNSW is optional for basic functionality
	}

	logger.Info("bootstrap.project.init.success",
		"project_id", config.ProjectID,
		"data_dir", config.DataDir,
	)

	return &ProjectInfo{
		ProjectID: config.ProjectID,
		DataDir:   config.DataDir,
		Engine:    config.Engine,
	}, nil
}

// OpenProject opens an existing CIE project.
// Returns the storage backend for querying the project.
func OpenProject(config ProjectConfig, logger *slog.Logger) (*storage.EmbeddedBackend, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Validate project ID
	if config.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	// Set defaults
	if config.Engine == "" {
		config.Engine = "rocksdb"
	}
	if config.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		config.DataDir = filepath.Join(homeDir, ".cie", "data", config.ProjectID)
	}

	// Check if data directory exists
	if _, err := os.Stat(config.DataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("project not found: %s (run 'cie init' first)", config.DataDir)
	}

	logger.Debug("bootstrap.project.open",
		"project_id", config.ProjectID,
		"data_dir", config.DataDir,
	)

	// Open embedded backend
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		DataDir:   config.DataDir,
		Engine:    config.Engine,
		ProjectID: config.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("open backend: %w", err)
	}

	return backend, nil
}

// ListProjects returns a list of project IDs in the default data directory.
func ListProjects() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".cie", "data")
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No projects yet
		}
		return nil, fmt.Errorf("read data dir: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			projects = append(projects, entry.Name())
		}
	}

	return projects, nil
}
