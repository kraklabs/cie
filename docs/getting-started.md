# Getting Started with CIE

This guide will help you install CIE, index your first codebase, and start using it with Claude Code or Cursor in under 5 minutes.

**What you'll learn:**
- How to install the CIE CLI
- How to start the infrastructure with `cie start`
- How to index your first codebase
- How to verify everything is working

---

## Prerequisites

Before installing CIE, ensure you have these requirements:

- **Docker and Docker Compose**: Required for running the background services (Ollama and CIE Server).
- **Go 1.24+**: Only if you plan to build the CLI from source (recommended for development).
- **Git**: For cloning the repository and indexing code.

---

## Installation

Choose the installation method that works best for you:

### Option 1: Install Script (Recommended)

The easiest way to get the CIE CLI is using our installation script:

```bash
curl -sSL https://raw.githubusercontent.com/kraklabs/cie/main/install.sh | sh
```

This will download the correct binary for your OS and architecture and place it in your path.

### Option 2: Build from Source

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
- Automatically detects if you have the CIE infrastructure running.

### Step 2: Start the Infrastructure

CIE relies on Ollama for embeddings and a server for processing. Use the `start` command to automate the Docker setup:

```bash
cie start
```

**What happens:**
- Verifies Docker is running.
- Starts the `cie-server` and `ollama` containers.
- Downloads the required embedding model (`nomic-embed-text`) if missing.
- Performs a health check to ensure everything is ready.

### Step 3: Index Your Code

Now that the infrastructure is up, you can index your repository:

```bash
cie index
```

**What happens:**
- The CLI communicates with the Docker container to index your code.
- Files are parsed using Tree-sitter.
- Semantic embeddings are generated and stored.

### Step 4: Verify the Index

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
| `cie start` | Start Docker infrastructure (Ollama + CIE Server) |
| `cie stop` | Stop Docker infrastructure (preserves data) |
| `cie reset --yes` | Delete all indexed data for the project |
| `cie reset --yes --docker` | Reset data and Docker volumes (full reset) |
| `cie index` | Index or reindex the codebase |
| `cie status` | Show index statistics |
| `cie query <script>` | Execute a CozoScript query |
| `cie --mcp` | Start as an MCP server for AI assistants |

---

## Next Steps

Now that you have CIE running, explore these resources:

- **[MCP Integration](./mcp-integration.md)**: Connect CIE to Claude Code or Cursor.
- **[Configuration Guide](./configuration.md)**: Customize how CIE indexes your code.
- **[Troubleshooting](./troubleshooting.md)**: Common issues and solutions.
