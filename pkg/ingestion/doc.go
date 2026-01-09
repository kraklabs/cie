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

// Package ingestion provides the code indexing pipeline for CIE.
//
// The ingestion package is responsible for parsing source code, extracting
// semantic information (functions, types, calls), generating embeddings,
// and storing the results in the CIE database for code intelligence queries.
//
// # Pipeline Overview
//
// The ingestion pipeline processes code in five stages:
//
//  1. Discovery: Find source files using configurable glob patterns
//  2. Parsing: Use Tree-sitter to parse code into ASTs
//  3. Extraction: Extract functions, types, and call relationships
//  4. Embedding: Generate vector embeddings for semantic search
//  5. Storage: Store entities and relationships in CozoDB
//
// Each stage is designed for reliability, performance, and resumability
// through checkpointing and incremental updates.
//
// # Supported Languages
//
// The following languages are fully supported with Tree-sitter parsing:
//   - Go (.go)
//   - Python (.py)
//   - TypeScript (.ts, .tsx)
//   - JavaScript (.js, .jsx)
//
// Additionally, Protocol Buffers (.proto) are supported via regex parsing.
//
// Each language parser extracts:
//   - Functions/methods with signatures and bodies
//   - Types, interfaces, classes, and structs
//   - Function call relationships
//   - File and package metadata
//
// # Quick Start
//
// Create and run a local indexing pipeline:
//
//	config := ingestion.Config{
//	    ProjectID: "my-project",
//	    RepoSource: ingestion.RepoSource{
//	        Type:  "git_url",
//	        Value: "https://github.com/user/repo.git",
//	    },
//	    IngestionConfig: ingestion.DefaultConfig(),
//	}
//
//	pipeline, err := ingestion.NewLocalPipeline(config, logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer pipeline.Close()
//
//	result, err := pipeline.Run(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Indexed %d files, %d functions\n",
//	    result.FilesProcessed, result.FunctionsExtracted)
//
// # Key Components
//
// LocalPipeline is the main entry point for indexing:
//
//	pipeline := ingestion.NewLocalPipeline(config, logger)
//	result, err := pipeline.Run(ctx)
//
// LocalPipeline orchestrates the entire 5-stage pipeline without requiring
// a Primary Hub, storing results in a local embedded CozoDB instance.
//
// Batcher splits large Datalog scripts into manageable chunks:
//
//	batcher := ingestion.NewBatcher(1000, 2*1024*1024)
//	batches, err := batcher.Batch(script)
//
// Batcher ensures scripts stay within CozoDB's size limits by splitting
// them into batches of ~1000 mutations or ~2MB each.
//
// CallResolver handles import resolution and cross-file references:
//
//	resolver := ingestion.NewCallResolver()
//	resolver.BuildIndex(files, functions, imports, packageNames)
//	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)
//
// CallResolver maps function calls across package boundaries, enabling
// accurate call graph construction.
//
// EmbeddingGenerator produces semantic embeddings concurrently:
//
//	embeddingGen := ingestion.NewEmbeddingGenerator(provider, concurrency, logger)
//	result, err := embeddingGen.EmbedFunctions(ctx, functions)
//
// Supports multiple providers: OpenAI, Nomic, Ollama, and Mock for testing.
//
// RepoLoader loads code from git repositories or local paths:
//
//	repoLoader := ingestion.NewRepoLoader(logger)
//	result, err := repoLoader.LoadRepository(repoSource, excludeGlobs, maxFileSizeBytes)
//	defer repoLoader.Close()  // Cleans up temp directories
//
// # Configuration
//
// The pipeline is configured through Config and IngestionConfig:
//
//	config := &ingestion.Config{
//	    ProjectID: "my-project",
//	    RepoSource: ingestion.RepoSource{
//	        Type:  "local_path",
//	        Value: "/path/to/code",
//	    },
//	    IngestionConfig: ingestion.IngestionConfig{
//	        ParserMode:        "auto",           // "treesitter", "simplified", "auto"
//	        EmbeddingProvider: "openai",         // "openai", "nomic", "ollama", "mock"
//	        MaxFileSizeBytes:  1024 * 1024,      // 1MB default
//	        MaxCodeTextBytes:  100 * 1024,       // 100KB default
//	        ExcludeGlobs: []string{
//	            "node_modules/**",
//	            ".git/**",
//	            "vendor/**",
//	        },
//	        Concurrency: struct {
//	            ParseWorkers int
//	            EmbedWorkers int
//	        }{
//	            ParseWorkers: 4,
//	            EmbedWorkers: 8,
//	        },
//	        LocalDataDir:         "~/.cie/data/my-project",
//	        LocalEngine:          "rocksdb",  // "rocksdb", "sqlite", "mem"
//	        BatchTargetMutations: 2000,
//	        WriteMode:            "bulk",     // "bulk" or "per_statement"
//	    },
//	}
//
// Use DefaultConfig() for sensible defaults.
//
// # Incremental Updates
//
// The package supports incremental indexing using file checksums and
// checkpointing. Only changed files are re-processed on subsequent runs:
//
//	// First run: indexes everything
//	result1, err := pipeline.Run(ctx)
//
//	// Second run: only processes changed files
//	result2, err := pipeline.Run(ctx)
//
// Checkpoints are saved automatically and can be configured via Config.CheckpointPath.
//
// # Metrics
//
// Indexing progress and statistics are available through the result:
//
//	result, err := pipeline.Run(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Files processed: %d\n", result.FilesProcessed)
//	fmt.Printf("Functions extracted: %d\n", result.FunctionsExtracted)
//	fmt.Printf("Types extracted: %d\n", result.TypesExtracted)
//	fmt.Printf("Parse errors: %d (%.1f%%)\n",
//	    result.ParseErrors, result.ParseErrorRate*100)
//	fmt.Printf("Total duration: %v\n", result.TotalDuration)
//
// Prometheus metrics are also exported for monitoring production systems.
package ingestion
