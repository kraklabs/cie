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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kraklabs/cie/pkg/storage"
)

// LocalPipeline orchestrates ingestion to a local CozoDB backend.
// This is the standalone/open-source version that doesn't require Primary Hub.
type LocalPipeline struct {
	config        Config
	logger        *slog.Logger
	repoLoader    *RepoLoader
	parser        CodeParser
	embeddingGen  *EmbeddingGenerator
	backend       *storage.EmbeddedBackend
	checkpointMgr *CheckpointManager
	datalogBuild  *DatalogBuilder
}

// IngestionResult summarizes the ingestion run.
type IngestionResult struct {
	// ProjectID is the unique identifier for the indexed project.
	ProjectID string

	// RunID is the unique identifier for this ingestion run (UUID).
	RunID string

	// FilesProcessed is the total number of source files successfully parsed.
	FilesProcessed int

	// FunctionsExtracted is the total number of functions/methods discovered.
	FunctionsExtracted int

	// TypesExtracted is the total number of types/classes/interfaces discovered.
	TypesExtracted int

	// DefinesEdges is the number of file-to-function relationships created.
	DefinesEdges int

	// CallsEdges is the number of function-to-function call relationships created.
	CallsEdges int

	// EntitiesSent is the total number of entities written to storage.
	EntitiesSent int

	// EntitiesRetried is the number of entities that required retry due to transient failures.
	EntitiesRetried int

	// LastCommittedIndex is the replication log index of the last committed write.
	LastCommittedIndex uint64

	// ParseErrors is the number of files that failed to parse.
	ParseErrors int

	// ParseErrorRate is the percentage of files that failed (0.0-1.0).
	ParseErrorRate float64

	// EmbeddingErrors is the number of functions/types that failed embedding generation.
	EmbeddingErrors int

	// CodeTextTruncated is the number of functions whose code was truncated due to size limits.
	CodeTextTruncated int

	// TopSkipReasons maps skip reasons to counts (e.g., "too_large": 5, "binary": 2).
	TopSkipReasons map[string]int

	// ParseDuration is the time spent parsing source files.
	ParseDuration time.Duration

	// EmbedDuration is the time spent generating embeddings.
	EmbedDuration time.Duration

	// WriteDuration is the time spent writing entities to storage.
	WriteDuration time.Duration

	// TotalDuration is the total time for the entire ingestion run.
	TotalDuration time.Duration
}

// parseFilesResult holds the aggregated results from parallel parsing.
type parseFilesResult struct {
	files           []FileEntity
	functions       []FunctionEntity
	types           []TypeEntity
	defines         []DefinesEdge
	definesTypes    []DefinesTypeEdge
	calls           []CallsEdge
	imports         []ImportEntity
	unresolvedCalls []UnresolvedCall
	packageNames    map[string]string
}

