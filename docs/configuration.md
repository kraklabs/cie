# Configuration Reference

This document provides comprehensive reference for all CIE configuration options, environment variables, embedding providers, and advanced settings.

**What you'll learn:**
- How to configure CIE via `.cie/project.yaml`
- All available configuration options with defaults
- Environment variable reference
- Embedding and LLM provider setup
- Performance tuning and advanced settings

---

## Overview

CIE uses a three-tier configuration system with clear precedence:

```
1. Hardcoded Defaults (lowest priority)
   ↓
2. Configuration File (.cie/project.yaml)
   ↓
3. Environment Variables (highest priority)
```

**Configuration hierarchy:**
- Default values are defined in the CIE binary
- `.cie/project.yaml` overrides defaults
- Environment variables override everything

**Quick configuration check:**

```bash
# View effective configuration
cie config show

# Validate configuration
cie config validate
```

---

## Configuration File (.cie/project.yaml)

### Location and Discovery

CIE searches for configuration in this order:

1. Path specified by `CIE_CONFIG_PATH` environment variable
2. `.cie/project.yaml` in current directory
3. `.cie/project.yaml` in parent directories (walks up)

**Default location:**
```
your-project/
└── .cie/
    └── project.yaml
```

**Creating a configuration file:**

```bash
# Initialize with defaults (recommended)
cd your-project
cie init

# Manually create
mkdir -p .cie
cat > .cie/project.yaml << 'EOF'
version: "1"
project_id: "my-project"
# ... rest of config
EOF
```

### Schema Version

The current configuration schema version is **`"1"`**.

All configuration files must specify:
```yaml
version: "1"
```

Future schema changes will use different version numbers for compatibility.

---

## Configuration Schema

### Top-Level Structure

```yaml
version: "1"                 # Config schema version (required)
project_id: "my-project"     # Project identifier (required)

cie:                         # CIE server settings
  primary_hub: "..."
  edge_cache: "..."

embedding:                   # Embedding provider
  provider: "..."
  base_url: "..."
  model: "..."
  api_key: "..."

indexing:                    # Indexing behavior
  parser_mode: "..."
  batch_target: 500
  max_file_size: 1048576
  exclude: [...]

roles:                       # Custom role patterns (optional)
  custom:
    role_name:
      file_pattern: "..."
      name_pattern: "..."
      description: "..."

llm:                         # LLM for narratives (optional)
  enabled: false
  base_url: "..."
  model: "..."
  api_key: "..."
```

---

## Field Reference

### version

- **Type:** `string`
- **Required:** Yes
- **Default:** N/A
- **Values:** `"1"` (current schema version)
- **Description:** Configuration schema version. Must be `"1"` for current CIE versions.

**Example:**
```yaml
version: "1"
```

---

### project_id

- **Type:** `string`
- **Required:** Yes
- **Default:** N/A (generated from directory name during `cie init`)
- **Description:** Unique identifier for your project. Used for database file naming and project identification.

**Example:**
```yaml
project_id: "my-project"
```

**Naming recommendations:**
- Use lowercase letters, numbers, hyphens
- No spaces or special characters
- Keep it short and descriptive
- Example: `my-api`, `frontend-app`, `ml-pipeline`

---

### cie (CIE Server Configuration)

Configuration for CIE Primary Hub and Edge Cache servers.

#### cie.primary_hub

- **Type:** `string`
- **Required:** No
- **Default:** `"localhost:50051"`
- **Environment Override:** `CIE_PRIMARY_HUB`
- **Description:** gRPC address for write operations. Primary Hub handles all write requests and replication.

**Format:** `host:port` (no protocol prefix)

**Example:**
```yaml
cie:
  primary_hub: "localhost:50051"        # Local development
  # primary_hub: "cie-hub.example.com:50051"  # Production
```

#### cie.edge_cache

- **Type:** `string`
- **Required:** No
- **Default:** `"http://localhost:9090"` (detected automatically if Docker is running)
- **Environment Override:** `CIE_BASE_URL`
- **Description:** HTTP URL for read and write operations when delegating to a remote server. When using the Docker infrastructure, this should point to the CIE server.

**Format:** Full URL with protocol (`http://` or `https://`)

**Example:**
```yaml
cie:
  edge_cache: "http://localhost:9090"   # Local Docker server
```

**Architecture note:** In production deployments, Primary Hub handles writes while Edge Cache(s) handle reads. For local development, both typically run on localhost.

---

### embedding (Embedding Provider Configuration)

Configuration for the embedding provider used for semantic search.

#### embedding.provider

