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
	"time"
)

// Config holds configuration for the ingestion pipeline.
type Config struct {
	// ProjectID is the target project identifier in Primary Hub.
	ProjectID string

	// RepoSource specifies where to load the repository from.
	RepoSource RepoSource

	// IngestionConfig controls parsing, embedding, and batching behavior.
	IngestionConfig IngestionConfig
}

// RepoSource specifies the repository source.
type RepoSource struct {
	Type  string // "git_url" or "local_path"
	Value string // URL or local filesystem path
}

// IngestionConfig controls the ingestion pipeline behavior.
type IngestionConfig struct {
	// ParserMode specifies which parser to use: "treesitter", "simplified", or "auto".
	// Auto mode uses Tree-sitter if available, falling back to simplified parser.
	ParserMode ParserMode

	// LanguagesSupported is a list of language identifiers for Tree-sitter parsing.
	// Empty list means auto-detect from file extensions.
	LanguagesSupported []string

	// EmbeddingProvider specifies the embedding generation provider.
	// Options: "mock", "nomic", "ollama", "openai"
	// Environment variables per provider:
	//   - nomic: NOMIC_API_KEY (required), NOMIC_API_BASE, NOMIC_MODEL
	//   - ollama: OLLAMA_BASE_URL, OLLAMA_EMBED_MODEL
	//   - openai: OPENAI_API_KEY (required), OPENAI_API_BASE, OPENAI_EMBED_MODEL
	EmbeddingProvider string

	// BatchTargetMutations is the target number of mutations per ExecuteWrite batch.
	// Range: 500-2000. Actual batches may be smaller if approaching size limits.
	BatchTargetMutations int

	// MaxFileSizeBytes is the maximum file size to process (default: 1MB).
	// Files exceeding this are skipped with a warning.
	MaxFileSizeBytes int64

	// MaxCodeTextBytes is the maximum size for function code_text (default: 100KB).
	// CodeText exceeding this is truncated with a warning.
	MaxCodeTextBytes int64

	// ExcludeGlobs are glob patterns for files/directories to exclude.
	// Supports full glob syntax: *, **, ?, [abc], [a-z], [!abc]
	// Common patterns: ["node_modules/**", ".git/**", "dist/**", "vendor/**"]
	ExcludeGlobs []string

	// Concurrency controls worker pools.
	Concurrency ConcurrencyConfig

	// PrimaryHubAddr is the gRPC address of the Primary Hub.
	PrimaryHubAddr string

	// TLS configuration for gRPC connection to Primary Hub.
	// If nil, uses insecure connection (development only).
	TLSConfig *TLSConfig

	// Timeouts for gRPC operations.
	GRPCTimeout time.Duration

	// Retry configuration for transient failures.
	RetryConfig RetryConfig

	// EmbeddingRetry controls retry behavior specifically for embedding provider errors.
	// By default mirrors RetryConfig but can be tuned independently.
	EmbeddingRetry RetryConfig

	// ResumePolicy controls behavior when checkpoint exists but server state cannot be verified.
	// Options: "fail_fast", "force_reprocess", "trust_checkpoint"
	// Default: "fail_fast" (safest option)
	ResumePolicy ResumePolicy

	// ReplicationLogLimit is the maximum number of replication log entries to fetch per page.
	// Default: 10000. The pipeline will paginate automatically if needed.
	ReplicationLogLimit uint32

	// WriteMode controls how mutations are sent to Primary Hub.
	//  - "bulk": send batches produced by the batcher (default for throughput)
	//  - "per_statement": send one ExecuteWrite per individual statement for robustness
	WriteMode string

	// CheckpointPath is the directory for storing checkpoint files.
	// If empty, checkpoints are stored in the current working directory.
	CheckpointPath string

	// ForceReindex when true generates unique request_ids per run, allowing
	// complete re-indexing of a project even if it was previously indexed.
	// When false (default), request_ids are deterministic by batch position,
	// ensuring idempotent re-runs that skip already-committed batches.
	ForceReindex bool

	// === LOCAL MODE (Standalone/Open-Source) ===

	// LocalDataDir is the directory where local CozoDB stores its data.
	// Defaults to ~/.cie/data/<project_id>
	// Only used when running in local mode (without Primary Hub).
	LocalDataDir string

	// LocalEngine is the CozoDB storage engine for local mode.
	// Options: "rocksdb" (default), "sqlite", or "mem".
	LocalEngine string
}