// NewLocalPipeline creates a new local ingestion pipeline.
func NewLocalPipeline(config Config, logger *slog.Logger) (*LocalPipeline, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create components
	repoLoader := NewRepoLoader(logger)

	// Create parser based on mode
	var parser CodeParser
	parserMode := config.IngestionConfig.ParserMode
	if parserMode == "" {
		parserMode = ParserModeAuto
	}

	switch parserMode {
	case ParserModeTreeSitter:
		logger.Info("parser.mode", "mode", "treesitter")
		parser = NewTreeSitterParser(logger)
	case ParserModeSimplified:
		logger.Info("parser.mode", "mode", "simplified")
		parser = NewParser(logger)
	case ParserModeAuto:
		tsParser := NewTreeSitterParser(logger)
		if tsParser != nil {
			logger.Info("parser.mode", "mode", "treesitter", "selected_by", "auto")
			parser = tsParser
		} else {
			logger.Info("parser.mode", "mode", "simplified", "selected_by", "auto", "reason", "treesitter_unavailable")
			parser = NewParser(logger)
		}
	default:
		logger.Warn("parser.mode.unknown", "mode", parserMode, "fallback", "treesitter")
		parser = NewTreeSitterParser(logger)
	}

	// Set max CodeText size from config
	if config.IngestionConfig.MaxCodeTextBytes > 0 {
		parser.SetMaxCodeTextSize(config.IngestionConfig.MaxCodeTextBytes)
	}

	// Create embedding provider
	embeddingProvider, err := CreateEmbeddingProvider(config.IngestionConfig.EmbeddingProvider, logger)
	if err != nil {
		return nil, fmt.Errorf("create embedding provider: %w", err)
	}
	embeddingGen := NewEmbeddingGenerator(embeddingProvider, config.IngestionConfig.Concurrency.EmbedWorkers, logger)

	// Create local backend
	backend, err := storage.NewEmbeddedBackend(storage.EmbeddedConfig{
		DataDir:   config.IngestionConfig.LocalDataDir,
		Engine:    config.IngestionConfig.LocalEngine,
		ProjectID: config.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("create local backend: %w", err)
	}

	// Ensure schema exists
	if err := backend.EnsureSchema(); err != nil {
		_ = backend.Close()
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Create HNSW indexes for semantic search
	if err := backend.CreateHNSWIndex(); err != nil {
		logger.Warn("hnsw.index.create.warning", "err", err)
		// Don't fail - HNSW is optional for basic functionality
	}

	// Checkpoint manager
	checkpointMgr := NewCheckpointManager(config.IngestionConfig.CheckpointPath)

	return &LocalPipeline{
		config:        config,
		logger:        logger,
		repoLoader:    repoLoader,
		parser:        parser,
		embeddingGen:  embeddingGen,
		backend:       backend,
		checkpointMgr: checkpointMgr,
		datalogBuild:  NewDatalogBuilder(),
	}, nil
}

// Close cleans up resources.
func (p *LocalPipeline) Close() error {
	var lastErr error
	if p.backend != nil {
		if err := p.backend.Close(); err != nil {
			lastErr = err
		}
	}
	if p.repoLoader != nil {
		if err := p.repoLoader.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// generateRunID generates a deterministic run ID for log correlation.
func (p *LocalPipeline) generateRunID(startTime time.Time) string {
	roundedTime := startTime.Truncate(time.Second)
	baseID := fmt.Sprintf("run-%s-%d", p.config.ProjectID, roundedTime.Unix())
	hash := sha256.Sum256([]byte(baseID))
	return hex.EncodeToString(hash[:16])
}

// Run executes the full local ingestion pipeline.
func (p *LocalPipeline) Run(ctx context.Context) (*IngestionResult, error) {
	startTime := time.Now()
	runID := p.generateRunID(startTime)
	p.logger.Info("local.ingestion.start", "project_id", p.config.ProjectID, "run_id", runID)

	// Step 1: Load repository
	p.logger.Info("local.ingestion.step.load_repo", "run_id", runID)
	loadResult, err := p.repoLoader.LoadRepository(
		p.config.RepoSource,
		p.config.IngestionConfig.ExcludeGlobs,
		p.config.IngestionConfig.MaxFileSizeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("load repository: %w", err)
	}

	// Sort files by path for deterministic processing
	sort.Slice(loadResult.Files, func(i, j int) bool {
		return loadResult.Files[i].Path < loadResult.Files[j].Path
	})

	// Step 2: Parse files and extract entities
	p.logger.Info("local.ingestion.step.parse_files", "run_id", runID, "file_count", len(loadResult.Files))
	parseStart := time.Now()

	parseWorkers := p.config.IngestionConfig.Concurrency.ParseWorkers
	if parseWorkers <= 0 {
		parseWorkers = 4
	}

	parseResult, parseErrors := p.parseFilesParallel(ctx, loadResult.Files, parseWorkers)

	parseDuration := time.Since(parseStart)
	codeTextTruncated := p.parser.GetTruncatedCount()

	allFiles := parseResult.files
	allFunctions := parseResult.functions
	allTypes := parseResult.types
	allDefines := parseResult.defines
	allDefinesTypes := parseResult.definesTypes
	allCalls := parseResult.calls
	allImports := parseResult.imports
	allUnresolvedCalls := parseResult.unresolvedCalls
	packageNames := parseResult.packageNames

	// Step 2b: Resolve cross-package calls
	if len(allUnresolvedCalls) > 0 {
		resolver := NewCallResolver()
		resolver.BuildIndex(allFiles, allFunctions, allImports, packageNames)
		resolvedCalls := resolver.ResolveCalls(allUnresolvedCalls)
		allCalls = append(allCalls, resolvedCalls...)

		p.logger.Info("local.ingestion.cross_package_calls.resolved",
			"local_calls", len(allCalls)-len(resolvedCalls),
			"cross_package_resolved", len(resolvedCalls),
		)
	}

	parseErrorRate := 0.0
	if len(loadResult.Files) > 0 {
		parseErrorRate = float64(parseErrors) / float64(len(loadResult.Files)) * 100.0
	}

	p.logger.Info("local.ingestion.parse.complete",
		"files", len(allFiles),
		"functions", len(allFunctions),
		"types", len(allTypes),
		"defines", len(allDefines),
		"calls", len(allCalls),
		"parse_errors", parseErrors,
		"code_text_truncated", codeTextTruncated,
		"duration_ms", parseDuration.Milliseconds(),
	)

	// Step 3: Generate embeddings for functions
	p.logger.Info("local.ingestion.step.generate_embeddings", "run_id", runID, "function_count", len(allFunctions))
	embedStart := time.Now()

	embedResult, err := p.embeddingGen.EmbedFunctions(ctx, allFunctions)
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
	}
	allFunctions = embedResult.Functions
	embeddingErrors := embedResult.ErrorCount

	embedDuration := time.Since(embedStart)
	p.logger.Info("local.ingestion.embeddings.functions.complete",
		"count", len(allFunctions),
		"errors", embeddingErrors,
		"duration_ms", embedDuration.Milliseconds(),
	)

	// Step 3b: Generate embeddings for types
	if len(allTypes) > 0 {
		p.logger.Info("local.ingestion.step.generate_type_embeddings", "run_id", runID, "type_count", len(allTypes))
		typeEmbedStart := time.Now()

		typeEmbedResult, err := p.embeddingGen.EmbedTypes(ctx, allTypes)
		if err != nil {
			return nil, fmt.Errorf("generate type embeddings: %w", err)
		}
		allTypes = typeEmbedResult.Types
		embeddingErrors += typeEmbedResult.ErrorCount

		typeEmbedDuration := time.Since(typeEmbedStart)
		p.logger.Info("local.ingestion.embeddings.types.complete",
			"count", len(allTypes),
			"errors", typeEmbedResult.ErrorCount,
			"duration_ms", typeEmbedDuration.Milliseconds(),
		)
		embedDuration += typeEmbedDuration
	}

	// Step 4: Validate entities
	p.logger.Info("local.ingestion.step.validate_entities")
	if err := ValidateEntities(allFiles, allFunctions, allDefines, allCalls); err != nil {
		return nil, fmt.Errorf("entity validation failed: %w", err)
	}

	// Step 5: Write to local CozoDB
	p.logger.Info("local.ingestion.step.write_local", "run_id", runID,
		"files", len(allFiles),
		"functions", len(allFunctions),
		"types", len(allTypes),
		"defines", len(allDefines),
		"calls", len(allCalls),
		"imports", len(allImports),
	)
	writeStart := time.Now()

	// Generate Datalog mutations
	mutations := p.datalogBuild.BuildMutationsWithTypes(
		allFiles,
		allFunctions,
		allTypes,
		allDefines,
		allDefinesTypes,
		allCalls,
		allImports,
	)

	// Execute mutations
	if err := p.backend.Execute(ctx, mutations); err != nil {
		return nil, fmt.Errorf("write to local db: %w", err)
	}

	writeDuration := time.Since(writeStart)
	totalDuration := time.Since(startTime)

	entitiesSent := len(allFiles) + len(allFunctions) + len(allTypes) +
		len(allDefines) + len(allDefinesTypes) + len(allCalls) + len(allImports)

	p.logger.Info("local.ingestion.write.complete",
		"entities_written", entitiesSent,
		"duration_ms", writeDuration.Milliseconds(),
	)

	// Build result
	result := &IngestionResult{
		ProjectID:          p.config.ProjectID,
		RunID:              runID,
		FilesProcessed:     len(allFiles),
		FunctionsExtracted: len(allFunctions),
		TypesExtracted:     len(allTypes),
		DefinesEdges:       len(allDefines),
		CallsEdges:         len(allCalls),
		EntitiesSent:       entitiesSent,
		EntitiesRetried:    0, // No retries in local mode
		LastCommittedIndex: 0, // No replication log in local mode
		ParseErrors:        parseErrors,
		ParseErrorRate:     parseErrorRate,
		EmbeddingErrors:    embeddingErrors,
		CodeTextTruncated:  codeTextTruncated,
		TopSkipReasons:     loadResult.SkipReasons,
		ParseDuration:      parseDuration,
		EmbedDuration:      embedDuration,
		WriteDuration:      writeDuration,
		TotalDuration:      totalDuration,
	}

	p.logger.Info("local.ingestion.complete",
		"project_id", p.config.ProjectID,
		"run_id", runID,
		"files", result.FilesProcessed,
		"functions", result.FunctionsExtracted,
		"types", result.TypesExtracted,
		"entities_written", result.EntitiesSent,
		"parse_errors", result.ParseErrors,
		"embedding_errors", result.EmbeddingErrors,
		"total_duration_ms", result.TotalDuration.Milliseconds(),
	)

	return result, nil
}

// parseFilesParallel parses files in parallel using a worker pool.
func (p *LocalPipeline) parseFilesParallel(ctx context.Context, files []FileInfo, numWorkers int) (*parseFilesResult, int) {
	if len(files) == 0 {
		return &parseFilesResult{packageNames: make(map[string]string)}, 0
	}

	// For small file sets, use sequential parsing
	if len(files) < 10 || numWorkers <= 1 {
		return p.parseFilesSequential(ctx, files)
	}

	jobs := make(chan int, len(files))

	type fileResult struct {
		index       int
		result      *ParseResult
		err         error
		packageName string
		filePath    string
	}
	resultsChan := make(chan fileResult, len(files))

	var errorCount int32

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				fileInfo := files[i]
				pr, err := p.parser.ParseFile(fileInfo)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
					p.logger.Warn("local.ingestion.parse_file.error", "path", fileInfo.Path, "err", err)
					resultsChan <- fileResult{index: i, err: err, filePath: fileInfo.Path}
					continue
				}

				resultsChan <- fileResult{
					index:       i,
					result:      pr,
					packageName: pr.PackageName,
					filePath:    fileInfo.Path,
				}
			}
		}()
	}

	for i := range files {
		jobs <- i
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	parseResults := make([]*ParseResult, len(files))
	packageNames := make(map[string]string)
	var mu sync.Mutex

	for fr := range resultsChan {
		if fr.err != nil {
			continue
		}
		parseResults[fr.index] = fr.result
		if fr.packageName != "" {
			mu.Lock()
			packageNames[fr.filePath] = fr.packageName
			mu.Unlock()
		}
	}

	result := &parseFilesResult{
		packageNames: packageNames,
	}
	for _, pr := range parseResults {
		if pr == nil {
			continue
		}
		result.files = append(result.files, pr.File)
		result.functions = append(result.functions, pr.Functions...)
		result.types = append(result.types, pr.Types...)
		result.defines = append(result.defines, pr.Defines...)
		result.definesTypes = append(result.definesTypes, pr.DefinesTypes...)
		result.calls = append(result.calls, pr.Calls...)
		result.imports = append(result.imports, pr.Imports...)
		result.unresolvedCalls = append(result.unresolvedCalls, pr.UnresolvedCalls...)
	}

	return result, int(errorCount)
}

