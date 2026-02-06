# Getting Started with CIE

This guide will help you install CIE, index your first codebase, and start using it with Claude Code or Cursor in under 5 minutes.

**What you'll learn:**
- How to install the CIE CLI
- How to index your first codebase
- How to verify everything is working

---

## Prerequisites

Before installing CIE, ensure you have these requirements:

- **Git**: For cloning repositories and indexing code.
- **Go 1.24+**: Only if you plan to build the CLI from source.
- Optional: **Ollama** for semantic search embeddings (see [Optional: Enable Semantic Search](#optional-enable-semantic-search)).

---

## Installation

Choose the installation method that works best for you:

### Option 1: Homebrew (Recommended)

The easiest way to install CIE on macOS or Linux:

```bash
brew tap kraklabs/cie
brew install cie
```

### Option 2: Install Script

Download the correct binary for your OS and architecture:

```bash
curl -sSL https://raw.githubusercontent.com/kraklabs/cie/main/install.sh | sh
```

### Option 3: Build from Source

If you have Go 1.24 installed, you can build the CLI yourself. Our `Makefile` automatically handles all dependencies, including the CozoDB C library:

```bash
# Clone the repository
git clone https://github.com/kraklabs/cie.git
cd cie

# Build CIE
make build

# The binary will be at ./bin/cie
# You can move it to your PATH
sudo mv ./bin/cie /usr/local/bin/
```

---

## Your First Project

Follow these steps to index a codebase and start using CIE.

### Step 1: Initialize CIE

Navigate to your project directory and initialize CIE:

```bash
cd /path/to/your/project
cie init -y
```

**What happens:**
- Creates a `.cie/` directory.
- Generates a `.cie/project.yaml` file with sensible defaults.

### Step 2: Index Your Code

Index your repository:

```bash
cie index
```

**What happens:**
- CIE parses your code locally using Tree-sitter and stores the index in `~/.cie/data/<project>/`.
- Functions, types, and call graphs are extracted and stored.

> **Note:** Ollama is optional. Without it, CIE indexes all metadata (functions, types, calls) but skips embeddings. 20+ tools work without embeddings; semantic search requires them.

### Step 3: Verify the Index

Check the index status and try a basic query:

```bash
cie status
```

**Expected output:**
```text
Project: your-project-name
Files: 142
Functions: 3,431
Types: 287
Last indexed: 1 minute ago
```

---

## Basic Usage

### Essential CLI Commands

| Command | Description |
|---------|-------------|
| `cie init` | Initialize CIE in a project |
| `cie index` | Index or reindex the codebase |
| `cie status` | Show index statistics |
| `cie query <script>` | Execute a CozoScript query |
| `cie --mcp` | Start as an MCP server for AI assistants |
| `cie serve` | Start a local HTTP server |
| `cie reset --yes` | Delete all indexed data for the project |

---

## Optional: Enable Semantic Search

CIE works without any embedding provider -- grep, call graph, function finder, and 20+ tools work immediately after indexing. To enable semantic search:

1. Install Ollama:
   ```bash
   brew install ollama
   ```

2. Pull the embedding model:
   ```bash
   ollama pull nomic-embed-text
   ```

3. Start Ollama:
   ```bash
   ollama serve
   ```

4. Re-index to generate embeddings:
   ```bash
   cie index --full
   ```

No configuration changes needed -- the defaults already point to local Ollama.

---

## Server and Remote Modes

### Local HTTP Server

If you want to expose CIE as an HTTP API (for example, for custom integrations), you can run it as a local server:

```bash
cie serve --port 9090
```

This starts a local HTTP server that:
- Uses your local indexed data from `~/.cie/data/<project_id>/`
- Exposes a REST API for querying the index

However, `cie --mcp` now works directly in embedded mode -- no server needed. For most users, the MCP integration is the recommended way to connect CIE to AI assistants.

### Remote Mode (Enterprise)

For enterprise and distributed setups, CIE supports an `edge_cache` mode where the CLI connects to a remote CIE server. See the [Configuration Guide](./configuration.md) for details.

---

## Next Steps

Now that you have CIE running, explore these resources:

- **[MCP Integration](./mcp-integration.md)**: Connect CIE to Claude Code or Cursor.
- **[Configuration Guide](./configuration.md)**: Customize how CIE indexes your code.
- **[Troubleshooting](./troubleshooting.md)**: Common issues and solutions.
