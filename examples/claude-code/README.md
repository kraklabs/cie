# CIE Configuration for Claude Code

This example shows how to configure CIE as an MCP server for Claude Code, enabling deep codebase understanding through the Model Context Protocol.

## Prerequisites

Before setting up CIE with Claude Code, ensure you have:

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

3. **Claude Code installed:**
   - Desktop: Download from [claude.ai/code](https://claude.ai/code)
   - CLI: Follow installation instructions for your platform

## Quick Setup

### 1. Copy the Configuration

Copy the contents of `mcp-config.json` to your Claude Code settings file:

**macOS/Linux:**
```bash
~/.config/claude/settings.json
```

**Windows:**
```
%APPDATA%\Claude\settings.json
```

If the file doesn't exist, create it with the MCP configuration as the content.

If it already exists, add the `mcpServers` object to your existing settings, or merge the `cie` entry into your existing `mcpServers` object.

### 2. Update the Path

Edit the settings file and replace `/Users/yourname/project/.cie/project.yaml` with the **absolute path** to your project's CIE configuration.

**Important:** Must be an absolute path. Relative paths and `~` do not work in MCP configurations.

**Example paths:**
- macOS: `/Users/jane/code/my-app/.cie/project.yaml`
- Linux: `/home/jane/code/my-app/.cie/project.yaml`
- Windows: `C:/Users/jane/code/my-app/.cie/project.yaml` (use forward slashes)

### 3. Restart Claude Code

Close and restart Claude Code to load the new configuration.

### 4. Verify It Works

In Claude Code, try this query:
```
List the main functions in my project
```

If CIE is working, Claude Code will use the CIE tools to analyze your codebase and provide detailed information about your functions.

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
- `CIE_BASE_URL` - Edge Cache URL for remote/enterprise setup (optional, see docs)

### disabled

Set to `false` to enable the server (default), or `true` to temporarily disable it without removing the configuration.

## Usage Examples

Once configured, you can ask Claude Code natural language questions about your codebase. CIE provides 23+ specialized tools for code intelligence.

### Find Functions

Ask Claude to find specific functions by name or purpose:

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

### Tools not showing in Claude Code

**Solution:**
1. Verify the configuration is valid JSON (no trailing commas, proper quotes)
2. Ensure you saved the settings file
3. Restart Claude Code completely (not just reload)
4. Check Claude Code logs for errors

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

## More Information

For comprehensive documentation, see:

- **[MCP Integration Guide](../../docs/mcp-integration.md)** - Complete setup guide with advanced configuration
- **[Tools Reference](../../docs/tools-reference.md)** - Documentation for all 23+ CIE tools
- **[Configuration Reference](../../docs/configuration.md)** - All configuration options for `.cie/project.yaml`
- **[Troubleshooting Guide](../../docs/troubleshooting.md)** - Solutions to common issues
- **[Architecture Overview](../../docs/architecture.md)** - How CIE works internally

## Support

- **Issues:** [github.com/kraklabs/cie/issues](https://github.com/kraklabs/cie/issues)
- **Documentation:** [github.com/kraklabs/cie](https://github.com/kraklabs/cie)