// parseFilesSequential parses files sequentially.
func (p *LocalPipeline) parseFilesSequential(ctx context.Context, files []FileInfo) (*parseFilesResult, int) {
	result := &parseFilesResult{
		packageNames: make(map[string]string),
	}
	errorCount := 0

	for _, fileInfo := range files {
		select {
		case <-ctx.Done():
			return result, errorCount
		default:
		}

		pr, err := p.parser.ParseFile(fileInfo)
		if err != nil {
			errorCount++
			p.logger.Warn("local.ingestion.parse_file.error", "path", fileInfo.Path, "err", err)
			continue
		}

		result.files = append(result.files, pr.File)
		result.functions = append(result.functions, pr.Functions...)
		result.types = append(result.types, pr.Types...)
		result.defines = append(result.defines, pr.Defines...)
		result.definesTypes = append(result.definesTypes, pr.DefinesTypes...)
		result.calls = append(result.calls, pr.Calls...)
		result.imports = append(result.imports, pr.Imports...)
		result.unresolvedCalls = append(result.unresolvedCalls, pr.UnresolvedCalls...)
		if pr.PackageName != "" {
			result.packageNames[fileInfo.Path] = pr.PackageName
		}
	}

	return result, errorCount
}

// Backend returns the underlying storage backend.
func (p *LocalPipeline) Backend() *storage.EmbeddedBackend {
	return p.backend
}
