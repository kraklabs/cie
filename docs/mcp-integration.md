# MCP Integration Guide

Complete guide for integrating CIE as an MCP (Model Context Protocol) server with AI assistants like Claude Code and Cursor.

> **Target Audience:** Developers who want to use CIE's code intelligence tools within their AI coding assistants.
>
> **Related Documentation:**
> - [Getting Started](./getting-started.md) - Initial setup and indexing
> - [Tools Reference](./tools-reference.md) - Complete tool documentation
> - [Configuration](./configuration.md) - `.cie/project.yaml` reference

---

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Claude Code Setup](#claude-code-setup)
- [Cursor Setup](#cursor-setup)
- [Verifying Integration](#verifying-integration)
- [Using CIE from AI Assistants](#using-cie-from-ai-assistants)
- [Troubleshooting](#troubleshooting)
- [Advanced Configuration](#advanced-configuration)

---

## Overview

### What is MCP?

The Model Context Protocol (MCP) is an open standard for connecting AI assistants to external tools and data sources. CIE implements MCP v1.4.0, allowing AI assistants like Claude Code and Cursor to query your codebase using 23+ specialized code intelligence tools.

### Why use CIE with AI Assistants?

When integrated via MCP, your AI assistant gains powerful capabilities:

- **Semantic Search** - Find code by meaning, not just keywords
- **Call Graph Analysis** - Trace function calls and dependencies
- **Architecture Understanding** - Answer questions about codebase structure
- **HTTP Route Discovery** - List all API endpoints automatically
- **Security Audits** - Verify sensitive patterns are absent
- **Smart Navigation** - Jump to definitions, find implementations

### What to Expect

Once configured, your AI assistant can:

1. **Answer questions about your codebase** - "What are the main entry points?" or "How does authentication work?"
2. **Find code by description** - "Find the function that handles JWT validation"
3. **Analyze dependencies** - "What calls this function?" or "Show me the call graph"
4. **Discover APIs** - "List all HTTP POST endpoints"

All responses include file paths and line numbers for easy navigation.

---

## Prerequisites

Before setting up MCP integration, ensure:

- [ ] **CIE is installed** - Run `cie --version` to verify (see [Getting Started](./getting-started.md))
- [ ] **Project is indexed** - Run `cie index` in your project directory
- [ ] **AI assistant installed** - Claude Code or Cursor is installed on your system
- [ ] **CIE in PATH** - Running `which cie` (Unix) or `where cie` (Windows) finds the binary

**Verify CIE is working:**

```bash
cd /path/to/your/project
cie index   # Ensure code is indexed
cie status  # Verify status
```

---

## Claude Code Setup

### macOS / Linux

**Step 1: Locate Claude Code Configuration**

Claude Code stores MCP server configurations in:

```
macOS:   ~/.config/claude/settings.json
Linux:   ~/.config/claude/settings.json
```

If the file doesn't exist, create it:

```bash
mkdir -p ~/.config/claude
touch ~/.config/claude/settings.json
```

**Step 2: Add CIE MCP Server**

Edit `~/.config/claude/settings.json`:

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp"]
    }
  }
}
```

> **Note:** CIE MCP server runs in embedded mode by default -- it reads directly from the local CozoDB database at `~/.cie/data/<project>/`. No HTTP server or Docker infrastructure is needed. For distributed setups, configure `edge_cache` in `.cie/project.yaml` to use a remote server.

If you have multiple projects, you can use the `--config` flag to specify which one to use.

**Example for a project at `/Users/alice/code/myapp`:**

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp", "--config", "/Users/alice/code/myapp/.cie/project.yaml"]
    }
  }
}
```

**Step 3: Restart Claude Code**

Quit and restart Claude Code to load the new MCP server configuration.

**Step 4: Verify Connection**

In Claude Code, ask:

```
What CIE tools are available?
```

You should see a list of 23+ tools like `cie_semantic_search`, `cie_find_function`, `cie_list_endpoints`, etc.

### Windows

**Step 1: Locate Configuration**

Claude Code configuration is typically at:

```
%APPDATA%\Claude\settings.json
```

Full path example: `C:\Users\Alice\AppData\Roaming\Claude\settings.json`

If the file doesn't exist, create it:

```powershell
New-Item -Path "$env:APPDATA\Claude" -ItemType Directory -Force
New-Item -Path "$env:APPDATA\Claude\settings.json" -ItemType File
```

**Step 2: Add CIE Server**

Edit `settings.json` with the same structure as macOS/Linux:

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp", "--config", "C:\\Users\\Alice\\code\\myapp\\.cie\\project.yaml"]
    }
  }
}
```

**Important:** Use double backslashes (`\\`) or forward slashes (`/`) in Windows paths.

**Step 3: Verify CIE is in PATH**

Open PowerShell and run:

```powershell
where.exe cie
```

If not found, either:
- Add CIE installation directory to PATH
- Use absolute path to `cie.exe` in the `command` field

**Step 4: Restart and Verify**

Restart Claude Code and test with "What CIE tools are available?"

---

## Cursor Setup

### macOS / Linux

**Step 1: Open Cursor Settings**

Cursor supports MCP configuration through:

1. **UI Method (Recommended):**
   - Open Cursor Settings (Cmd+, or Ctrl+,)
   - Search for "MCP" or "Model Context Protocol"
   - Add new MCP server

2. **Manual Method:**
   - Edit `~/.cursor/mcp.json` (create if doesn't exist)

**Step 2: Add CIE Server**

**Via UI:**
- Server name: `cie`
- Command: `cie`
- Args: `--mcp --config /absolute/path/to/project/.cie/project.yaml`

**Via JSON file (`~/.cursor/mcp.json`):**

```json
{
  "mcpServers": {
    "cie": {
      "command": "cie",
      "args": ["--mcp"]
    }
  }
}
```

**Step 3: Restart Cursor**

Close and reopen Cursor to load the MCP server.

**Step 4: Test Integration**

Ask Cursor:

```
Use CIE to find the main entry point in this project
```

Cursor should invoke `cie_semantic_search` or `cie_analyze` and return results with file paths.

### Windows

Follow the same steps as macOS/Linux, but:

- Config file: `%APPDATA%\Cursor\mcp.json`
- Use Windows-style paths with double backslashes or forward slashes

---

## Verifying Integration

### Step 1: Check CIE Server Responds

Test the MCP server manually:

```bash
cd /path/to/your/project
cie --mcp
```

In embedded mode, the MCP server communicates via stdin/stdout silently -- there is no visible output. The server is ready when the process starts. Press Ctrl+C to stop.

### Step 2: Verify Tools are Available

In your AI assistant, ask:

```
What CIE tools are available?
```

**Expected response:** A list including:

- `cie_semantic_search`
- `cie_find_function`
- `cie_get_call_graph`
- `cie_list_endpoints`
- `cie_trace_path`
- ...and 18 more tools

### Step 3: Test a Simple Query

Try a semantic search:

```
Use CIE to find authentication-related code
```

**Expected response:**

```
Found 3 functions related to authentication:

1. **AuthMiddleware** (87% match)
   File: internal/http/middleware.go:42
   Handles JWT token validation for HTTP requests

2. **ValidateToken** (76% match)
   File: internal/auth/jwt.go:23
   Validates and parses JWT tokens

3. **Login** (68% match)
   File: internal/handlers/auth.go:15
   Handles user login and token generation
```

### Step 4: Verify Index is Up to Date

If results seem outdated, check index status:

```bash
cie status
```

If "Last indexed" is old, re-index:

```bash
cie index
```

---

## Using CIE from AI Assistants

### Finding Functions by Description

**Ask:**
```
Find the function that handles database connection pooling
```

**Behind the scenes:** Uses `cie_semantic_search` with embeddings to find semantically similar functions.

**Result:**
```
[HIGH] **NewConnectionPool** (92% match)
File: internal/db/pool.go:34-67
Creates and configures a new database connection pool with
configurable size, timeouts, and health checks.
```

### Tracing Call Paths

**Ask:**
```
How does the application reach the HandleUserLogin function?
```

**Behind the scenes:** Uses `cie_trace_path` to find execution paths from entry points.

**Result:**
```
Found 2 call paths to HandleUserLogin:

Path 1 (4 steps):
main → SetupRouter → RegisterAuthRoutes → HandleUserLogin
/cmd/server/main.go:23 → /internal/http/router.go:45 → /internal/http/auth_routes.go:12 → /internal/handlers/auth.go:78

Path 2 (3 steps):
TestLogin → HandleUserLogin
/internal/handlers/auth_test.go:34 → /internal/handlers/auth.go:78
```

### Understanding Architecture

**Ask:**
```
What are the main entry points in this codebase?
```

**Behind the scenes:** Uses `cie_analyze` with LLM-powered architectural analysis.

**Result:**
```
Main Entry Points:

1. **main** (cmd/server/main.go:23)
   - HTTP server entry point
   - Initializes database, router, middleware
   - Listens on :8080

2. **init** (internal/config/config.go:12)
   - Loads environment variables
   - Sets up default configuration

3. **main** (cmd/cli/main.go:15)
   - CLI entry point for admin commands
