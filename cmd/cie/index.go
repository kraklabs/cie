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

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/schollz/progressbar/v3"
	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/pkg/ingestion"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// runIndex executes the 'index' CLI command, indexing the repository for code intelligence.
//
// It parses source files using Tree-sitter, generates embeddings, and stores the results
// in a local CozoDB database. The indexing process can be run incrementally (default) or
// forced to reindex everything from scratch.
//
// Flags:
//   - --full: Force full reindex, ignoring previous checkpoint (default: false)
//   - --force-full-reindex: Delete checkpoint and reindex everything from scratch
//   - --embed-workers: Number of parallel embedding workers (default: 8)
//   - --debug: Enable debug logging (default: false)
//   - --metrics-addr: HTTP address for Prometheus metrics (default: disabled)
//
// Examples:
//
//	cie index                  Incremental index (only changed files)
//	cie index --full           Force full reindex
//	cie index --embed-workers 16  Use 16 parallel workers for embeddings
func runIndex(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	full := fs.Bool("full", false, "Force full reindex")
	forceFullReindex := fs.Bool("force-full-reindex", false, "Delete checkpoint and reindex everything from scratch")
	embedWorkers := fs.Int("embed-workers", 8, "Number of parallel embedding workers")
	debug := fs.Bool("debug", false, "Enable debug logging")
	metricsAddr := fs.String("metrics-addr", "", "HTTP listen address for Prometheus metrics (empty to disable)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie index [options]

Description:
  Index the current repository to build a searchable code intelligence
  database. This parses source files using Tree-sitter, extracts functions,
  types, and call graphs, and generates embeddings for semantic search.

  The indexing process runs incrementally by default, only processing
  changed files since the last index. Use --full to force a complete
  reindex from scratch.

  Indexed data is stored locally in ~/.cie/data/<project_id>/

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Initial indexing (or incremental update)
  cie index

  # Force full reindex of entire repository
  cie index --full

  # Delete checkpoint and reindex everything
  cie index --force-full-reindex

  # Use 16 parallel workers for faster embedding generation
  cie index --embed-workers 16

  # Enable debug logging and expose metrics
  cie index --debug --metrics-addr :9090

Notes:
  Indexing may take several minutes for large repositories. Progress
  indicators will show files processed and errors encountered.

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		errors.FatalError(errors.NewConfigError(
			"Cannot load CIE configuration",
			"Configuration file is missing or invalid",
			"Run 'cie init' to create a new configuration",
			err,
		), false)
	}

	// Setup logging
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Check for existing data
	if !*full && !*forceFullReindex {
		hasData, funcCount, err := checkLocalData(cfg)
		if err == nil && hasData {
			fmt.Printf("Project '%s' already has %d functions indexed.\n\n", cfg.ProjectID, funcCount)
			fmt.Println("Choose indexing mode:")
			fmt.Println("  cie index --full           Reindex everything from scratch")
			os.Exit(0)
		}
	}

	// Start Prometheus metrics endpoint (optional)
	if *metricsAddr != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", promhttp.Handler())
			srv := &http.Server{Addr: *metricsAddr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
			logger.Info("metrics.http.start", "addr", *metricsAddr, "path", "/metrics")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Warn("metrics.http.error", "err", err)
			}
		}()
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("shutdown.signal", "signal", sig.String())
		cancel()
	}()

	// Get current directory as repo path
	cwd, err := os.Getwd()
	if err != nil {
		errors.FatalError(errors.NewInternalError(
			"Cannot access current directory",
			"Failed to determine working directory",
			"This is unexpected. Please report this issue at github.com/kraklabs/kraken/issues",
			err,
		), false)
	}

	// Map embedding provider
	embeddingProvider := mapEmbeddingProvider(cfg.Embedding.Provider)

	// Delete local data if force-full-reindex is requested
	if *forceFullReindex {
		homeDir, _ := os.UserHomeDir()
		dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)
		if err := os.RemoveAll(dataDir); err == nil {
			logger.Info("data.deleted", "path", dataDir)
		} else if !os.IsNotExist(err) {
			logger.Warn("data.delete.error", "path", dataDir, "err", err)
		}
	}

	runLocalIndex(ctx, logger, cfg, cwd, embeddingProvider, *embedWorkers, globals)
}

