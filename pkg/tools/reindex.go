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

package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kraklabs/cie/pkg/ingestion"
	"github.com/kraklabs/cie/pkg/storage"
)

// ReindexArgs contains arguments for the Reindex tool.
type ReindexArgs struct {
	// Force performs a full reindex instead of incremental
	Force bool

	// Paths is an optional list of specific file paths to reindex
	Paths []string

	// ProjectRoot is the root directory of the project
	ProjectRoot string

	// ConfigPath is the path to the .cie/project.yaml config file
	ConfigPath string
	
	// CancelJobID if set, cancels the specified running job instead of starting a new one
	CancelJobID string
}

// ReindexResult contains the results of a reindex operation.
type ReindexResult struct {
	JobID          string
	Status         string
	FilesProcessed int
	FunctionsFound int
	TypesFound     int
	Duration       time.Duration
	IsIncremental  bool
	Cancelled      bool
	Error          string
}

// Reindex performs full or incremental reindexing of the codebase.
// This can be called from the MCP server to trigger reindexing.
func Reindex(ctx context.Context, args ReindexArgs) (*ToolResult, error) {
	coordinator := storage.GetReindexCoordinator()

	// Handle cancellation request
	if args.CancelJobID != "" {
		cancelled := coordinator.CancelReindex(args.CancelJobID)
		if cancelled {
			return NewResult(fmt.Sprintf(
				"# Reindex Cancelled ✅\n\n**Job ID:** `%s`\n\nThe reindex operation has been cancelled.",
				args.CancelJobID,
			)), nil
		}
		return NewResult(fmt.Sprintf(
			"# Cancel Request Failed\n\nNo active job found with ID: `%s`",
			args.CancelJobID,
		)), nil
	}

	// Try to start indexing - this will wait for active queries to complete
	jobID, err := coordinator.StartReindex(ctx, args.Force, args.Paths)
	if err != nil {
		// Indexing already in progress or other error
		status := coordinator.GetStatus()
		return NewResult(fmt.Sprintf(
			"# Reindex Request Rejected\n\n"+
				"**Status:** %s\n\n"+
				"**Current Job:** `%s`\n"+
				"**Started:** %s\n\n"+
				"Please wait for the current reindex to complete, or cancel it with:\n"+
				"```\ncie_reindex(cancel_job_id=\"%s\")\n```",
			status.State, status.JobID, status.StartedAt, status.JobID,
		)), nil
	}

	// Perform the actual reindexing
	result := performReindexWithCoordinator(ctx, args, jobID, coordinator)

	// Mark as finished in coordinator
	var finishErr error
	if result.Error != "" {
		finishErr = fmt.Errorf("%s", result.Error)
	}
	coordinator.FinishReindex(jobID, finishErr)

	// Return formatted result
	return formatReindexResult(result), nil
}

// performReindexWithCoordinator does the actual reindexing work.
func performReindexWithCoordinator(
	ctx context.Context,
	args ReindexArgs,
	jobID string,
	coordinator *storage.ReindexCoordinator,
) *ReindexResult {
	result := &ReindexResult{
		JobID:  jobID,
		Status: "running",
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// Check for cancellation periodically
	checkCancel := func() bool {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			result.Status = "cancelled"
			result.Error = "cancelled by user"
			return true
		default:
			return false
		}
	}

	if checkCancel() {
		return result
	}

	// Determine project root
	projectRoot := args.ProjectRoot
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Sprintf("cannot determine project root: %v", err)
			return result
		}
		projectRoot = cwd
	}

	// Load configuration
	config, err := loadConfigForReindex(args.ConfigPath, projectRoot)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("cannot load configuration: %v", err)
		return result
	}

	// Set force reindex flag if requested
	if args.Force {
		config.IngestionConfig.ForceReindex = true
	}

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if checkCancel() {
		return result
	}

	// Create and run pipeline
	pipeline, err := ingestion.NewLocalPipeline(*config, logger)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("create pipeline: %v", err)
		return result
	}
	defer pipeline.Close()

	if checkCancel() {
		return result
	}

	// If specific paths are provided, we need to handle partial reindex
	if len(args.Paths) > 0 && !args.Force {
		// For partial reindex, we'll use incremental logic with specific files
		result.IsIncremental = true
	}

	// Run the pipeline
	ingestionResult, err := pipeline.Run(ctx)
	if err != nil {
		// Check if it was cancelled
		if ctx.Err() == context.Canceled {
			result.Cancelled = true
			result.Status = "cancelled"
			result.Error = "cancelled by user"
		} else {
			result.Status = "failed"
			result.Error = fmt.Sprintf("ingestion failed: %v", err)
		}
		return result
	}

	// Populate result
	result.Status = "completed"
	result.FilesProcessed = ingestionResult.FilesProcessed
	result.FunctionsFound = ingestionResult.FunctionsExtracted
	result.TypesFound = ingestionResult.TypesExtracted
	result.IsIncremental = !args.Force && len(args.Paths) > 0

	return result
}

