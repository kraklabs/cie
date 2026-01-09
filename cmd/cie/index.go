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
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/kraklabs/cie/pkg/ingestion"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func runIndex(args []string, configPath string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	full := fs.Bool("full", false, "Force full reindex")
	forceFullReindex := fs.Bool("force-full-reindex", false, "Delete checkpoint and reindex everything from scratch")
	embedWorkers := fs.Int("embed-workers", 8, "Number of parallel embedding workers")
	debug := fs.Bool("debug", false, "Enable debug logging")
	metricsAddr := fs.String("metrics-addr", "", "HTTP listen address for Prometheus metrics (empty to disable)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie index [options]

Indexes the current repository using configuration from .cie/project.yaml.
Data is stored locally in ~/.cie/data/<project_id>/

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
			srv := &http.Server{Addr: *metricsAddr, Handler: mux}
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
		fmt.Fprintf(os.Stderr, "Error: cannot get current directory: %v\n", err)
		os.Exit(1)
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

	runLocalIndex(ctx, logger, cfg, cwd, embeddingProvider, *embedWorkers)
}

// checkLocalData checks if the project has local indexed data.
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

func runLocalIndex(ctx context.Context, logger *slog.Logger, cfg *Config, repoPath, embeddingProvider string, embedWorkers int) {
	// Ensure checkpoint directory exists
	checkpointDir := filepath.Join(ConfigDir(repoPath), "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot create checkpoint directory: %v\n", err)
		os.Exit(1)
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
		os.Setenv("OLLAMA_BASE_URL", cfg.Embedding.BaseURL)
		os.Setenv("OLLAMA_EMBED_MODEL", cfg.Embedding.Model)
	case "openai":
		os.Setenv("OPENAI_API_BASE", cfg.Embedding.BaseURL)
		os.Setenv("OPENAI_EMBED_MODEL", cfg.Embedding.Model)
		if cfg.Embedding.APIKey != "" {
			os.Setenv("OPENAI_API_KEY", cfg.Embedding.APIKey)
		}
	}

	pipeline, err := ingestion.NewLocalPipeline(config, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: create pipeline: %v\n", err)
		os.Exit(1)
	}
	defer pipeline.Close()

	logger.Info("indexing.starting",
		"mode", "local",
		"project_id", cfg.ProjectID,
		"repo_path", repoPath,
		"embedding_provider", embeddingProvider,
	)

	result, err := pipeline.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: indexing failed: %v\n", err)
		os.Exit(1)
	}

	printResult(result)
}

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

func printResult(result *ingestion.IngestionResult) {
	fmt.Println()
	fmt.Println("=== Indexing Complete ===")
	fmt.Printf("Project ID: %s\n", result.ProjectID)
	fmt.Printf("Files Processed: %d\n", result.FilesProcessed)
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