// checkLocalData checks if local indexed data exists and returns the function count.
//
// Returns:
//   - bool: true if local data exists and is accessible
//   - int: number of functions indexed (0 if no data)
//   - error: error if database cannot be opened
func checkLocalData(cfg *Config) (bool, int, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, 0, err
	}

	dataDir := filepath.Join(homeDir, ".cie", "data", cfg.ProjectID)
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return false, 0, nil
	}

	// TODO: Query CozoDB for actual function count
	// For now, just check if directory exists
	return true, -1, nil
}

// runLocalIndex executes the local indexing pipeline, writing results to the embedded database.
//
// It walks the repository, parses source files, generates embeddings in parallel, and stores
// all extracted code intelligence data in the local CozoDB database.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - logger: Structured logger for progress reporting
//   - cfg: CIE configuration with project settings
//   - repoPath: Absolute path to the repository root
//   - embeddingProvider: Embedding provider name (ollama, nomic, mock)
//   - embedWorkers: Number of parallel workers for embedding generation
//   - globals: Global CLI flags for progress/output control
func runLocalIndex(ctx context.Context, logger *slog.Logger, cfg *Config, repoPath, embeddingProvider string, embedWorkers int, globals GlobalFlags) {
	// Ensure checkpoint directory exists
	checkpointDir := filepath.Join(ConfigDir(repoPath), "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0750); err != nil {
		errors.FatalError(errors.NewPermissionError(
			"Cannot create checkpoint directory",
			"Permission denied or insufficient disk space",
			"Check permissions on .cie/checkpoints/ or free up disk space",
			err,
		), false)
	}

	// Combine default excludes with user-specified ones
	defaults := ingestion.DefaultConfig()
	excludeGlobs := append(defaults.ExcludeGlobs, cfg.Indexing.Exclude...)

	config := ingestion.Config{
		ProjectID: cfg.ProjectID,
		RepoSource: ingestion.RepoSource{
			Type:  "local_path",
			Value: repoPath,
		},
		IngestionConfig: ingestion.IngestionConfig{
			ParserMode:           ingestion.ParserMode(cfg.Indexing.ParserMode),
			EmbeddingProvider:    embeddingProvider,
			BatchTargetMutations: cfg.Indexing.BatchTarget,
			MaxFileSizeBytes:     cfg.Indexing.MaxFileSize,
			CheckpointPath:       checkpointDir,
			ExcludeGlobs:         excludeGlobs,
			Concurrency: ingestion.ConcurrencyConfig{
				ParseWorkers: 4,
				EmbedWorkers: embedWorkers,
			},
		},
	}

	// Set embedding environment based on provider
	switch embeddingProvider {
	case "ollama":
		_ = os.Setenv("OLLAMA_BASE_URL", cfg.Embedding.BaseURL)
		_ = os.Setenv("OLLAMA_EMBED_MODEL", cfg.Embedding.Model)
	case "openai":
		_ = os.Setenv("OPENAI_API_BASE", cfg.Embedding.BaseURL)
		_ = os.Setenv("OPENAI_EMBED_MODEL", cfg.Embedding.Model)
		if cfg.Embedding.APIKey != "" {
			_ = os.Setenv("OPENAI_API_KEY", cfg.Embedding.APIKey)
		}
	}

	pipeline, err := ingestion.NewLocalPipeline(config, logger)
	if err != nil {
		errors.FatalError(errors.NewDatabaseError(
			"Cannot initialize indexing pipeline",
			"Failed to open or initialize the database",
			"Try 'cie reset' to rebuild the database, or close other CIE instances",
			err,
		), false)
	}
	defer func() { _ = pipeline.Close() }()

	// Set up progress reporting
	progressCfg := NewProgressConfig(globals)
	var currentBar *progressbar.ProgressBar
	var currentPhase string

	pipeline.SetProgressCallback(func(current, total int64, phase string) {
		// Create new bar when phase changes
		if phase != currentPhase {
			if currentBar != nil {
				_ = currentBar.Finish()
			}
			currentPhase = phase
			currentBar = NewProgressBar(progressCfg, total, phaseDescription(phase))
		}
		if currentBar != nil {
			_ = currentBar.Set64(current)
		}
	})

	logger.Info("indexing.starting",
		"mode", "local",
		"project_id", cfg.ProjectID,
		"repo_path", repoPath,
		"embedding_provider", embeddingProvider,
	)

	result, err := pipeline.Run(ctx)

	// Clean up progress bar
	if currentBar != nil {
		_ = currentBar.Finish()
	}

	if err != nil {
		errors.FatalError(errors.NewDatabaseError(
			"Indexing operation failed",
			"An error occurred during repository indexing",
			"Check the error details above. If this persists, try 'cie reset --force'",
			err,
		), false)
	}

	printResult(result)
}

