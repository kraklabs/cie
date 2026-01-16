<div align="center">
  <h1>🦑 CIE - Code Intelligence Engine</h1>
  <p><strong>Give your AI assistant deep understanding of your codebase</strong></p>

  [![CI](https://github.com/kraklabs/cie/actions/workflows/ci.yml/badge.svg)](https://github.com/kraklabs/cie/actions/workflows/ci.yml)
  [![codecov](https://codecov.io/gh/kraklabs/cie/branch/main/graph/badge.svg)](https://codecov.io/gh/kraklabs/cie)
  [![Go Report Card](https://goreportcard.com/badge/github.com/kraklabs/cie)](https://goreportcard.com/report/github.com/kraklabs/cie)
  [![Go Version](https://img.shields.io/github/go-mod/go-version/kraklabs/cie)](go.mod)
  [![License](https://img.shields.io/badge/license-AGPL%20v3-blue.svg)](LICENSE)

  <p>
    <a href="#quick-start">Quick Start</a> •
    <a href="#features">Features</a> •
    <a href="#documentation">Documentation</a> •
    <a href="#support">Support</a>
  </p>
</div>

---

CIE indexes your codebase and provides semantic search, call graph analysis, and AI-powered code understanding through the Model Context Protocol (MCP).

## Why CIE?

- 🔍 **Semantic Search** - Find code by meaning, not just text matching
- 🌳 **Call Graph Analysis** - Trace execution paths from entry points to any function
- 🤖 **MCP Native** - Works seamlessly with Claude Code, Cursor, and any MCP client
- ⚡ **Fast** - Indexes 100k LOC in seconds, queries in milliseconds
- 🔒 **Private** - All data stays local, your code never leaves your machine
- 🎯 **Accurate** - Keyword boosting ensures relevant results for function searches

## Features

### 🔍 Semantic Code Search

Find code by meaning, not keywords:

```bash
# Ask: "Where is authentication middleware?"
# Use cie_semantic_search tool via MCP
```

**Example output:**
```
🟢 88% match: AuthMiddleware (internal/http/auth.go:42)
🟡 76% match: ValidateToken (internal/auth/jwt.go:103)
```

### 🌳 Call Graph Analysis

Trace how execution reaches any function:

```bash
# Question: "How does main() reach database.Connect()?"
# Use cie_trace_path tool
```

**Example output:**
```
main → InitApp → SetupDatabase → database.Connect
  ├─ File: cmd/server/main.go:25
  ├─ File: internal/app/init.go:42
  └─ File: internal/database/setup.go:18
```

### 🎯 HTTP Endpoint Discovery

List all API endpoints automatically:

```bash
# Use cie_list_endpoints tool
```

**Example output:**
```
[GET]    /api/v1/users          → HandleGetUsers
[POST]   /api/v1/users          → HandleCreateUser
[DELETE] /api/v1/users/:id      → HandleDeleteUser
```

### 🌐 Multi-Language Support

Supports Go, Python, JavaScript, TypeScript, and more through Tree-sitter parsers.

## Quick Start

### Prerequisites

- **Go 1.24+** - [Download](https://go.dev/dl/)
- **CozoDB C library** (libcozo_c) - See [installation guide](#cozodb-installation)
- **Embedding provider** - Ollama (recommended), OpenAI, or Nomic

#### CozoDB Installation

**macOS:**
```bash
brew install cozo
```

**Linux:**
```bash
# Download from https://github.com/cozodb/cozo/releases
wget https://github.com/cozodb/cozo/releases/latest/download/libcozo_c-linux-x86_64.tar.gz
tar xzf libcozo_c-linux-x86_64.tar.gz
sudo cp libcozo_c.so /usr/local/lib/
```

**Windows:**
See [CozoDB releases](https://github.com/cozodb/cozo/releases) for DLL installation.

### Installation

```bash
# Install from source
git clone https://github.com/kraklabs/cie.git
cd kraken/modules/cie

# Build
make build

# Or install directly with go
go install github.com/kraklabs/cie/modules/cie/cmd/cie@latest
```

> **Note:** CIE is part of the Kraken monorepo. A standalone package is planned for easier installation.

### First Steps

```bash
# 1. Navigate to your project
cd /path/to/your/repo

# 2. Initialize CIE
cie init

# Output:
# ✓ Created .cie/project.yaml
# ✓ Project ID: your-repo-name
# ℹ Run 'cie index' to start indexing

# 3. Index the repository
cie index

# Output:
# Indexing... ████████████████████ 100%
# ✓ Indexed 1,234 files, 5,678 functions
# ⏱ Completed in 3.2s

# 4. Check status
cie status

# Output:
# Project: your-repo-name
# Files: 1,234
# Functions: 5,678
# Types: 890
# Last indexed: 2 minutes ago
```

### Common Issues

**"libcozo_c not found"** - Ensure CozoDB is installed and in your library path.
**"No embedding provider configured"** - Set `OLLAMA_HOST` or configure another provider in `.cie/project.yaml`.

### MCP Server Mode

CIE can run as an MCP server for integration with Claude Code:

```bash
cie --mcp
```

Configure in your Claude Code settings:

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/project/.cie/project.yaml"]
    }
  }
}
```

## Configuration

CIE uses a YAML configuration file (`.cie/project.yaml`):

```yaml
project_id: my-project

indexing:
  parser_mode: treesitter
  exclude:
    - "node_modules/**"
    - ".git/**"
    - "vendor/**"

embedding:
  provider: ollama
  base_url: http://localhost:11434
  model: nomic-embed-text
```

## MCP Tools

When running as an MCP server, CIE provides 20+ tools organized by category:

### Navigation & Search

| Tool | Description |
|------|-------------|
| `cie_grep` | Fast literal text search (no regex) |
| `cie_semantic_search` | Meaning-based search using embeddings |
| `cie_find_function` | Find functions by name (handles receiver syntax) |
| `cie_find_type` | Find types/interfaces/structs |
| `cie_find_similar_functions` | Find functions with similar names |
| `cie_list_files` | List indexed files with filters |
| `cie_list_functions_in_file` | List all functions in a file |

### Call Graph Analysis

| Tool | Description |
|------|-------------|
| `cie_find_callers` | Find what calls a function |
| `cie_find_callees` | Find what a function calls |
| `cie_trace_path` | Trace call paths from entry points to target |
| `cie_get_call_graph` | Get complete call graph for a function |

### Code Understanding

| Tool | Description |
|------|-------------|
| `cie_analyze` | Architectural analysis with LLM narrative (v1.1.0) |
| `cie_get_function_code` | Get function source code |
| `cie_directory_summary` | Get directory overview with main functions |
| `cie_find_implementations` | Find types that implement an interface |
| `cie_get_file_summary` | Get summary of all entities in a file |

### HTTP/API Discovery

| Tool | Description |
|------|-------------|
| `cie_list_endpoints` | List HTTP/REST endpoints from common Go frameworks |
| `cie_list_services` | List gRPC services and RPC methods from .proto files |

### Security & Verification

| Tool | Description |
|------|-------------|
| `cie_verify_absence` | Verify patterns don't exist (security audits) |

### System

| Tool | Description |
|------|-------------|
| `cie_index_status` | Check indexing health and statistics |
| `cie_search_text` | Regex-based text search in function code |
| `cie_raw_query` | Execute raw CozoScript queries |

> **📖 For detailed documentation of each tool with examples, see [Tools Reference](docs/tools-reference.md)** _(Coming soon)_

## Data Storage

CIE stores indexed data locally in `~/.cie/data/<project_id>/` using CozoDB with RocksDB backend. This ensures:

- Your code never leaves your machine
- Fast local queries
- Persistent index across sessions

## Embedding Providers

CIE supports multiple embedding providers:

| Provider | Configuration |
|----------|--------------|
| **Ollama** | `OLLAMA_HOST`, `OLLAMA_EMBED_MODEL` |
| **OpenAI** | `OPENAI_API_KEY`, `OPENAI_EMBED_MODEL` |
| **Nomic** | `NOMIC_API_KEY` |

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Step-by-step tutorial from installation to first query _(Coming soon)_ |
| [Configuration](docs/configuration.md) | Complete configuration reference _(Coming soon)_ |
| [Tools Reference](docs/tools-reference.md) | All 20+ MCP tools with examples _(Coming soon)_ |
| [Architecture](docs/architecture.md) | How CIE works internally _(Coming soon)_ |
| [MCP Integration](docs/mcp-integration.md) | Setting up with Claude Code, Cursor _(Coming soon)_ |
| [Testing Guide](docs/testing.md) | Running tests and adding new tests ✅ |
| [Benchmarks](docs/benchmarks.md) | Performance data and tuning ✅ |
| [Exit Codes](docs/exit-codes.md) | CLI exit codes for scripting ✅ |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions _(Coming soon)_ |

## Architecture

CIE uses a hybrid architecture combining static analysis with LLM-powered insights:

```
┌─────────────────────────────────────────────────────────────┐
│                         CLI / MCP Server                     │
└─────────────────┬───────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
┌───────▼────────┐  ┌───────▼────────┐
│   Ingestion    │  │  Query Engine  │
│   Pipeline     │  │   (Hybrid)     │
│                │  │                │
│ Tree-sitter → │  │ • Keyword      │
│ AST Analysis → │  │   Boost        │
│ Embedding     │  │ • Semantic     │
│ Generation    │  │   Search       │
└───────┬────────┘  └───────┬────────┘
        │                   │
        └─────────┬─────────┘
                  │
        ┌─────────▼─────────┐
        │  CozoDB (Datalog) │
        │  + RocksDB        │
        │  (~/.cie/data/)   │
        └───────────────────┘
```

**Key Components:**

- **Ingestion Pipeline**: Tree-sitter parsing → AST analysis → Embedding generation
- **Storage**: CozoDB (Datalog) + RocksDB (snapshots) for local-first data
- **Query Engine**: Hybrid search combining keyword boosting with semantic similarity
- **MCP Server**: JSON-RPC over stdio for AI tool integration

**Code Structure:**
```
cie/
├── cmd/cie/           # CLI tool with init, index, query commands
├── pkg/
│   ├── ingestion/     # Tree-sitter parsers and indexing pipeline
│   ├── tools/         # 20+ MCP tool implementations
│   ├── llm/           # LLM provider abstractions (OpenAI, Ollama)
│   ├── cozodb/        # CozoDB wrapper for Datalog queries
│   └── storage/       # Storage backend interface
└── docs/              # Documentation
```

For in-depth architecture details, see [Architecture Guide](docs/architecture.md) _(Coming soon)_

## Development

### Testing

CIE uses a two-tier testing approach:

**Unit Tests (default)** - Fast in-memory tests, no CozoDB installation required:
```bash
# Run all unit tests
go test ./...

# Run with short flag
go test -short ./...
```

**Integration Tests** - Use Docker containers with CozoDB:
```bash
# Build test container (first time only)
make docker-build-cie-test

# Run integration tests
go test -tags=cozodb ./...
```

The testcontainer infrastructure automatically handles:
- Building Docker images if missing
- Mounting project directories
- Cleaning up containers
- Graceful fallback if Docker unavailable

For detailed testing documentation, see [docs/testing.md](docs/testing.md).

### Writing Tests

Use the CIE testing helpers for easy test setup:

```go
import cietest "github.com/kraklabs/cie/internal/testing"

func TestMyFeature(t *testing.T) {
    backend := cietest.SetupTestBackend(t)
    cietest.InsertTestFunction(t, backend, "func1", "MyFunc", "file.go", 10, 20)

    result := cietest.QueryFunctions(t, backend)
    require.Len(t, result.Rows, 1)
}
```

### Building

```bash
# Build all commands
make build-all

# Format code
make fmt

# Run linter
make lint
```

## Support

Need help or want to contribute?

- 📖 **Documentation**: [docs/](docs/)
- 🐛 **Report Issues**: [GitHub Issues](https://github.com/kraklabs/cie/issues/new?labels=cie)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/kraklabs/cie/discussions)
- 📧 **Email**: support@kraklabs.com

**Before opening an issue:**
1. Check the [troubleshooting guide](docs/troubleshooting.md) _(Coming soon)_
2. Search [existing issues](https://github.com/kraklabs/cie/issues?q=label%3Acie)
3. Include CIE version: `cie --version`
4. Provide minimal reproduction steps

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

CIE is dual-licensed:

### Open Source License (AGPL v3)

CIE is free and open source under the **GNU Affero General Public License v3.0** (AGPL v3).

**Use CIE for free if:**
- You're building open source software
- You can release your modifications under AGPL v3
- You're okay with the copyleft requirements

See [LICENSE](LICENSE) for full AGPL v3 terms.

### Commercial License

Need to use CIE in a closed-source product or service? We offer commercial licenses that remove AGPL requirements.

**Commercial licensing is right for you if:**
- You want to use CIE in a proprietary product
- You want to offer CIE as a managed service without releasing your code
- Your organization's policies prohibit AGPL-licensed software
- You want to modify CIE without releasing your modifications

**Pricing:** Contact licensing@kraklabs.com for details.

See [LICENSE.commercial](LICENSE.commercial) for more information.

**Why dual licensing?**
This model allows us to:
- Keep CIE free for the open source community
- Ensure improvements benefit everyone through AGPL's copyleft
- Sustainably fund development through commercial licensing
- Enable enterprise adoption without legal concerns

### Third-Party Components

CIE includes some third-party components with their own licenses:
- **CozoDB C Headers** (MPL 2.0) - See [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md) for details

These components are compatible with AGPL v3 and retain their original licenses.

## Related Projects

- [CozoDB](https://github.com/cozodb/cozo) - The embedded database powering CIE
- [Tree-sitter](https://tree-sitter.github.io/) - Parser generator for code analysis
- [MCP](https://modelcontextprotocol.io/) - Model Context Protocol specification