// ConcurrencyConfig controls worker pool sizes.
type ConcurrencyConfig struct {
	ParseWorkers int // Number of parallel file parsers
	EmbedWorkers int // Number of parallel embedding generators
}

// RetryConfig controls retry behavior for gRPC calls.
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retries
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Multiplier     float64       // Backoff multiplier (exponential)
}

// TLSConfig holds TLS configuration for gRPC connections.
// Used for enterprise mode with Primary Hub. In standalone mode, this is ignored.
type TLSConfig struct {
	Enabled            bool   // Enable TLS
	CertFile           string // Client certificate file
	KeyFile            string // Client key file
	CAFile             string // CA certificate file for server verification
	InsecureSkipVerify bool   // Skip server certificate verification (development only)
}

// ResumePolicy controls behavior when checkpoint exists but server state cannot be verified.
type ResumePolicy string

const (
	// ResumePolicyFailFast fails immediately if server state cannot be verified.
	// This is the safest option but requires manual intervention.
	ResumePolicyFailFast ResumePolicy = "fail_fast"

	// ResumePolicyForceReprocess re-sends all batches if server state cannot be verified.
	// Relies on idempotency (:replace + request_id) to prevent duplicates.
	// Safe but potentially wasteful of resources.
	ResumePolicyForceReprocess ResumePolicy = "force_reprocess"

	// ResumePolicyTrustCheckpoint trusts the local checkpoint without server verification.
	// Skips batches marked as sent in checkpoint even if server state is unknown.
	// WARNING: Can lead to data loss if checkpoint is stale.
	ResumePolicyTrustCheckpoint ResumePolicy = "trust_checkpoint"
)

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() IngestionConfig {
	return IngestionConfig{
		ParserMode:           ParserModeAuto, // Use Tree-sitter if available
		LanguagesSupported:   []string{},     // Auto-detect
		EmbeddingProvider:    "mock",         // Safe default for testing
		BatchTargetMutations: 2000,           // Increased for fewer replication log entries (reduces edge CPU usage)
		MaxFileSizeBytes:     1048576,        // 1MB
		MaxCodeTextBytes:     102400,         // 100KB (balance between coverage and performance)
		ExcludeGlobs: []string{
			// Version control
			".git/**",
			// Dependencies
			"node_modules/**", "vendor/**",
			// Build outputs
			"dist/**", "build/**", "bin/**", "**/bin/**", "out/**",
			// IDE and editor
			".idea/**", ".vscode/**", "*.swp", "*.swo",
			// Next.js / React
			".next/**", ".nuxt/**",
			// CIE own files
			".cie/**",
			// Compiled binaries and objects
			"*.o", "*.so", "*.dylib", "*.exe", "*.dll", "*.a",
			// Large generated/cache files
			"*.pack", "*.pack.gz", "*.pack.old",
			// Common cache directories
			".cache/**", "coverage/**", "tmp/**", ".tmp/**",
			// Minified files (usually not useful to index)
			"*.min.js", "*.min.css",
			// Lock files (not code)
			"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum",
		},
		Concurrency:    ConcurrencyConfig{ParseWorkers: 4, EmbedWorkers: 8},
		PrimaryHubAddr: "localhost:50051",
		GRPCTimeout:    120 * time.Second, // Increased for large batches over slow networks
		RetryConfig: RetryConfig{
			MaxRetries:     5,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     5 * time.Second,
			Multiplier:     2.0,
		},
		EmbeddingRetry: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 200 * time.Millisecond,
			MaxBackoff:     2 * time.Second,
			Multiplier:     2.0,
		},
		ResumePolicy:        ResumePolicyFailFast, // Safest default: fail if can't verify server state
		ReplicationLogLimit: 10000,                // Default limit for pagination
		WriteMode:           "bulk",               // Default to bulk mode unless overridden
	}
}