// phaseDescription returns a human-readable description for each pipeline phase.
func phaseDescription(phase string) string {
	switch phase {
	case "parsing":
		return "Parsing files"
	case "embedding":
		return "Generating embeddings"
	case "embedding_types":
		return "Embedding types"
	case "writing":
		return "Writing to database"
	default:
		return phase
	}
}

// mapEmbeddingProvider maps user-facing provider names to internal identifiers.
//
// Maps:
//   - "ollama" → "ollama"
//   - "nomic" → "nomic"
//   - "mock" → "mock"
//   - unknown → "mock" (fallback for testing)
//
// Returns the internal provider identifier string.
func mapEmbeddingProvider(provider string) string {
	switch provider {
	case "ollama":
		return "ollama"
	case "nomic":
		return "nomic"
	case "openai":
		return "openai"
	case "mock":
		return "mock"
	default:
		return "mock"
	}
}

// printResult prints the indexing result summary to stdout.
//
// Displays statistics about files processed, functions extracted, embeddings generated,
// and overall execution time. Used to provide user feedback after indexing completes.
func printResult(result *ingestion.IngestionResult) {
	fmt.Println()
	fmt.Println("=== Indexing Complete ===")
	fmt.Printf("Project ID: %s\n", result.ProjectID)

	// Add progress indicators for file processing
	fmt.Printf("Files Processed: %d ", result.FilesProcessed)
	if result.ParseErrors > 0 {
		successRate := 100.0 * (1.0 - result.ParseErrorRate)
		fmt.Printf("(%.1f%% success rate)\n", successRate)
	} else {
		fmt.Println("✓")
	}

	fmt.Printf("Functions Extracted: %d\n", result.FunctionsExtracted)
	fmt.Printf("Types Extracted: %d\n", result.TypesExtracted)
	fmt.Printf("Defines Edges: %d\n", result.DefinesEdges)
	fmt.Printf("Calls Edges: %d\n", result.CallsEdges)
	fmt.Printf("Entities Written: %d\n", result.EntitiesSent)

	if result.ParseErrors > 0 {
		fmt.Printf("Parse Errors: %d (%.2f%%)\n", result.ParseErrors, result.ParseErrorRate)
	}
	if result.EmbeddingErrors > 0 {
		fmt.Printf("Embedding Errors: %d\n", result.EmbeddingErrors)
	}
	if result.CodeTextTruncated > 0 {
		fmt.Printf("CodeText Truncated: %d\n", result.CodeTextTruncated)
	}

	if len(result.TopSkipReasons) > 0 {
		fmt.Println("\nSkipped Files:")
		for reason, count := range result.TopSkipReasons {
			fmt.Printf("  %s: %d\n", reason, count)
		}
	}

	fmt.Println("\nTimings:")
	fmt.Printf("  Parse: %s\n", result.ParseDuration)
	fmt.Printf("  Embed: %s\n", result.EmbedDuration)
	fmt.Printf("  Write: %s\n", result.WriteDuration)
	fmt.Printf("  Total: %s\n", result.TotalDuration)
	fmt.Println()

	// Show data location
	homeDir, _ := os.UserHomeDir()
	fmt.Printf("Data stored in: %s\n", filepath.Join(homeDir, ".cie", "data", result.ProjectID))
}
