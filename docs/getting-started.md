# Getting Started with CIE

This guide will help you install CIE, index your first codebase, and start using it with Claude Code or Cursor in under 5 minutes. Whether you're new to code intelligence tools or experienced with AI-assisted development, you'll be productive quickly.

**What you'll learn:**
- How to install CIE on your system
- How to index your first codebase
- How to use CIE with AI assistants via MCP
- Essential commands and workflows

---

## Prerequisites

Before installing CIE, ensure you have these requirements:

| Requirement | Version | Purpose | Verification |
|-------------|---------|---------|--------------|
| **Go** | 1.24+ | Building CIE | `go version` |
| **CozoDB C library** | Latest | Graph storage backend | See installation below |
| **Embedding Provider** | — | Semantic search | Ollama (recommended), OpenAI, or Nomic |
| **Git** | Any recent | Repository indexing | `git --version` |

### Quick Verification

Check if you have the prerequisites:

```bash
# Check Go version (need 1.24+)
go version

# Check Git
git --version

# Check if CozoDB library is installed (macOS)
ls /opt/homebrew/lib/libcozo_c.dylib

# Check if Ollama is running (if using Ollama)
curl http://localhost:11434/api/tags
```

### Installing Prerequisites

**CozoDB C Library:**

The CozoDB library provides the graph database backend for CIE. Installation varies by platform:

