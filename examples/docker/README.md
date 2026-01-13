# CIE Docker Example

Run CIE (Code Intelligence Engine) in a containerized environment using Docker Compose. This example demonstrates production-ready deployment with Ollama for embeddings and follows Docker best practices.

## Prerequisites

- **Docker** 20.10 or later
- **Docker Compose** 2.0 or later

Verify your installation:

```bash
docker --version
docker compose version
```

## Quick Start

Get CIE running in 5 steps:

### 1. Navigate to This Example

```bash
cd modules/cie/examples/docker
```

### 2. (Optional) Customize Environment

```bash
cp .env.example .env
# Edit .env to customize project ID, log level, etc.
```

The defaults work out of the box - customization is optional.

### 3. Start Services

```bash
docker compose up -d
```

This starts:
- **CIE**: Code Intelligence Engine (CLI-based)
- **Ollama**: Embedding generation service

### 4. Pull Embedding Model (First Time Only)

```bash
docker compose exec ollama ollama pull nomic-embed-text
```

This downloads the embedding model (~1GB). Only needed once - the model persists in the `ollama-data` volume.

### 5. Index Your Code

```bash
# Place your code in ./code/ directory
mkdir -p code
cp -r /path/to/your/repo code/

# Index the code
docker compose exec cie cie index /workspace
```

Done! CIE is now ready to answer questions about your code.

## Usage Examples

### Query Your Code

```bash
# Ask questions
docker compose exec cie cie query "what does the main function do"

# Search for patterns
docker compose exec cie cie grep "func main" --path-pattern="*.go"

# List all functions
docker compose exec cie cie list-functions --path="cmd/"

# Analyze architecture
docker compose exec cie cie analyze "what are the main entry points"
```

### View Logs

```bash
# CIE logs
docker compose logs -f cie

# Ollama logs
docker compose logs -f ollama

# All services
docker compose logs -f
```

### Stop Services

```bash
# Stop (data persists in volumes)
docker compose down

# Stop and remove all data
docker compose down -v
```

## Configuration

### Volume Mounts

The compose file mounts three types of volumes:

| Mount | Purpose | Type | Notes |
|-------|---------|------|-------|
| `./code:/workspace:ro` | Your source code | Bind mount, read-only | Place code in `./code/` directory |
| `cie-data:/data` | CIE index & config | Named volume | Persists between restarts |
| `ollama-data:/root/.ollama` | Ollama models | Named volume | Persists embedding models (~1GB) |

**Security Note:** The code mount is read-only (`:ro`) - CIE never modifies your source code.

### Environment Variables

Configure CIE via environment variables (set in `.env` file):

| Variable | Default | Description |
|----------|---------|-------------|
| `CIE_PROJECT_ID` | `my-project` | Unique identifier for your project |
| `CIE_LOG_LEVEL` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |
| `OLLAMA_EMBED_MODEL` | `nomic-embed-text` | Embedding model (alternatives: `mxbai-embed-large`, `all-minilm`) |
| `OLLAMA_HOST` | `http://ollama:11434` | Ollama service URL (auto-configured for Docker) |

**Optional LLM Narrative:**

For enhanced `cie analyze` output, uncomment these in `.env`:

```bash
CIE_LLM_URL=http://ollama:11434
CIE_LLM_MODEL=llama3
```

See `.env.example` for all available options.

### Network Architecture

```
┌─────────────────────────────────────┐
│  Docker Compose Network             │
│  (cie-network)                      │
│                                     │
│  ┌──────────┐      ┌─────────────┐ │
│  │   CIE    │─────▶│   Ollama    │ │
│  │  :8080   │      │   :11434    │ │
│  └──────────┘      └─────────────┘ │
│       │                             │
│       │ (volume mounts)             │
│       ▼                             │
│  ./code (read-only)                 │
│  cie-data (persistent)              │
└─────────────────────────────────────┘
```

## Usage Scenarios

### Scenario 1: Local Development

Integrate CIE into your development workflow:

**Project Structure:**
```
your-project/
├── docker-compose.yml          # Your app's compose file
├── modules/cie/examples/docker/
│   └── docker-compose.yml      # CIE compose file
└── src/                        # Your code
```

**Merge the compose files:**

```yaml
# In your docker-compose.yml
services:
  app:
    # Your application
    build: .
    ports:
      - "3000:3000"

  cie:
    image: ghcr.io/kraklabs/cie:latest
    volumes:
      - .:/workspace:ro
      - cie-data:/data
    environment:
      - CIE_PROJECT_ID=my-app
      - OLLAMA_HOST=http://ollama:11434
    depends_on:
      - ollama

  ollama:
    image: ollama/ollama:latest
    volumes:
      - ollama-data:/root/.ollama
    ports:
      - "11434:11434"
```

Now `docker compose up` starts your app + CIE together.

### Scenario 2: CI/CD Integration

Use CIE in GitHub Actions for code analysis:

```yaml
# .github/workflows/code-analysis.yml
name: Code Analysis

on: [pull_request]

jobs:
  analyze:
    runs-on: ubuntu-latest

    services:
      ollama:
        image: ollama/ollama:latest
        ports:
          - 11434:11434

    steps:
      - uses: actions/checkout@v4

      - name: Pull embedding model
        run: |
          docker exec ${{ job.services.ollama.id }} \
            ollama pull nomic-embed-text

      - name: Index codebase
        run: |
          docker run --rm \
            --network ${{ job.container.network }} \
            -v ${{ github.workspace }}:/workspace:ro \
            -e OLLAMA_HOST=http://ollama:11434 \
            -e CIE_PROJECT_ID=${{ github.repository }} \
            ghcr.io/kraklabs/cie:latest \
            cie index /workspace

      - name: Analyze changes
        run: |
          docker run --rm \
            --network ${{ job.container.network }} \
            -v ${{ github.workspace }}:/workspace:ro \
            -e OLLAMA_HOST=http://ollama:11434 \
            -e CIE_PROJECT_ID=${{ github.repository }} \
            ghcr.io/kraklabs/cie:latest \
            cie analyze "what changed in this PR"
```

### Scenario 3: MCP Server Mode

Run CIE as an MCP (Model Context Protocol) server for Claude Code or Cursor:

```yaml
services:
  cie:
    image: ghcr.io/kraklabs/cie:latest
    command: ["cie", "--mcp", "--config", "/data/.cie/project.yaml"]
    volumes:
      - ./code:/workspace:ro
      - cie-data:/data
    ports:
      - "8080:8080"  # HTTP MCP server
    environment:
      - CIE_PROJECT_ID=my-project
      - OLLAMA_HOST=http://ollama:11434
```

Then configure Claude Code to connect to `http://localhost:8080`.

## Troubleshooting

### Volume Permission Issues

**Symptom:** CIE can't write to `/data` directory

**Cause:** CIE runs as non-root user (uid=65532) in distroless image

**Solution:**

```bash
# Create data directory with correct permissions
docker compose down
docker volume rm docker_cie-data
docker compose up -d
```

The volume will be created with correct permissions on first run.

### Ollama Model Not Found

**Symptom:** Error: "embedding model not found: nomic-embed-text"

**Cause:** Embedding model wasn't pulled after first start

**Solution:**

```bash
docker compose exec ollama ollama pull nomic-embed-text
```

Verify the model is available:

```bash
docker compose exec ollama ollama list
```

### CIE Can't Connect to Ollama

**Symptom:** Error: "failed to connect to ollama: connection refused"

**Cause:** Ollama service not healthy or wrong hostname

**Solution:**

1. Check Ollama health:
```bash
docker compose ps ollama
# Should show "healthy" status
```

2. Check Ollama is responding:
```bash
curl http://localhost:11434/api/tags
```

3. Check environment variable:
```bash
docker compose exec cie printenv OLLAMA_HOST
# Should output: http://ollama:11434
```

### Port Conflicts

**Symptom:** Error: "port 11434 already in use"

**Cause:** Another service using Ollama's default port

**Solution:**

Edit `docker-compose.yml` to use a different port:

```yaml
services:
  ollama:
    ports:
      - "11435:11434"  # Use 11435 instead
```

Then update `OLLAMA_HOST` in `.env`:

```bash
OLLAMA_HOST=http://ollama:11434  # Keep internal port the same
```

### Code Not Being Indexed

**Symptom:** `cie query` returns "no results"

**Cause:** Code directory is empty or not mounted correctly

**Solution:**

1. Verify code mount:
```bash
docker compose exec cie ls -la /workspace
# Should show your code files
```

2. Re-run indexing:
```bash
docker compose exec cie cie index /workspace
```

3. Check index status:
```bash
docker compose exec cie cie status
```

## Advanced Usage

### Custom Configuration

Mount a custom `project.yaml` config:

```yaml
services:
  cie:
    volumes:
      - ./code:/workspace:ro
      - cie-data:/data
      - ./.cie/project.yaml:/data/.cie/project.yaml:ro  # Custom config
```

### Distributed Architecture

For production deployments with Primary Hub + Edge Cache, see the full distributed setup in the root directory:

```bash
# View distributed architecture example
cat ../../docker-compose.cie.yml
```

This includes:
- **Primary Hub**: Single writer for ingestion (gRPC on :50051)
- **Edge Cache**: Read replicas for queries (HTTP on :8080)
- **Ingestion**: On-demand indexing workers

### Health Monitoring

Check service health:

```bash
# All services
docker compose ps

# Specific health check
docker compose exec ollama curl -f http://localhost:11434/api/tags
```

### Resource Limits

Add resource constraints for production:

```yaml
services:
  cie:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 4G
        reservations:
          cpus: '1.0'
          memory: 2G
```

## More Information

- **[Main README](../../README.md)** - CIE overview and features
- **[Configuration Reference](../../docs/configuration.md)** - All config options
- **[Tools Reference](../../docs/tools-reference.md)** - 23+ MCP tools documentation
- **[MCP Integration Guide](../../docs/mcp-integration.md)** - Use with Claude Code/Cursor
- **[Architecture Guide](../../docs/architecture.md)** - How CIE works internally
- **[Dockerfile](../../Dockerfile)** - Multi-stage build details

## Support

- **Issues**: https://github.com/kraklabs/cie/issues
- **Discussions**: https://github.com/kraklabs/cie/discussions
- **Documentation**: https://docs.kraklabs.com/cie