- **Type:** `string`
- **Required:** No
- **Default:** `"ollama"`
- **Values:** `"ollama"`, `"openai"`, `"nomic"`, `"llamacpp"`, `"mock"`
- **Description:** Embedding provider type. Determines which service generates vector embeddings for semantic search.

**Provider comparison:**

| Provider | Type | API Key Required | Performance | Use Case |
|----------|------|------------------|-------------|----------|
| `ollama` | Local | No | Fast | **Recommended** for development |
| `openai` | Cloud | Yes | Fast | Production with OpenAI API |
| `nomic` | Cloud | Yes | Fast | Production with Nomic Atlas |
| `llamacpp` | Local | No | Medium | Self-hosted llama.cpp server |
| `mock` | Test | No | Instant | Testing/CI only |

**Example:**
```yaml
embedding:
  provider: "ollama"  # Recommended for local development
```

#### embedding.base_url

- **Type:** `string`
- **Required:** No
- **Default:** Varies by provider:
  - Ollama: `"http://localhost:11434"`
  - OpenAI: `"https://api.openai.com/v1"`
  - Nomic: `"https://api-atlas.nomic.ai/v1"`
  - LlamaCpp: `"http://localhost:8090"`
- **Environment Override:** Provider-specific (see [Environment Variables](#environment-variables))
- **Description:** Base URL for the embedding API endpoint.

**Example:**
```yaml
embedding:
  base_url: "http://localhost:11434"  # Ollama default
```

#### embedding.model

- **Type:** `string`
- **Required:** No
- **Default:** Varies by provider:
  - Ollama: `"nomic-embed-text"`
  - OpenAI: `"text-embedding-3-small"`
  - Nomic: `"nomic-embed-text-v1.5"`
- **Environment Override:** Provider-specific (see [Environment Variables](#environment-variables))
- **Description:** Model name for embedding generation.

**Recommended models by provider:**

| Provider | Model | Dimensions | Notes |
|----------|-------|------------|-------|
| Ollama | `nomic-embed-text` | 384 | Best balance of speed/quality |
| Ollama | `mxbai-embed-large` | 1024 | Higher quality, slower |
| OpenAI | `text-embedding-3-small` | 1536 | Good quality, cost-effective |
| OpenAI | `text-embedding-3-large` | 3072 | Highest quality, expensive |
| Nomic | `nomic-embed-text-v1.5` | 768 | Asymmetric search optimized |

**Example:**
```yaml
embedding:
  model: "nomic-embed-text"  # Ollama model
```

#### embedding.api_key

- **Type:** `string`
- **Required:** Only for cloud providers (OpenAI, Nomic)
- **Default:** N/A
- **Environment Override:** `OPENAI_API_KEY`, `NOMIC_API_KEY`
- **Description:** API key for cloud embedding providers. Not needed for local providers (Ollama, LlamaCpp, Mock).

**Security best practice:** Use environment variables instead of storing keys in config file.

**Example:**
```yaml
embedding:
  api_key: "${OPENAI_API_KEY}"  # Reference env var
  # api_key: "sk-..."            # Avoid hardcoding keys
```

---

### indexing (Indexing Configuration)

Configuration for code indexing behavior.

#### indexing.parser_mode

- **Type:** `string`
- **Required:** No
- **Default:** `"auto"`
- **Values:** `"auto"`, `"treesitter"`
- **Description:** Code parser mode selection.
  - `"auto"`: Automatically selects best parser for each language
  - `"treesitter"`: Force Tree-sitter parser for all supported languages

**Example:**
```yaml
indexing:
  parser_mode: "auto"  # Recommended
```

**When to use `"treesitter"`:** Only if you want to enforce Tree-sitter parsing. The `"auto"` mode already uses Tree-sitter for Go, Python, JavaScript, and TypeScript.

#### indexing.batch_target

- **Type:** `integer`
- **Required:** No
- **Default:** `500`
- **Range:** `100` to `5000` (recommended)
- **Description:** Target number of mutations per batch when writing to CozoDB. Smaller batches improve network stability; larger batches improve throughput.

**Performance implications:**
- **Small (100-500):** More stable over slow networks, more round-trips
- **Medium (500-2000):** Balanced for most use cases
- **Large (2000-5000):** Maximum throughput, requires stable network

**Example:**
```yaml
indexing:
  batch_target: 500  # Conservative for development
  # batch_target: 2000  # Production with stable network
```

#### indexing.max_file_size

- **Type:** `integer`
- **Required:** No
- **Default:** `1048576` (1 MB)
- **Unit:** Bytes
- **Description:** Maximum file size to index. Files larger than this are skipped.

**Common values:**
- `1048576` (1 MB) - Default, good for most projects
- `5242880` (5 MB) - For projects with large generated files
- `524288` (512 KB) - For faster indexing, stricter filter

**Example:**
```yaml
indexing:
  max_file_size: 1048576  # 1 MB
```

**Performance note:** Larger files take longer to parse and generate more embeddings. If indexing is slow, consider lowering this value.

#### indexing.exclude

- **Type:** `array of strings`
- **Required:** No
- **Default:** See below
- **Description:** Glob patterns for files/directories to exclude from indexing.

**Default exclusions:**
```yaml
indexing:
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "dist/**"
    - "build/**"
    - "*.o"
    - "*.so"
    - "*.dylib"
    - "*.exe"
```

**Custom exclusions example:**
```yaml
indexing:
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "test/fixtures/**"      # Skip test data
    - "**/*.generated.go"     # Skip generated code
    - "**/*.pb.go"            # Skip protobuf files
    - "docs/**"               # Skip documentation
```

**Glob pattern syntax:**
- `*` - Matches any characters except `/`
- `**` - Matches any characters including `/`
- `?` - Matches single character
- `[...]` - Character class

**Performance tip:** Excluding large directories (like `node_modules`, generated files) significantly speeds up indexing.

---

### roles (Custom Role Configuration)

Optional configuration for custom role pattern matching.

#### roles.custom

- **Type:** `map[string]RolePattern`
- **Required:** No
- **Default:** Empty (no custom roles)
- **Description:** Define custom roles for your project's code organization. Roles help categorize functions for better semantic search and analysis.

**Built-in roles** (no configuration needed):
- `entry_point` - Main functions, init functions
- `router` - HTTP route definitions
- `handler` - HTTP request handlers
- `middleware` - Middleware functions
- `test` - Test functions

**Custom role structure:**
```yaml
roles:
  custom:
    role_name:
      file_pattern: "regex"     # Match file paths
      name_pattern: "regex"     # Match function names
      code_pattern: "regex"     # Match code content
      description: "string"     # Role explanation
```

**Example:**
```yaml
roles:
  custom:
    repository:
      file_pattern: ".*/repository/.*\\.go$"
      name_pattern: ".*Repository$"
      description: "Data access layer functions"

    service:
      file_pattern: ".*/service/.*\\.go$"
      name_pattern: ".*Service$"
      description: "Business logic services"

    validator:
      code_pattern: "validator|Validate"
      description: "Input validation functions"
```

**Use cases:**
- Domain-driven design layers (repository, service, controller)
- Microservice patterns (saga, circuit breaker)
- Project-specific conventions

---

### llm (LLM Configuration for Narrative Generation)

Optional configuration for LLM-powered narrative generation in `cie_analyze` tool.

#### llm.enabled

- **Type:** `boolean`
- **Required:** No
- **Default:** `false`
- **Environment Override:** `CIE_LLM_URL` (enables if set)
- **Description:** Enable LLM narrative generation for architectural analysis.

**Example:**
```yaml
llm:
  enabled: true
```

#### llm.base_url

- **Type:** `string`
- **Required:** If `enabled` is true
- **Default:** N/A
- **Environment Override:** `CIE_LLM_URL`
- **Description:** OpenAI-compatible API endpoint for LLM requests.

**Example:**
```yaml
llm:
  base_url: "http://localhost:11434"  # Ollama
  # base_url: "https://api.openai.com/v1"  # OpenAI
```

#### llm.model

- **Type:** `string`
- **Required:** If `enabled` is true
- **Default:** N/A
- **Environment Override:** `CIE_LLM_MODEL`
- **Description:** LLM model name for narrative generation.

**Recommended models:**

| Provider | Model | Context | Quality |
|----------|-------|---------|---------|
| Ollama | `llama2` | 4k | Good |
| Ollama | `mistral` | 8k | Better |
| Ollama | `codellama` | 16k | Code-optimized |
| OpenAI | `gpt-4o-mini` | 128k | Excellent |
| OpenAI | `gpt-4o` | 128k | Best |
| Anthropic | `claude-3-5-sonnet-20241022` | 200k | Excellent |

**Example:**
```yaml
llm:
  model: "llama2"  # Ollama local model
```

#### llm.api_key

- **Type:** `string`
- **Required:** Only for cloud providers
- **Default:** N/A
- **Environment Override:** `CIE_LLM_API_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`
- **Description:** API key for cloud LLM providers.

**Example:**
```yaml
llm:
  api_key: "${OPENAI_API_KEY}"  # Use env var (recommended)
```

#### llm.max_tokens

- **Type:** `integer`
- **Required:** No
- **Default:** `2000`
- **Range:** `100` to `4000`
- **Description:** Maximum tokens for LLM response.

**Example:**
```yaml
llm:
  max_tokens: 2000  # Default, good for summaries
```

---

## Environment Variables

Environment variables override configuration file values. Use them for:
- Per-environment configuration (dev, staging, prod)
- Secret management (API keys)
- CI/CD pipelines
- Docker deployments

### CIE Configuration Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CIE_CONFIG_PATH` | `string` | `.cie/project.yaml` | Explicit path to config file |
| `CIE_PROJECT_ID` | `string` | from config | Override project ID |
| `CIE_PRIMARY_HUB` | `string` | `localhost:50051` | Primary Hub gRPC address |
| `CIE_BASE_URL` | `string` | `http://localhost:8080` | Edge Cache HTTP URL |
| `CIE_LLM_URL` | `string` | — | Enable LLM, set base URL |
| `CIE_LLM_MODEL` | `string` | — | LLM model name |
| `CIE_LLM_API_KEY` | `string` | — | LLM API key |
| `CIE_SOFT_LIMIT_BYTES` | `integer` | `67108864` (64 MiB) | CozoDB script size limit |

### Ollama Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OLLAMA_HOST` | `string` | `http://localhost:11434` | Ollama server URL (embeddings) |
| `OLLAMA_BASE_URL` | `string` | `http://localhost:11434` | Alternative to OLLAMA_HOST |
| `OLLAMA_EMBED_MODEL` | `string` | `nomic-embed-text` | Embedding model name |
| `OLLAMA_MODEL` | `string` | `llama2` | LLM model name (for narratives) |

### OpenAI Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OPENAI_API_KEY` | `string` | — | **Required** for OpenAI |
| `OPENAI_API_BASE` | `string` | `https://api.openai.com/v1` | OpenAI API endpoint |
| `OPENAI_BASE_URL` | `string` | `https://api.openai.com/v1` | Alternative to API_BASE |
| `OPENAI_EMBED_MODEL` | `string` | `text-embedding-3-small` | Embedding model |
| `OPENAI_MODEL` | `string` | `gpt-4o-mini` | LLM model (for narratives) |

### Nomic Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `NOMIC_API_KEY` | `string` | — | **Required** for Nomic Atlas |
| `NOMIC_API_BASE` | `string` | `https://api-atlas.nomic.ai/v1` | Nomic API endpoint |
| `NOMIC_MODEL` | `string` | `nomic-embed-text-v1.5` | Embedding model |

### Anthropic Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ANTHROPIC_API_KEY` | `string` | — | **Required** for Anthropic |
| `ANTHROPIC_MODEL` | `string` | `claude-3-5-sonnet-20241022` | LLM model (for narratives) |

### LlamaCpp Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LLAMACPP_EMBED_URL` | `string` | `http://localhost:8090` | llama.cpp server endpoint |

---

## Embedding Providers

### Ollama (Recommended)

**Best for:** Local development, no API keys, good performance

**Prerequisites:**
```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull embedding model
ollama pull nomic-embed-text

# Verify
ollama list
```

**Configuration:**
```yaml
embedding:
  provider: "ollama"
  base_url: "http://localhost:11434"
  model: "nomic-embed-text"
```

**Alternative environment variables:**
```bash
export OLLAMA_HOST="http://localhost:11434"
export OLLAMA_EMBED_MODEL="nomic-embed-text"
cie index
```

**Recommended models:**

| Model | Size | Dimensions | Speed | Quality |
|-------|------|------------|-------|---------|
| `nomic-embed-text` | 274 MB | 384 |  | ⭐⭐⭐ |
| `mxbai-embed-large` | 669 MB | 1024 |  | ⭐⭐⭐⭐ |
| `all-minilm` | 45 MB | 384 |  | ⭐⭐ |

**Performance note:** `nomic-embed-text` provides the best balance. It supports **asymmetric search** (different encodings for documents vs queries).

---

### OpenAI

**Best for:** Production, cloud deployments, high quality

**Prerequisites:**
```bash
# Get API key from https://platform.openai.com/api-keys
export OPENAI_API_KEY="sk-..."
```

**Configuration:**
```yaml
embedding:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  model: "text-embedding-3-small"
  api_key: "${OPENAI_API_KEY}"  # Use env var
```

**Alternative environment variables:**
```bash
export OPENAI_API_KEY="sk-..."
export OPENAI_EMBED_MODEL="text-embedding-3-small"
cie index
```

**Model options:**

| Model | Dimensions | Cost (per 1M tokens) | Use Case |
|-------|------------|----------------------|----------|
| `text-embedding-3-small` | 1536 | $0.02 | **Recommended** balance |
| `text-embedding-3-large` | 3072 | $0.13 | Highest quality |
| `text-embedding-ada-002` | 1536 | $0.10 | Legacy (deprecated) |

**Cost estimation:**
- Small project (10k LOC): ~$0.05
- Medium project (100k LOC): ~$0.50
- Large project (1M LOC): ~$5.00

---

### Nomic

**Best for:** Production, specialized semantic search, Atlas platform

**Prerequisites:**
```bash
# Get API key from https://atlas.nomic.ai
export NOMIC_API_KEY="nk-..."
```

**Configuration:**
```yaml
embedding:
  provider: "nomic"
  base_url: "https://api-atlas.nomic.ai/v1"
  model: "nomic-embed-text-v1.5"
  api_key: "${NOMIC_API_KEY}"
```

**Alternative environment variables:**
```bash
export NOMIC_API_KEY="nk-..."
export NOMIC_MODEL="nomic-embed-text-v1.5"
cie index
```

**Features:**
- Asymmetric search optimized
- 8192 token context window
- Fast inference
- Atlas platform integration

---

### LlamaCpp/Qodo

**Best for:** Self-hosted, custom models, maximum control

**Prerequisites:**
```bash
# Install llama.cpp server
git clone https://github.com/ggerganov/llama.cpp
cd llama.cpp
make
./server -m models/qodo-embed-1-1.5b.gguf --port 8090
```

**Configuration:**
```yaml
embedding:
  provider: "llamacpp"
  base_url: "http://localhost:8090"
  model: "qodo-embed-1-1.5b"
```

**Alternative environment variables:**
```bash
export LLAMACPP_EMBED_URL="http://localhost:8090"
cie index
```

**Recommended model:**
- Qodo-Embed-1-1.5B (1536 dimensions, code-optimized)

---

### Mock Provider

**Best for:** Testing, CI/CD, development without embeddings

**Configuration:**
```yaml
embedding:
  provider: "mock"
```

**Features:**
- Deterministic embeddings (hash-based)
- No external dependencies
- Instant generation
- 384 dimensions

**Use cases:**
- Unit tests
- CI/CD pipelines without Ollama/API keys
- Development when semantic search not needed

**Limitation:** Mock embeddings are not semantically meaningful. Semantic search will not work correctly.

---

## LLM Providers (Narrative Generation)

LLM providers power the `cie_analyze` tool for architectural analysis with natural language summaries.

### Ollama (Recommended for Local)

**Configuration:**
```yaml
llm:
  enabled: true
  base_url: "http://localhost:11434"
  model: "llama2"
```

**Alternative environment variables:**
```bash
export CIE_LLM_URL="http://localhost:11434"
export CIE_LLM_MODEL="llama2"
cie analyze "What are the entry points?"
```

**Recommended models:**
- `llama2` - General purpose, 4k context
- `mistral` - Better quality, 8k context
- `codellama` - Code-optimized, 16k context

---

### OpenAI

**Configuration:**
```yaml
llm:
  enabled: true
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  api_key: "${OPENAI_API_KEY}"
```

**Alternative environment variables:**
```bash
export CIE_LLM_URL="https://api.openai.com/v1"
export CIE_LLM_MODEL="gpt-4o-mini"
export CIE_LLM_API_KEY="sk-..."
cie analyze "How does authentication work?"
```

---

### Anthropic

**Configuration:**
```yaml
llm:
  enabled: true
  base_url: "https://api.anthropic.com/v1"
  model: "claude-3-5-sonnet-20241022"
  api_key: "${ANTHROPIC_API_KEY}"
```

**Alternative environment variables:**
```bash
export CIE_LLM_URL="https://api.anthropic.com/v1"
export CIE_LLM_MODEL="claude-3-5-sonnet-20241022"
export ANTHROPIC_API_KEY="sk-ant-..."
cie analyze "Explain the replication architecture"
```

---

## Configuration Examples

### Minimal Configuration

Smallest valid config for local development with Ollama:

```yaml
version: "1"
project_id: "my-project"

embedding:
  provider: "ollama"
  model: "nomic-embed-text"
```

All other fields use defaults.

---

### Production Configuration

Full production config with OpenAI embeddings and LLM:

```yaml
version: "1"
project_id: "my-api-production"

cie:
  primary_hub: "cie-hub.example.com:50051"
  edge_cache: "https://cie-cache.example.com"

embedding:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  model: "text-embedding-3-small"
  api_key: "${OPENAI_API_KEY}"

indexing:
  parser_mode: "auto"
  batch_target: 2000  # Higher for stable network
  max_file_size: 5242880  # 5 MB for larger files
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "dist/**"
    - "**/*.pb.go"
    - "**/*.generated.go"

llm:
  enabled: true
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  api_key: "${OPENAI_API_KEY}"
  max_tokens: 3000
```

---

### Multi-Project Setup

Configuration for monorepo with custom roles:

```yaml
version: "1"
project_id: "monorepo"

embedding:
  provider: "ollama"
  model: "nomic-embed-text"

indexing:
  parser_mode: "auto"
  batch_target: 500
  max_file_size: 2097152  # 2 MB
  exclude:
    - ".git/**"
    - "**/node_modules/**"
    - "**/dist/**"
    - "packages/*/build/**"
    - "**/*.test.ts"
    - "**/*.test.go"

roles:
  custom:
    repository:
      file_pattern: ".*/repositories/.*\\.(go|ts)$"
      name_pattern: ".*Repository$"
      description: "Data access layer"

    service:
      file_pattern: ".*/services/.*\\.(go|ts)$"
      name_pattern: ".*Service$"
      description: "Business logic"

    api_handler:
      file_pattern: ".*/api/handlers/.*\\.(go|ts)$"
      description: "API request handlers"
```

---

### Docker Configuration

Configuration optimized for Docker deployment:

```yaml
version: "1"
project_id: "${PROJECT_ID}"

cie:
  primary_hub: "${CIE_PRIMARY_HUB:-cie-primary-hub:50051}"
  edge_cache: "${CIE_EDGE_CACHE:-http://cie-edge-cache:8080}"

embedding:
  provider: "${EMBEDDING_PROVIDER:-ollama}"
  base_url: "${OLLAMA_HOST:-http://ollama:11434}"
  model: "${OLLAMA_EMBED_MODEL:-nomic-embed-text}"
  api_key: "${EMBEDDING_API_KEY}"

indexing:
  parser_mode: "auto"
  batch_target: 1000
  max_file_size: 1048576
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
```

**Corresponding docker-compose.yml:**

```yaml
services:
  cie:
    image: kraklabs/cie:latest
    environment:
      PROJECT_ID: "my-project"
      CIE_PRIMARY_HUB: "cie-primary-hub:50051"
      CIE_EDGE_CACHE: "http://cie-edge-cache:8080"
      EMBEDDING_PROVIDER: "ollama"
      OLLAMA_HOST: "http://ollama:11434"
    volumes:
      - ./:/workspace
      - cie-data:/root/.cie

  ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama-data:/root/.ollama

volumes:
  cie-data:
  ollama-data:
```

---

### Custom Roles Configuration

Advanced config with detailed custom roles:

```yaml
version: "1"
project_id: "enterprise-api"

embedding:
  provider: "nomic"
  api_key: "${NOMIC_API_KEY}"

roles:
  custom:
    # Domain-driven design layers
    entity:
      file_pattern: ".*/domain/entities/.*\\.go$"
      name_pattern: "^(Create|Update|Delete).*$"
      description: "Domain entities and factories"

    repository:
      file_pattern: ".*/infrastructure/persistence/.*\\.go$"
      name_pattern: ".*Repository$"
      description: "Data access repositories"

    usecase:
      file_pattern: ".*/application/usecases/.*\\.go$"
      name_pattern: "Execute|Handle"
      description: "Use case implementations"

    # Cross-cutting concerns
    auth_middleware:
      code_pattern: "jwt|JWT|authentication|Authorization"
      file_pattern: ".*/middleware/.*\\.go$"
      description: "Authentication and authorization"

    validator:
      name_pattern: "Validate.*|.*Validator$"
      description: "Input validation logic"

    # Infrastructure
    grpc_handler:
      file_pattern: ".*/grpc/handlers/.*\\.go$"
      code_pattern: "pb\\.|protobuf"
      description: "gRPC service handlers"
```

---

## Advanced Options

### Parser Configuration

CIE uses Tree-sitter for code parsing.

**Supported languages:**
- Go (`.go`)
- Python (`.py`)
- JavaScript (`.js`)
- TypeScript (`.ts`, `.tsx`)

**Parser mode:**
```yaml
indexing:
  parser_mode: "auto"  # Recommended
  # parser_mode: "treesitter"  # Force Tree-sitter
```

**Language detection:** Automatic based on file extension. No configuration needed.

**Fallback behavior:** If Tree-sitter parsing fails, CIE logs a warning and skips the file.

---

### Exclusion Patterns

Exclude files from indexing using glob patterns.

**Performance recommendations:**

| Directory | Reason | Pattern |
|-----------|--------|---------|
| Version control | Not source code | `.git/**`, `.svn/**` |
| Dependencies | Third-party code | `node_modules/**`, `vendor/**` |
| Build artifacts | Generated files | `dist/**`, `build/**`, `bin/**` |
| IDE files | Editor metadata | `.idea/**`, `.vscode/**` |
| Generated code | Auto-generated | `**/*.pb.go`, `**/*.generated.*` |

**Advanced exclusion example:**
```yaml
indexing:
  exclude:
    # Version control
    - ".git/**"

    # Dependencies
    - "**/node_modules/**"
    - "**/vendor/**"
    - "**/third_party/**"

    # Build outputs
    - "**/dist/**"
    - "**/build/**"
    - "**/out/**"
    - "**/.next/**"
    - "**/.nuxt/**"

    # Generated code
    - "**/*.pb.go"
    - "**/*.generated.go"
    - "**/*.gen.ts"
    - "**/*.d.ts"

    # Test fixtures
    - "**/testdata/**"
    - "**/fixtures/**"

    # Minified files
    - "**/*.min.js"
    - "**/*.min.css"

    # Binary files
    - "**/*.o"
    - "**/*.so"
    - "**/*.dylib"
    - "**/*.exe"
    - "**/*.dll"
```

**Pattern testing:**
```bash
# Test exclusion pattern
echo "path/to/file.go" | grep -E "^(pattern)$"
```

---

### Performance Tuning

Optimize CIE performance for your use case.

#### Indexing Speed

**Factors affecting speed:**
1. File count and total LOC
2. Embedding provider latency
3. Batch size
4. CPU cores available

**Optimization tips:**

```yaml
indexing:
  # Increase batch size for faster indexing (requires stable network)
  batch_target: 2000

  # Reduce max file size to skip large files
  max_file_size: 524288  # 512 KB

  # Exclude directories aggressively
  exclude:
    - ".git/**"
    - "node_modules/**"
    - "vendor/**"
    - "**/*.min.js"
```

**Benchmark:**
```bash
time cie index
# Expected: 100k LOC in 30-60 seconds with Ollama
```

#### Memory Usage

**Memory consumption factors:**
- Number of functions indexed
- Embedding dimensions
- Call graph size

**Typical memory usage:**
- Small project (10k LOC): 100-200 MB
- Medium project (100k LOC): 500 MB - 1 GB
- Large project (1M LOC): 2-5 GB

**Reduce memory usage:**
```yaml
indexing:
  max_file_size: 524288  # Skip large files
  exclude:
    - "**/test/**"  # Skip tests if needed
```

#### Network Optimization

For remote Primary Hub / Edge Cache:

```yaml
cie:
  primary_hub: "cie-hub.example.com:50051"  # Use gRPC compression
  edge_cache: "https://cie-cache.example.com"

indexing:
  batch_target: 500  # Smaller batches for stability
```

---

### Storage Settings

CIE stores data in `.cie/db` (local mode) or connects to remote servers.

**Local storage:**
```
your-project/
└── .cie/
    ├── project.yaml
    └── db/              # CozoDB database files
        ├── cozo_data.db
        └── cozo_wal.db
```

**Storage size:**
- **Database:** ~10-50 KB per function (with embeddings)
- **Example:** 10k functions = ~200-500 MB

**Cleanup:**
```bash
# Remove index (keeps config)
rm -rf .cie/db

# Full cleanup
rm -rf .cie
```

**Backup:**
```bash
# Backup database
tar -czf cie-backup.tar.gz .cie/db
```

---

## Configuration Validation

### Command-Line Validation

```bash
# Validate configuration
cie config validate

# Show effective configuration (with env overrides)
cie config show

# Check specific setting
cie config get embedding.provider
```

**Expected output:**
```
OK Configuration is valid
  Version: 1
  Project: my-project
  Embedding: ollama (nomic-embed-text)
  Indexing: auto parser, 500 batch target
```

### Common Validation Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `version must be "1"` | Wrong schema version | Set `version: "1"` |
| `project_id is required` | Missing project ID | Add `project_id: "name"` |
| `unknown provider: xyz` | Invalid embedding provider | Use `ollama`, `openai`, `nomic`, `llamacpp`, or `mock` |
| `invalid YAML syntax` | YAML parsing error | Check indentation, quotes, colons |
| `file not found: .cie/project.yaml` | No config file | Run `cie init` or create file |

### Schema Validation

CIE validates configuration on load:

1. **Required fields:** `version`, `project_id`
2. **Type checking:** Integers, strings, booleans
3. **Enum validation:** Provider names, parser modes
4. **Range validation:** Batch target, max file size

**Manual validation with yamllint:**
```bash
yamllint .cie/project.yaml
```

---

## Troubleshooting

### Common Configuration Issues

#### Config File Not Found

**Symptom:**
```
Error: no .cie/project.yaml found in current or parent directories
```

**Solution:**
```bash
# Initialize CIE in project root
cie init

# Or specify config path
export CIE_CONFIG_PATH=/path/to/project.yaml
cie index
```

---

#### Invalid YAML Syntax

**Symptom:**
```
Error: yaml: line 12: mapping values are not allowed in this context
```

**Solution:**
- Check indentation (use spaces, not tabs)
- Verify quotes around strings with special characters
- Use yamllint for validation

**Example fix:**
```yaml
# Wrong (missing colon)
embedding
  provider: "ollama"

# Correct
embedding:
  provider: "ollama"
```

---

#### Environment Variable Not Recognized

**Symptom:**
```
Warning: Unknown environment variable CIE_INVALID_VAR
```

**Solution:**
- Check spelling against [Environment Variables](#environment-variables) table
- Verify variable name format (uppercase, underscores)
- Use `cie config show` to see effective config

---

#### Provider Connection Failure

**Symptom:**
```
Error: failed to connect to Ollama at http://localhost:11434
```

**Solution:**
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama
ollama serve

# Verify model is pulled
ollama list
ollama pull nomic-embed-text
```

---

#### Permission Errors

**Symptom:**
```
Error: permission denied: cannot write to .cie/db
```

**Solution:**
```bash
# Check directory permissions
ls -la .cie/

# Fix permissions
chmod -R 755 .cie/

# Check disk space
df -h .
```

---

### Debugging Configuration

**Enable verbose logging:**
```bash
cie --verbose config show
cie --verbose index
```

**Print effective configuration:**
```bash
# All settings
cie config show

# Specific field
cie config get embedding.provider
cie config get indexing.batch_target
```

**Environment variable precedence test:**
```bash
# Override in config file
cat .cie/project.yaml | grep batch_target
# Output: batch_target: 500

# Override with env var
export BATCH_TARGET=1000
cie config get indexing.batch_target
# Output: 1000
```

**Verify provider connection:**
```bash
# Test embedding provider
cie test embedding

# Expected output:
# OK Connected to Ollama at http://localhost:11434
# OK Model: nomic-embed-text
# OK Test embedding: 384 dimensions
```

**For more configuration troubleshooting issues, see the [Troubleshooting Guide](./troubleshooting.md).**

---

## Migration Guide

### Upgrading Configuration Schema

**Current version:** `"1"`

When upgrading to future schema versions:

1. **Check changelog** for breaking changes
2. **Backup configuration**
   ```bash
   cp .cie/project.yaml .cie/project.yaml.backup
   ```
3. **Run migration command** (future feature)
   ```bash
   cie config migrate --from 1 --to 2
   ```
4. **Validate new configuration**
   ```bash
   cie config validate
   ```

**Version compatibility:** CIE will warn about unsupported schema versions but may attempt to use defaults.

---

## Related Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](./getting-started.md) | Quick start guide and installation |
| [Tools Reference](./tools-reference.md) | Complete MCP tools documentation |
| [MCP Integration](./mcp-integration.md) | Setup with Claude Code, Cursor |
| [Architecture](./architecture.md) | How CIE works internally |
| [Troubleshooting](./troubleshooting.md) | Common issues and solutions |

---

## Quick Reference

**Essential commands:**
```bash
cie init                 # Create default config
cie start                # Start Docker infrastructure
cie stop                 # Stop infrastructure (preserves data)
cie reset --yes          # Delete indexed data
cie config show          # View effective config
cie config validate      # Check config validity
cie config get <field>   # Get specific value
```

**Config file location:**
```
.cie/project.yaml
```

**Environment precedence:**
```
Defaults < Config File < Environment Variables
```

**Get help:**
```bash
cie config --help
cie help
```

---

**Questions or issues?** See [Troubleshooting Guide](./troubleshooting.md) or file an issue on [GitHub](https://github.com/kraklabs/cie/issues).