- **macOS**: `brew install cozo`
- **Linux**: Download from [CozoDB releases](https://github.com/cozodb/cozo/releases), extract, and copy `libcozo_c.so` to `/usr/local/lib/`
- **Windows**: Download DLL from releases

For detailed instructions, see the [README installation section](../README.md#quick-start).

**Embedding Provider (Ollama - Recommended):**

Ollama provides local embeddings without API keys:

```bash
# Install Ollama (macOS/Linux)
curl https://ollama.ai/install.sh | sh

# Pull the embedding model
ollama pull nomic-embed-text

# Verify Ollama is running
ollama list
```

---

## Installation

Choose the installation method that works best for you:

### Option 1: Go Install (Quickest) ⚡

If you have Go 1.24+ and the CozoDB library installed:

```bash
# Install CIE
go install github.com/kraklabs/kraken/modules/cie/cmd/cie@latest

# Verify installation
cie version
```

**Expected output:**
```
CIE v0.1.0 (commit: abc1234, built: 2026-01-13)
Go version: go1.24.0
```

### Option 2: Build from Source 🛠️

For contributors or if you want the latest development version:

```bash
# Clone the repository
git clone https://github.com/kraklabs/kraken.git
cd kraken/modules/cie

# Build CIE
make build

# The binary will be at ./bin/cie
./bin/cie version

# Optionally, install to your PATH
sudo mv ./bin/cie /usr/local/bin/
```

See [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed build instructions.

### Option 3: Docker 🐳

Run CIE in a container for isolation:

```bash
# Clone the repository
git clone https://github.com/kraklabs/kraken.git
cd kraken/modules/cie

# Start CIE with Docker Compose
docker-compose up -d

# Access the CIE shell
docker-compose exec cie-shell bash

# Inside the container, use CIE normally
cie version
```

The docker-compose setup includes Ollama and all dependencies.

### Option 4: Homebrew 🍺

**Coming Soon** - We're working on a Homebrew formula for easier installation.

---

## Your First Project

Let's index a real codebase and verify CIE is working. This should take less than 5 minutes.

### Step 1: Initialize CIE

Navigate to a project directory and initialize CIE:

```bash
# Go to your project (or use a test project)
cd ~/code/my-project

# Initialize CIE
cie init
```

**What happens:**
- Creates `.cie/` directory
- Generates `.cie/project.yaml` configuration file
- Sets up default embedding provider (Ollama)

**Expected output:**
```
✓ Created .cie directory
✓ Generated project.yaml
✓ Configured Ollama embeddings (nomic-embed-text)

Project initialized! Next steps:
  1. Review .cie/project.yaml (optional)
  2. Run: cie index
```

### Step 2: Configure Embeddings (Optional)

By default, CIE uses Ollama with `nomic-embed-text`. If you want to use a different provider, edit `.cie/project.yaml`:

```yaml
embedding:
  provider: "ollama"  # Options: ollama, openai, nomic
  model: "nomic-embed-text"
  base_url: "http://localhost:11434"
  # For OpenAI:
  # provider: "openai"
  # model: "text-embedding-3-small"
  # api_key_env: "OPENAI_API_KEY"
```

**Tip:** Ollama is the easiest option because it runs locally and doesn't require API keys.

### Step 3: Index Your Code

Run the indexing process:

```bash
cie index
```

**What happens:**
- Parses code files using Tree-sitter (Go, Python, TypeScript)
- Extracts functions, types, and call graphs
- Generates embeddings for semantic search
- Stores everything in CozoDB graph database

**Expected output:**
```
⠿ Indexing codebase...
  ├─ Parsing files: 142 files found
  ├─ Go: 89 files (2,341 functions)
  ├─ Python: 31 files (678 functions)
  ├─ TypeScript: 22 files (412 functions)
  ├─ Generating embeddings: 3,431 functions
  └─ Building call graph: 8,921 relationships

✓ Indexing completed in 34.2s
  Database size: 42.3 MB
  Functions indexed: 3,431
  Call graph edges: 8,921
```

**Performance note:** Indexing speed depends on:
- Number of files/functions
- Embedding provider latency
- Available CPU cores

Typical performance: 100k LOC in ~30-60 seconds.

### Step 4: Verify the Index

Check the index status and try a basic query:

```bash
# View index status
cie status
```

**Expected output:**
```
Project: my-project
Status: ✓ Indexed
Last indexed: 2026-01-13 10:42:31
Database: /Users/user/code/my-project/.cie/db

Statistics:
  Files: 142
  Functions: 3,431
  Types: 287
  Call graph edges: 8,921
  Database size: 42.3 MB
```

**Try a semantic search:**

```bash
# Find authentication-related code
cie query "authentication middleware"
```

**Example output:**
```
🔍 Semantic search results (top 5):

1. [95% match] AuthMiddleware
   File: internal/http/middleware/auth.go:42
   Signature: func AuthMiddleware(next http.Handler) http.Handler

2. [89% match] ValidateJWT
   File: internal/auth/jwt.go:78
   Signature: func (s *Service) ValidateJWT(token string) (*Claims, error)

3. [84% match] RequireAuth
   File: internal/http/middleware/auth.go:103
   Signature: func RequireAuth(roles ...Role) func(http.Handler) http.Handler

(showing 3 of 5 results)
```

**Success!** 🎉 You've indexed your first codebase with CIE.

---

## Basic Usage

### Essential CLI Commands

| Command | Description | Example |
|---------|-------------|---------|
| `cie init` | Initialize CIE in a project | `cie init` |
| `cie index` | Index or reindex the codebase | `cie index` |
| `cie status` | Show index statistics | `cie status` |
| `cie query <text>` | Semantic search | `cie query "error handling"` |
| `cie grep <pattern>` | Fast literal text search | `cie grep "func main"` |
| `cie find-function <name>` | Find function by name | `cie find-function "NewServer"` |
| `cie trace-path --target=<fn>` | Trace call paths | `cie trace-path --target=HandleRequest` |
| `cie list-endpoints` | List HTTP endpoints | `cie list-endpoints` |
| `cie --mcp` | Start MCP server mode | `cie --mcp` |
| `cie version` | Show version info | `cie version` |
| `cie help` | Show all commands | `cie help` |

### MCP Server Mode

CIE works as an MCP (Model Context Protocol) server, providing 20+ tools to AI assistants like Claude Code and Cursor.

**Start MCP server:**

```bash
cie --mcp
```

**Expected output:**
```
🦑 CIE MCP Server
Listening on stdio for MCP requests...
Project: my-project (/Users/user/code/my-project)
Tools available: 23

Ready for AI assistant connections.
```

The server communicates via JSON-RPC over stdio and provides tools like:
- `cie_semantic_search` - Find code by meaning
- `cie_find_function` - Locate functions
- `cie_get_call_graph` - Analyze dependencies
- `cie_list_endpoints` - Discover HTTP routes
- ...and 19 more tools

For setup instructions with Claude Code or Cursor, see [MCP Integration Guide](./mcp-integration.md) (coming in T053).

### Query Examples

**1. Semantic search for concepts:**

```bash
cie query "database connection pooling"
```

Finds functions related to database connections, even if they don't contain those exact words.

**2. Find exact text patterns:**

```bash
cie grep ".GET("
```

Fast literal search across all indexed files.

**3. Find function definition:**

```bash
cie find-function "BuildRouter"
```

Locates the function and shows its signature and location.

**4. Trace execution paths:**

```bash
cie trace-path --target="SaveUser"
```

Shows how execution reaches `SaveUser` from entry points (like `main`).

**5. List all HTTP endpoints:**

```bash
cie list-endpoints
```

Discovers and lists all REST API routes in your codebase.

---

## Common Workflows

### Daily Development Workflow

When actively developing, keep your index up to date:

```bash
# Make code changes
git commit -m "Add new feature"

# Update the index (incremental, fast)
cie index

# Query your new code
cie query "new feature logic"
```

**Tip:** CIE detects file changes and only reindexes modified files, making updates fast (~1-5 seconds).

### Working with Multiple Projects

CIE creates a `.cie` directory in each project. Simply `cd` to different projects:

```bash
# Project A
cd ~/code/project-a
cie index

# Project B
cd ~/code/project-b
cie index

# Each project has independent index
```

No global configuration needed. Each project is self-contained.

### Updating the Index

**When to reindex:**
- After pulling new changes: `git pull && cie index`
- After switching branches: `git checkout feature && cie index`
- After significant refactoring
- When adding new files or packages

**Full reindex (if needed):**

```bash
# Clean and rebuild the entire index
rm -rf .cie/db
cie index
```

This is rarely needed. Incremental indexing handles most cases.

---

## Troubleshooting

Common issues and quick fixes:

| Issue | Symptom | Solution |
|-------|---------|----------|
| **CozoDB library not found** | `error while loading shared libraries: libcozo_c.so` | Install CozoDB library: `brew install cozo` (macOS) or copy to `/usr/local/lib` (Linux) |
| **Ollama connection failed** | `failed to connect to Ollama at http://localhost:11434` | Start Ollama: `ollama serve` or check if running: `ollama list` |
| **Slow indexing** | Takes >5 minutes for small project | Check embedding provider latency. Ollama is fastest (local). |
| **Index corruption** | `database file is locked` or similar | Stop all CIE processes, then reindex: `rm -rf .cie/db && cie index` |
| **No functions found** | `Functions indexed: 0` | Check file extensions. Supported: `.go`, `.py`, `.js`, `.ts`, `.tsx`. |
| **Permission denied** | `cannot create directory .cie` | Check write permissions in project directory: `ls -la` |

**Still having issues?** See the [Troubleshooting Guide](./troubleshooting.md) (coming in T054) or file an issue on [GitHub](https://github.com/kraklabs/kraken/issues).

---

## Next Steps

Now that you have CIE running, explore these resources:

| Documentation | What You'll Learn |
|---------------|-------------------|
| **[Configuration Guide](./configuration.md)** (coming in T050) | All configuration options, advanced settings, multiple embedding providers |
| **[Tools Reference](./tools-reference.md)** (coming in T051) | Complete documentation of all 20+ MCP tools with examples |
| **[MCP Integration](./mcp-integration.md)** (coming in T053) | Setup CIE with Claude Code, Cursor, and other AI assistants |
| **[Architecture](./architecture.md)** (coming in T052) | How CIE works internally: indexing pipeline, graph storage, embeddings |
| **[Testing](./testing.md)** | How to run tests, write new tests, and contribute test coverage |
| **[Contributing](../CONTRIBUTING.md)** | Build from source, development workflow, submitting PRs |

**Quick links:**
- **Main README**: [../README.md](../README.md)
- **GitHub Issues**: [Report bugs or request features](https://github.com/kraklabs/kraken/issues)
- **License**: [AGPL v3](../LICENSE) (dual licensing available)

---

**Questions or feedback?** File an issue or discussion on GitHub. We'd love to hear about your experience!