```

### Discovering HTTP Endpoints

**Ask:**
```
List all POST endpoints in this API
```

**Behind the scenes:** Uses `cie_list_endpoints` to extract HTTP routes.

**Result:**
```
Found 8 POST endpoints:

| Method | Path | Handler | File |
|--------|------|---------|------|
| POST | /api/v1/users | CreateUser | internal/handlers/users.go:23 |
| POST | /api/v1/login | Login | internal/handlers/auth.go:45 |
| POST | /api/v1/logout | Logout | internal/handlers/auth.go:78 |
| POST | /api/v1/projects | CreateProject | internal/handlers/projects.go:34 |
...
```

### Security Audits

**Ask:**
```
Use CIE to verify there are no hardcoded API keys or secrets in the src/ directory
```

**Behind the scenes:** Uses `cie_verify_absence` to check for sensitive patterns.

**Result:**
```
Yes PASS: No occurrences of sensitive patterns found

Checked patterns:
- "apiKey" (0 matches)
- "access_token" (0 matches)
- "secret" (0 matches)
- "password" (0 matches)

Scanned: 234 functions in src/
```

### Getting Function Source Code

**Ask:**
```
Show me the source code of the AuthMiddleware function
```

**Behind the scenes:** Uses `cie_get_function_code` to retrieve implementation.

**Result:**
```go
// File: internal/http/middleware.go:42-67

func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Missing authorization", http.StatusUnauthorized)
            return
        }

        claims, err := ValidateToken(token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), "user", claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## Troubleshooting

### CIE Server Not Starting

**Symptom:** AI assistant says "CIE server is not available" or similar error.

**Possible Causes:**

1. **CIE not in PATH**
   ```bash
   # Check if CIE is accessible
   which cie  # macOS/Linux
   where cie  # Windows
   ```

   **Solution:** Either add CIE to PATH or use absolute path in config:
   ```json
   {
     "mcpServers": {
       "cie": {
         "command": "/usr/local/bin/cie",
         "args": ["--mcp", "--config", "/path/to/.cie/project.yaml"]
       }
     }
   }
   ```

2. **Project not indexed**

   **Solution:** Run `cie index` in your project directory. Verify the index exists:
   ```bash
   ls -la /path/to/project/.cie/project.yaml
   cie status
   ```

3. **Wrong config file path**

   **Solution:** Verify `.cie/project.yaml` exists:
   ```bash
   ls -la /path/to/project/.cie/project.yaml
   ```

   If missing, run `cie index` in project directory.

4. **Relative path used instead of absolute**

   **Solution:** Change from `~/code/myapp` to `/Users/alice/code/myapp` (full path).

### Tools Not Appearing in AI Assistant

**Symptom:** AI assistant doesn't recognize CIE tools.

**Solutions:**

1. **Restart the AI assistant** - MCP servers load on startup
2. **Check JSON syntax** - Validate your `settings.json` or `mcp.json` is valid JSON
3. **Check server logs** - Test `cie --mcp` manually to see error messages
4. **Verify protocol version** - Ensure AI assistant supports MCP v1.4.0

### Queries are Slow

**Symptom:** CIE queries take >5 seconds.

**Possible Causes:**

1. **Large codebase without filters**

   **Solution:** Ask AI assistant to use filters:
   ```
   Find authentication code, but only search in internal/auth directory
   ```

   This adds `path_pattern="internal/auth"` to the query.

2. **Embedding model is slow**

   **Solution:** Configure faster embedding provider in `.cie/project.yaml`:
   ```yaml
   embedding:
     provider: ollama  # Local, fast
     model: nomic-embed-text
   ```

3. **Index is large and unoptimized**

   **Solution:** Exclude generated code from indexing:
   ```yaml
   indexing:
     exclude_patterns:
       - "**/node_modules/**"
       - "**/vendor/**"
       - "**/*.pb.go"
   ```

### Index is Out of Date

**Symptom:** CIE returns old code or doesn't find recent changes.

**Solution:** Re-index the project:

```bash
cd /path/to/project
cie index --force
```

Check index age:

```bash
cie status
```

**Output:**
```
Project: my-project
Last indexed: 2024-01-15 10:34:22 (3 hours ago)
Files indexed: 1,234
Functions indexed: 5,678
```

### "Project not indexed" Error

**Symptom:** CIE responds with "Project not indexed, please run cie index".

**Solution:**

```bash
cd /path/to/your/project
cie index
```

This creates `.cie/project.yaml` and builds the index.

### Permission Denied Errors

**Symptom:** "Permission denied" when starting MCP server.

**Solution:**

1. **Check file permissions:**
   ```bash
   ls -la $(which cie)
   chmod +x $(which cie)  # If not executable
   ```

