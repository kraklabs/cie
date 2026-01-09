# CIE - Code Intelligence Engine

CIE is a powerful code intelligence tool that indexes your codebase and provides semantic search, call graph analysis, and AI-powered code understanding through the Model Context Protocol (MCP).

## Features

- **Semantic Code Search**: Find code by meaning, not just keywords
- **Call Graph Analysis**: Trace function calls and understand code flow
- **Multi-Language Support**: Go, Python, JavaScript, TypeScript, and more
- **MCP Integration**: Works with Claude Code and other MCP-compatible tools
- **Local Storage**: All data stays on your machine using CozoDB

## Quick Start

### Prerequisites

- Go 1.24+
- CozoDB C library (libcozo_c)
- Ollama (for embeddings) or another embedding provider

### Installation

```bash
# Clone the repository
git clone https://github.com/kraklabs/cie.git
cd cie

# Build
make build

# Or directly with go
go build -o cie ./cmd/cie
```

### Usage

```bash
# Initialize a project
cd /path/to/your/repo
cie init

# Index the repository
cie index

# Check status
cie status

# Query the index
cie query "?[name, file_path] := *cie_function { name, file_path }" --limit 10
```

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

When running as an MCP server, CIE provides these tools:

| Tool | Description |
|------|-------------|
| `cie_grep` | Fast literal text search |
| `cie_semantic_search` | Meaning-based search using embeddings |
| `cie_find_function` | Find functions by name |
| `cie_find_callers` | Find what calls a function |
| `cie_find_callees` | Find what a function calls |
| `cie_get_function_code` | Get function source code |
| `cie_list_endpoints` | List HTTP/REST endpoints |
| `cie_trace_path` | Trace call paths between functions |
| `cie_analyze` | Architectural analysis with AI |
| `cie_find_type` | Find types/interfaces/structs |
| `cie_find_implementations` | Find interface implementations |
| `cie_directory_summary` | Get directory overview |

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

## Architecture

```
cie
├── cmd/
│   ├── cie/           # CLI tool
│   ├── cie-agent/     # Autonomous agent
│   └── mcp-server/    # MCP server
├── internal/
│   ├── ingestion/     # Code indexing pipeline
│   ├── tools/         # MCP tool implementations
│   ├── agent/         # ReAct agent
│   ├── llm/           # LLM provider abstractions
│   ├── cozodb/        # CozoDB wrapper
│   └── storage/       # Storage backend
└── docs/              # Documentation
```

## Development

```bash
# Run tests
go test ./...

# Run tests with CozoDB (requires libcozo_c)
go test -tags=cozodb ./...

# Build all commands
make build-all

# Format code
make fmt

# Run linter
make lint
```

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

## Related Projects

- [CozoDB](https://github.com/cozodb/cozo) - The embedded database powering CIE
- [Tree-sitter](https://tree-sitter.github.io/) - Parser generator for code analysis
- [MCP](https://modelcontextprotocol.io/) - Model Context Protocol specification
