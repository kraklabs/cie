# CIE Ingestion Pipeline (F1.M1)

This package implements the initial full repository ingestion pipeline for CIE (Code Intelligence Engine).

## Overview

The ingestion pipeline performs a complete index of a repository into the CIE Primary Hub by:

1. **Loading** repository contents (git clone or local path)
2. **Parsing** code to extract functions (Tree-sitter for Go, Python, JS/TS)
3. **Extracting** relationships (defines edges and same-file calls edges)
4. **Generating** embeddings for function code
5. **Converting** everything into Datalog mutations
6. **Sending** mutations to Primary Hub via ExecuteWrite in batches

## Architecture

### Components

- **RepoLoader**: Clones/loads repository and applies exclude patterns
- **Parser**: Extracts functions and relationships from source code
- **EmbeddingGenerator**: Generates embeddings for function code
- **DatalogBuilder**: Converts entities into Datalog mutation scripts
- **Batcher**: Splits mutations into batches targeting ~500-2000 mutations
- **GRPCClient**: Sends batches to Primary Hub with retry logic
- **CheckpointManager**: Tracks progress for restartability
- **Pipeline**: Orchestrates the full ingestion process

### Data Model

The ingestion creates the following entities in CozoDB:

- **cie_file**: File entities (id, path, hash, language, size)
- **cie_function**: Function entities (id, name, signature, file_path, code_text, embedding, ranges)
- **cie_defines**: Edges from file to function (file defines function)
- **cie_calls**: Edges from caller to callee function

All IDs are deterministic and stable across re-runs for idempotency.

## Usage

### Command Line

```bash
./cie-ingestion \
  -project-id my-project \
  -repo-type local_path \
  -repo-value /path/to/repo \
  -primary-hub-addr localhost:50051 \
  -embedding-provider mock \
  -batch-target 1000
```

### Programmatic

```go
config := ingestion.Config{
    ProjectID: "my-project",
    RepoSource: ingestion.RepoSource{
        Type:  "local_path",
        Value: "/path/to/repo",
    },
    IngestionConfig: ingestion.DefaultConfig(),
}
config.IngestionConfig.PrimaryHubAddr = "localhost:50051"

pipeline, err := ingestion.NewPipeline(config, logger)
if err != nil {
    log.Fatal(err)
}
defer pipeline.Close()

result, err := pipeline.Run(ctx)
```

## Configuration

### Repository Source

- **git_url**: Clone from Git repository (shallow clone, depth=1)
- **local_path**: Read from local filesystem path

### Embedding Providers

- **mock**: Deterministic mock embeddings (384 dimensions, for testing)
- **nomic**: Nomic Atlas API for high-quality code embeddings
  - Requires: `NOMIC_API_KEY` environment variable
  - Optional: `NOMIC_API_BASE` (default: `https://api-atlas.nomic.ai/v1`)
  - Optional: `NOMIC_MODEL` (default: `nomic-embed-text-v1.5`)
- **ollama**: Local Ollama server for offline embedding generation
  - Optional: `OLLAMA_BASE_URL` (default: `http://localhost:11434`)
  - Optional: `OLLAMA_EMBED_MODEL` (default: `nomic-embed-text`)
- **openai**: OpenAI-compatible API (works with OpenAI, Azure OpenAI, Together AI, etc.)
  - Requires: `OPENAI_API_KEY` environment variable
  - Optional: `OPENAI_API_BASE` (default: `https://api.openai.com/v1`)
  - Optional: `OPENAI_EMBED_MODEL` (default: `text-embedding-3-small`)

### Batching

- **batch_target_mutations**: Target mutations per batch (500-2000)
- Batches are automatically split to stay under size limits (2MB soft, 4MB hard)

### Exclude Patterns

Default exclude patterns:
- `.git/**`
- `node_modules/**`
- `dist/**`, `build/**`
- `vendor/**`
- Binary files (`*.o`, `*.so`, `*.dylib`, `*.exe`)

Supports full glob syntax:
- `*` - matches any sequence of non-separator characters
- `**` - matches any sequence including directory separators (any depth)
- `?` - matches any single non-separator character
- `[abc]` - matches any character in the brackets
- `[a-z]` - matches any character in the range
- `[!abc]` or `[^abc]` - matches any character NOT in the brackets

### TLS/mTLS Configuration

For production deployments, configure TLS for secure gRPC communication:

**Via Environment Variables:**
```bash
export GRPC_TLS_ENABLED=true
export GRPC_TLS_CA_CERT=/path/to/ca.pem
export GRPC_TLS_CLIENT_CERT=/path/to/client.pem  # For mTLS
export GRPC_TLS_CLIENT_KEY=/path/to/client-key.pem  # For mTLS
export GRPC_TLS_SERVER_NAME=primaryhub.example.com
```

**Via Config:**
```go
config.IngestionConfig.TLSConfig = &ingestion.TLSConfig{
    Enabled:        true,
    CACertPath:     "/path/to/ca.pem",
    ClientCertPath: "/path/to/client.pem",  // For mTLS
    ClientKeyPath:  "/path/to/client-key.pem",  // For mTLS
    ServerName:     "primaryhub.example.com",
}
```