// loadConfigForReindex loads the project configuration for reindexing.
func loadConfigForReindex(configPath, projectRoot string) (*ingestion.Config, error) {
	// Start with default ingestion config
	ingestionCfg := ingestion.DefaultConfig()

	// CRITICAL: Set embedding dimensions to match the provider
	// Mock provider uses 384 dimensions, others use 768 (nomic) or 1536 (openai)
	if ingestionCfg.EmbeddingProvider == "mock" {
		ingestionCfg.EmbeddingDimensions = 384
	} else if ingestionCfg.EmbeddingDimensions == 0 {
		ingestionCfg.EmbeddingDimensions = 768 // default for most providers
	}

	// Determine project ID and data directory
	projectID := ""
	dataDir := ""
	if projectRoot != "" {
		projectID = filepath.Base(projectRoot)
		// Default data dir: ~/.cie/data/<project_id>
		homeDir, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(homeDir, ".cie", "data", projectID)
		}
	}

	// Try to load existing config if available
	if configPath == "" {
		configPath = filepath.Join(projectRoot, ".cie", "project.yaml")
	}

	if _, err := os.Stat(configPath); err == nil {
		// Config exists, we could load additional settings from it
		// For now, use defaults with auto-detected project ID
	}

	// Set the data directory
	ingestionCfg.LocalDataDir = dataDir
	ingestionCfg.LocalEngine = "rocksdb"

	// Build the full config
	return &ingestion.Config{
		ProjectID: projectID,
		RepoSource: ingestion.RepoSource{
			Type:  "local_path",
			Value: projectRoot,
		},
		IngestionConfig: ingestionCfg,
	}, nil
}

// formatReindexResult formats the result as markdown for display.
func formatReindexResult(result *ReindexResult) *ToolResult {
	if result.Cancelled {
		text := fmt.Sprintf(
			"# Reindex Cancelled ⏹️\n\n"+
				"**Job ID:** `%s`\n"+
				"**Duration:** %s\n\n"+
				"The reindex operation was cancelled by the user.",
			result.JobID,
			result.Duration.Round(time.Millisecond),
		)
		return NewResult(text)
	}

	if result.Status == "failed" {
		text := fmt.Sprintf(
			"# Reindex Failed ❌\n\n"+
				"**Job ID:** `%s`\n"+
				"**Duration:** %s\n\n"+
				"**Error:**\n```\n%s\n```",
			result.JobID,
			result.Duration.Round(time.Millisecond),
			result.Error,
		)
		return NewError(text)
	}

	reindexType := "Full"
	if result.IsIncremental {
		reindexType = "Incremental"
	}

	text := fmt.Sprintf(
		"# Reindex Complete ✅\n\n"+
			"**Job ID:** `%s`\n"+
			"**Type:** %s\n"+
			"**Duration:** %s\n\n"+
			"## Results\n"+
			"| Metric | Count |\n"+
			"|--------|-------|\n"+
			"| Files Processed | %d |\n"+
			"| Functions Found | %d |\n"+
			"| Types Found | %d |",
		result.JobID,
		reindexType,
		result.Duration.Round(time.Millisecond),
		result.FilesProcessed,
		result.FunctionsFound,
		result.TypesFound,
	)

	return NewResult(text)
}

// GetReindexStatus returns the current reindex status for display.
func GetReindexStatus() string {
	coordinator := storage.GetReindexCoordinator()
	status := coordinator.GetStatus()

	var result string
	result = "# Reindex Status\n\n"

	// Current state
	switch {
	case status.IsIndexing:
		result += "🔄 **Status:** Indexing in progress\n"
	case status.IsDraining:
		result += "⏳ **Status:** Draining active queries...\n"
	case status.IsReopening:
		result += "📂 **Status:** Reopening database...\n"
	default:
		result += "✅ **Status:** Ready\n"
	}

	if status.JobID != "" {
		result += fmt.Sprintf("**Job ID:** `%s`\n", status.JobID)
	}

	if status.StartedAt != "" {
		result += fmt.Sprintf("**Started:** %s\n", status.StartedAt)
	}

	// Auto-reindex status
	if status.AutoReindexEnabled {
		result += "\n📡 **Auto-reindex:** Enabled\n"
	} else {
		result += "\n📡 **Auto-reindex:** Disabled\n"
	}

	// Pending changes
	if status.PendingChanges > 0 {
		result += fmt.Sprintf("⏳ **Pending Changes:** %d files detected\n", status.PendingChanges)
	}

	// Last reindex time
	if status.LastReindexAt != "" {
		result += fmt.Sprintf("🕐 **Last Reindex:** %s\n", status.LastReindexAt)
	}

	// Last error
	if status.LastError != "" {
		result += fmt.Sprintf("\n⚠️ **Last Error:** %s\n", status.LastError)
	}

	return result
}

// CheckReindexInProgress returns an error if reindexing is currently in progress.
// This is useful for tools that need to fail gracefully during reindex.
func CheckReindexInProgress() error {
	return storage.CheckIndexingInProgress()
}

// GetIndexingState returns the current indexing state.
func GetIndexingState() storage.IndexingStatus {
	coordinator := storage.GetReindexCoordinator()
	return coordinator.GetStatus()
}

// IndexingStatus is an alias for storage.IndexingStatus
type IndexingStatus = storage.IndexingStatus
