# Migration Guide: Docker to Embedded Mode

This guide helps you migrate from CIE's Docker-based workflow (`cie start`/`cie stop`) to the new embedded mode, where CIE runs as a single binary with no external dependencies.

## What Changed

CIE previously required Docker to run a local HTTP server (Primary Hub + Edge Cache) for the MCP tools to function. The new embedded mode removes this requirement entirely:

| Before (Docker) | After (Embedded) |
|-----------------|-------------------|
| `cie init` → `cie start` → `cie index` → `cie --mcp` | `cie init` → `cie index` → `cie --mcp` |
| Required Docker Desktop running | No Docker needed |
| Data stored in Docker volumes | Data stored in `~/.cie/data/<project>/` |
| MCP server connected to Edge Cache via HTTP | MCP server reads directly from local CozoDB |
| `edge_cache: http://localhost:9090` | `edge_cache: ""` (empty) |
| Ollama required for indexing | Ollama optional (only for semantic search) |

## Migration Steps

### 1. Stop Docker Infrastructure

If you still have CIE containers running:

```bash
# Stop containers (if using old workflow)
docker compose down

# Or if you used the embedded docker-compose:
docker stop cie-primary-hub cie-edge-cache 2>/dev/null
```

### 2. Re-initialize Configuration

```bash
cd /path/to/your/project
cie init --force -y
```

This regenerates `.cie/project.yaml` with the new defaults (`edge_cache: ""`).

If you prefer to edit manually, update your `.cie/project.yaml`:

```yaml
# Before (Docker mode)
cie:
    edge_cache: "http://localhost:9090"

# After (Embedded mode)
cie:
    edge_cache: ""
```

### 3. Re-index Locally

```bash
cie index --full
```

This writes directly to `~/.cie/data/<project_id>/` using the embedded CozoDB engine.

**Without Ollama:** Indexing works without Ollama. You get all 20+ structural tools (grep, call graph, function finder, etc.). Only `cie_semantic_search` requires embeddings. You can add Ollama later:

```bash
brew install ollama
ollama serve &
ollama pull nomic-embed-text
cie index --full   # Re-index with embeddings
```

### 4. Verify

```bash
cie status
```

Expected output:
```
Project: your-project
Mode: embedded
Files: <count>
Functions: <count>
```

### 5. Update MCP Configuration

Remove `CIE_BASE_URL` from your MCP configurations if present.

**Claude Code** (`~/.claude/settings.json` or `.mcp.json`):

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp", "--config", "/absolute/path/.cie/project.yaml"]
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp", "--config", "/absolute/path/.cie/project.yaml"]
    }
  }
}
```

No `env` section needed for embedded mode.

### 6. Clean Up Docker Resources (Optional)

```bash
# Remove Docker images
docker rmi kraklabs/cie-primary-hub kraklabs/cie-edge-cache 2>/dev/null

# Remove Docker volumes
docker volume rm cie_rocksdb_data cie_primary_data 2>/dev/null

# Remove old docker-compose files from project root
rm -f docker-compose.yml
```

## Auto-Fallback Behavior

If your `.cie/project.yaml` still has `edge_cache` configured but the server is unreachable, CIE automatically falls back to embedded mode when local data exists:

```
Warning: Edge Cache at http://localhost:9090 is not reachable. Falling back to embedded mode.
  Tip: Remove 'edge_cache' from .cie/project.yaml or run 'cie init --force -y' to use embedded mode by default.
CIE MCP Server v1.5.0 starting (embedded (fallback) mode)...
```

This means you can migrate gradually without breaking your workflow.

## Removed Commands

| Command | Status | Replacement |
|---------|--------|-------------|
| `cie start` | Removed | Not needed (embedded mode) |
| `cie stop` | Removed | Not needed (embedded mode) |
| `cie serve` | Still available | For advanced users who want the HTTP API |

## Remote/Enterprise Mode

If you're using CIE Enterprise with a remote Edge Cache, no migration is needed. Set `edge_cache` to your server URL:

```yaml
cie:
    edge_cache: "https://cie.yourcompany.com"
```

The MCP server will connect to the remote Edge Cache as before.

## Troubleshooting

### "No local data found" after migration

Your index data may be in Docker volumes instead of the local filesystem. Re-index:

```bash
cie index --full
```

### MCP tools return empty results

Verify the index exists and has data:

```bash
cie status
ls ~/.cie/data/<your-project-id>/
```

If empty, re-run `cie index --full`.

### Old config still references Docker

Run `cie init --force -y` to regenerate the configuration, or manually set `edge_cache: ""` in `.cie/project.yaml`.

## Related Documentation

- [Getting Started](./getting-started.md) - Fresh installation guide
- [Configuration Reference](./configuration.md) - All configuration options
- [Architecture Overview](./architecture.md) - How embedded mode works internally
- [Troubleshooting](./troubleshooting.md) - Common issues and solutions