**TLS Modes:**
- **Insecure** (default in dev): No TLS, for development only
- **TLS with system CA**: Set `Enabled: true` only
- **TLS with custom CA**: Set `Enabled: true` and `CACertPath`
- **mTLS**: Set all of `Enabled`, `CACertPath`, `ClientCertPath`, `ClientKeyPath`

## Idempotency & Restartability

The ingestion is **idempotent** and **restartable**:

- **Deterministic IDs**: Entity IDs are derived from stable properties (path, name, signature, range)
- **:replace semantics**: Uses `:replace` in Datalog to overwrite existing entities cleanly
- **Checkpointing**: Progress is saved periodically to allow resume after crashes
- **Re-run safety**: Re-running ingestion on the same repo overwrites cleanly without duplicates
- **Server verification**: On resume, verifies committed batches against Primary Hub's replication log

### Resume Policies

Configure the resume policy via CLI flag `--resume-policy` or config:

```go
config.IngestionConfig.ResumePolicy = ingestion.ResumePolicyFailFast // or "force_reprocess", "trust_checkpoint"
```

- `fail_fast` (default, safest): Fails immediately if server state cannot be verified
- `force_reprocess`: Re-sends all batches; relies on idempotency to prevent duplicates
- `trust_checkpoint`: Trusts local checkpoint without server verification (risk of data loss)

### Checkpoint Format

Checkpoints are saved as JSON files: `checkpoint-{project_id}.json`

Contains:
- Last processed file
- Last committed log index
- Sent batch request IDs (for server verification)
- Counts of entities processed
- Timestamps

## Parser Implementation

### Tree-sitter Parser (Default)

The default parser uses **Tree-sitter** for accurate AST-based code parsing. This provides:

- **Precise function extraction** with correct line/column ranges
- **Complete signature extraction** including generics and type parameters
- **Same-file call graph extraction** (calls edges between functions in the same file)
- **Proper handling of**:
  - Nested functions and closures
  - Methods on structs/classes
  - Anonymous functions (lambda, arrow functions)
  - Error-tolerant parsing (continues even with syntax errors)

**Supported Languages:**
- Go (including generics)
- Python (including class methods, lambdas)
- JavaScript (function declarations, arrow functions, methods)
- TypeScript (all JS features + type annotations, interfaces)

### Parser Modes

Configure the parser mode via CLI flag `--parser-mode` or config:

```go
config.IngestionConfig.ParserMode = ingestion.ParserModeTreeSitter // or "simplified", "auto"
```

- `treesitter`: Use Tree-sitter (requires CGO)
- `simplified`: Use regex-based fallback parser
- `auto` (default): Use Tree-sitter if available, fallback to simplified

### Git Support

- **Git clone**: Implemented using `exec.Command("git", "clone")`
  - Uses shallow clone (--depth 1) for efficiency
  - Temporary directories are cleaned up automatically
  - Requires `git` to be available in PATH
  - **URL validation**: Git URLs are validated to prevent command injection attacks

### Embeddings

- **Multiple providers**: Mock, Nomic, Ollama, and OpenAI-compatible APIs
- **Normalized vectors**: All embeddings are normalized to unit length (L2 norm = 1.0)
- **Validation**: NaN and Inf values are rejected with clear error messages
- **Dimension consistency**: All functions in a file must have same embedding dimension

### Cross-Language

- **No cross-language call graph**: v1 focuses on single-language, same-file relationships
- **No import resolution**: Cannot resolve cross-file calls yet

## Observability

The pipeline logs progress at key stages:

- Repository loading (file count, language distribution)
- Parsing progress (files processed, functions extracted)
- Embedding generation (count, duration)
- Batch sending (batch number, committed log index)

Final summary includes:
- Total entities processed
- Timings for each phase
- Last committed log index

## Error Handling

- **Transient errors**: Automatically retried with exponential backoff
  - gRPC UNAVAILABLE, DEADLINE_EXCEEDED
  - Embedding provider errors
- **Permanent errors**: Fail immediately
  - INVALID_ARGUMENT, NOT_FOUND, FAILED_PRECONDITION
  - Datalog parse errors
- **Checkpointing**: Progress is saved periodically to allow resume

## Testing

Run tests:

```bash
go test ./internal/cie/ingestion/...
```

Test fixtures:
- Small test repositories in `test/fixtures/`
- Mock embedding provider for deterministic tests

## Future Work (F1.M2+)

- **Incremental ingestion**: Delta-by-commit updates (F1.M2)
- **Cross-file call resolution**: Resolve calls across files (requires import resolution)
- **Additional languages**: Rust, Java, C/C++, Ruby, etc. via Tree-sitter grammars
- **Worker pools**: Parallel parsing and embedding generation (partially implemented)
- **Metrics & Observability**: Prometheus metrics, OpenTelemetry tracing integration
- **Batch embedding**: Batch embedding API calls for better throughput
