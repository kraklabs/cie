# CIE Configuration for Cursor

This example shows how to configure CIE as an MCP server for Cursor, enabling deep codebase understanding through the Model Context Protocol.

## Prerequisites

Before setting up CIE with Cursor, ensure you have:

1. **CIE installed:**
   ```bash
   go install github.com/kraklabs/cie@latest
   ```

2. **Project indexed:**
   ```bash
   cd /path/to/your/project
   cie init
   cie index
   ```

3. **Cursor installed:**
   - Download from [cursor.sh](https://cursor.sh)

## Quick Setup

### 1. Copy the Configuration

Copy the `.cursor/` directory to your project root:

```bash
cp -r examples/cursor/.cursor /path/to/your/project/
```

This places the MCP configuration file at `/path/to/your/project/.cursor/mcp.json`.

### 2. Update the Path

Edit `.cursor/mcp.json` in your project and replace `/Users/yourname/project/.cie/project.yaml` with the **absolute path** to your project's CIE configuration.

**Important:** Must be an absolute path. Relative paths and `~` do not work in MCP configurations.

**Example paths:**
- macOS: `/Users/jane/code/my-app/.cie/project.yaml`
- Linux: `/home/jane/code/my-app/.cie/project.yaml`
- Windows: `C:/Users/jane/code/my-app/.cie/project.yaml` (use forward slashes)

### 3. Restart Cursor

Close and restart Cursor to load the new configuration.

### 4. Verify It Works

In Cursor chat, try this query:
```
List the main functions in my project
```

If CIE is working, Cursor will use the CIE tools to analyze your codebase and provide detailed information about your functions.

## Configuration Options

### command

The path to the CIE binary.

- If CIE is in your PATH (after `go install`), use: `"cie"`
- Otherwise, use the absolute path: `"/path/to/cie"`

### args

Command-line arguments for CIE:

- `--mcp` - Start CIE in MCP server mode (JSON-RPC over stdio)
- `--config PATH` - Absolute path to your project's `.cie/project.yaml` configuration file

### env (Optional)

Environment variables for CIE:

- `CIE_LOG_LEVEL` - Logging verbosity (options: `debug`, `info`, `warn`, `error`)
- `CIE_BASE_URL` - Edge Cache URL for distributed setup (advanced, see docs)

### disabled

Set to `false` to enable the server (default), or `true` to temporarily disable it without removing the configuration.

## Usage Examples

Once configured, you can ask Cursor natural language questions about your codebase. CIE provides 23+ specialized tools for code intelligence.

### Find Functions

Ask Cursor to find specific functions by name or purpose:

```
Find the authentication middleware function
```

```
Show me all HTTP handlers in the gateway
```

### Trace Execution

Understand how code flows through your application:

```
Show me how the app handles user login
```

```
Trace the call path from main to the database connection
```

### Semantic Search

Search by meaning, not just text:

```
Find code related to rate limiting
```

```
Show me where we handle payment processing
```

### Architecture Questions

Get high-level understanding of your codebase:

```
What are the main entry points in this codebase?
```

```
How is the HTTP routing organized?
```

```
List all gRPC services and their methods
```

## Troubleshooting

### CIE not found

**Error:** `Command not found: cie`

**Solution:** Ensure CIE is in your PATH:
```bash
which cie  # Should print the path to cie
```

If not found, either:
- Run `go install github.com/kraklabs/cie@latest` again
- Use the absolute path in `"command"`: `"/Users/yourname/go/bin/cie"`

### Config file not found

**Error:** `Failed to load config: ...`

**Solution:** The path to `project.yaml` must be absolute. Use `pwd` to get the full path:
```bash
cd /path/to/your/project
pwd  # Prints: /Users/yourname/code/myapp
# Then use: /Users/yourname/code/myapp/.cie/project.yaml
```

### MCP server not loading in Cursor

**Solution:**
1. Verify `.cursor/mcp.json` is in your project root
2. Ensure the configuration is valid JSON (no trailing commas, proper quotes)
3. Check that the `.cie/project.yaml` path is absolute and correct
4. Restart Cursor completely (not just reload window)
5. Check Cursor's output/logs for MCP server errors

### Tools not showing in Cursor

**Solution:**
1. Verify CIE is running: `ps aux | grep cie`
2. Check that your project is indexed: `cie status`
3. Look for MCP-related errors in Cursor's developer console
4. Ensure you're using Cursor version 0.40+ (MCP support)

### Permission denied errors

**Error:** `Permission denied: /path/to/.cie/...`

**Solution:** Ensure CIE has read access to your project directory:
```bash
ls -la /path/to/project/.cie/
```

If needed, adjust permissions:
```bash
chmod 755 /path/to/project/.cie/
```

### CIE is working but results seem incomplete

**Solution:** Your project might not be fully indexed. Re-run indexing:
```bash
cd /path/to/your/project
cie index
```

## Per-Project vs Global Configuration

### Per-Project (Recommended)

Place `.cursor/mcp.json` in each project root. Each project gets its own CIE instance with project-specific indexing.

**Advantages:**
- Isolated per project
- Different CIE versions per project
- Project-specific configuration
- Easier to share with team

### Global Configuration

Alternatively, place configuration in `~/.cursor/mcp.json` to apply to all projects.

**Note:** This is less flexible and not recommended for multi-project workflows.

## More Information

For comprehensive documentation, see:

- **[MCP Integration Guide](../../docs/mcp-integration.md)** - Complete setup guide with advanced configuration
- **[Tools Reference](../../docs/tools-reference.md)** - Documentation for all 23+ CIE tools
- **[Configuration Reference](../../docs/configuration.md)** - All configuration options for `.cie/project.yaml`
- **[Troubleshooting Guide](../../docs/troubleshooting.md)** - Solutions to common issues
- **[Architecture Overview](../../docs/architecture.md)** - How CIE works internally
- **[Cursor Documentation](https://cursor.sh/docs)** - Official Cursor docs

## Support

- **Issues:** [github.com/kraklabs/cie/issues](https://github.com/kraklabs/cie/issues)
- **Documentation:** [github.com/kraklabs/cie](https://github.com/kraklabs/cie)
