// Copyright 2025 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/kraklabs/cie/pkg/ingestion"
)

// BuildIngestionConfig собирает конфиг пайплайна индексации из конфига проекта.
// Используется и командой cie index, и MCP-реиндексом — одна точка правды для defaults и exclude.
func BuildIngestionConfig(cfg *Config, repoPath, dataDir, checkpointDir string, forceReindex bool, embedWorkers int) (ingestion.Config, string) {
	defaults := ingestion.DefaultConfig()
	excludeGlobs := append(defaults.ExcludeGlobs, cfg.Indexing.Exclude...)

	embedProvider := cfg.Embedding.Provider
	if embedProvider == "" {
		embedProvider = "ollama"
	}
	dim := cfg.Embedding.Dimensions
	if dim <= 0 {
		dim = 768
	}
	batchTarget := cfg.Indexing.BatchTarget
	if batchTarget <= 0 {
		batchTarget = 500
	}
	maxFileSize := cfg.Indexing.MaxFileSize
	if maxFileSize <= 0 {
		maxFileSize = 1024 * 1024
	}
	parserMode := ingestion.ParserMode(cfg.Indexing.ParserMode)
	if parserMode == "" {
		parserMode = ingestion.ParserModeAuto
	}
	if embedWorkers <= 0 {
		embedWorkers = 8
	}

	// Определяем режим детекции изменений: git или hash-based
	useGit := cfg.Indexing.UseGit
	if !useGit {
		// Если явно отключили в конфиге — проверим есть ли git
		// Hash-based режим работает с любой VCS или без VCS вообще
	}

	config := ingestion.Config{
		ProjectID: cfg.ProjectID,
		RepoSource: ingestion.RepoSource{
			Type:  "local_path",
			Value: repoPath,
		},
		IngestionConfig: ingestion.IngestionConfig{
			ParserMode:           parserMode,
			EmbeddingProvider:    embedProvider,
			EmbeddingDimensions:  dim,
			BatchTargetMutations: batchTarget,
			MaxFileSizeBytes:     maxFileSize,
			CheckpointPath:       checkpointDir,
			LocalDataDir:         dataDir,
			LocalEngine:          "rocksdb",
			ExcludeGlobs:         excludeGlobs,
			ForceReindex:         forceReindex,
			UseGitDelta:          useGit, // Передаём настройку из конфига
			Concurrency: ingestion.ConcurrencyConfig{
				ParseWorkers: 4,
				EmbedWorkers: embedWorkers,
			},
		},
	}
	return config, embedProvider
}