2. **Check config file permissions:**
   ```bash
   ls -la /path/to/project/.cie/project.yaml
   ```

### Multiple Projects, Wrong One Selected

**Symptom:** CIE searches the wrong project.

**Solution:** Specify the exact config path in MCP server config:

```json
{
  "mcpServers": {
    "cie-project-a": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/project-a/.cie/project.yaml"]
    },
    "cie-project-b": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/project-b/.cie/project.yaml"]
    }
  }
}
```

---

## Advanced Configuration

### Multiple Projects

To work with multiple projects, configure separate MCP servers:

```json
{
  "mcpServers": {
    "cie-frontend": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/frontend/.cie/project.yaml"]
    },
    "cie-backend": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/backend/.cie/project.yaml"]
    },
    "cie-mobile": {
      "command": "cie",
      "args": ["--mcp", "--config", "/path/to/mobile/.cie/project.yaml"]
    }
  }
}
```

Your AI assistant will have access to all three projects simultaneously.

**Usage:**
```
Use cie-frontend to find the login component
Use cie-backend to find the authentication API
```

### Custom Embedding Provider

By default, CIE uses the embedding provider configured in `.cie/project.yaml`. To use a custom provider:

**Edit `.cie/project.yaml`:**

```yaml
project_id: my-project

embedding:
  provider: openai
  base_url: https://api.openai.com/v1
  model: text-embedding-3-small
  api_key: ${OPENAI_API_KEY}  # Or set via environment variable

# Alternative: Use local Ollama
embedding:
  provider: ollama
  base_url: http://localhost:11434
  model: nomic-embed-text
```

See [Configuration Guide](./configuration.md) for all embedding provider options.

### Performance Tuning

#### Exclude Non-Code Files

Reduce index size by excluding generated files:

```yaml
indexing:
  exclude_patterns:
    - "**/node_modules/**"
    - "**/vendor/**"
    - "**/*.pb.go"       # gRPC generated
    - "**/*.gen.go"      # Code generators
    - "**/dist/**"
    - "**/build/**"
```

#### Use Filters in Queries

Guide your AI assistant to use filters:

```
Find authentication code in the internal/auth directory only
```

This adds `path_pattern="internal/auth"` to reduce search space.

#### Optimize Semantic Search

For faster semantic searches, set minimum similarity threshold:

```
Find login code with at least 70% confidence
```

This adds `min_similarity=0.7` to filter low-quality matches.

### Custom Tool Behavior

Some CIE tools support advanced parameters. Ask your AI assistant to use them:

**Example: Exclude test files**
```
Find all HTTP handlers but exclude test files
```

Behind the scenes:
```json
{
  "tool": "cie_semantic_search",
  "arguments": {
    "query": "HTTP handlers",
    "role": "handler",
    "exclude_paths": "_test[.]go"
  }
}
```

**Example: Search specific language**
```
Find Python functions that handle database connections
```

Behind the scenes:
```json
{
  "tool": "cie_list_files",
  "arguments": {
    "language": "python"
  }
}
```

Then `cie_semantic_search` with `path_pattern` filter.

### Debugging MCP Connection

To see raw MCP communication:

**macOS/Linux:**
```bash
# Run CIE server with debug logging
cie --mcp --verbose 2>&1 | tee cie-mcp-debug.log
```

**Windows:**
```powershell
cie --mcp --verbose 2>&1 | Tee-Object -FilePath cie-mcp-debug.log
```

This logs all JSON-RPC messages for troubleshooting protocol issues.

### Environment Variables

CIE respects these environment variables (can override `.cie/project.yaml`):

| Variable | Description | Example |
|----------|-------------|---------|
| `CIE_PROJECT_ID` | Project identifier | `my-project` |
| `CIE_LLM_URL` | LLM API for `cie_analyze` | `http://localhost:11434` |
| `CIE_LLM_MODEL` | LLM model name | `qwen2.5-coder:7b` |
| `OPENAI_API_KEY` | OpenAI API key | `sk-...` |
| `OLLAMA_HOST` | Ollama server URL | `http://localhost:11434` |

Set these before starting the AI assistant if you want to override config file settings.

---

## Need More Help?

- **Tools Reference**: See [tools-reference.md](./tools-reference.md) for complete tool documentation
- **Configuration**: See [configuration.md](./configuration.md) for `.cie/project.yaml` options
- **Architecture**: See [architecture.md](./architecture.md) to understand how CIE works internally
- **MCP Spec**: Read the [Model Context Protocol specification](https://modelcontextprotocol.io/specification) for protocol details

---

**Last Updated:** 2026-02-06
