# CIE Troubleshooting Guide

A comprehensive guide for diagnosing and resolving common CIE issues.

**Quick Links:**
- [Quick Diagnostics](#quick-diagnostics) - Run these first
- [Installation Issues](#installation-issues) - Can't install CIE
- [Indexing Problems](#indexing-problems) - Index is empty or errors
- [Query Errors](#query-errors) - No results or timeouts
- [MCP Integration](#mcp-integration-issues) - AI assistant integration
- [Performance](#performance-problems) - Slow indexing/queries
- [Advanced Debugging](#advanced-debugging) - Deep diagnostic techniques
- [Getting Help](#getting-help) - Where to ask questions

---

## Quick Diagnostics

Run these commands first to gather system information:

```bash
# System information
cie --version          # Shows version, Go version, build info
docker ps              # Verify Docker containers are running
cie start              # Verify infrastructure health

# Configuration health
cie config show        # Display effective configuration
cie status             # Verify connection to server and index status

# Quick embedding test
curl http://localhost:11434/api/tags
```

**What to look for:**
- `cie --version` should show version without library errors
- `go version` should be 1.24 or newer
- `cie config show` should display valid YAML without errors
- `cie status` should show function count > 0 if indexed
- Ollama curl should return JSON list of models (if using Ollama)
- `.cie/db/` directory should exist with files if project is indexed

---

## Installation Issues

### Issue: CozoDB Library Not Found (macOS/Linux)

**Symptoms:**
- `error while loading shared libraries: libcozo_c.so`
- `Library not loaded: /.../libcozo_c.dylib`
- `fatal error: semawakeup on Darwin signal stack` (macOS specific)

**Cause:**
CIE uses CozoDB as its graph engine, which requires a C library. If the binary was not compiled with static linking or the dynamic library is missing from the search path, it will fail to start.

**Solution:**

1. **Use the Docker-based approach (Recommended):**
   By using `cie start` and letting the processing happen inside Docker, you avoid all local library dependency issues.

2. **Rebuild with Static Linking:**
   If you must run the CLI locally, use the provided `Makefile` which handles library downloading and static linking automatically:
   ```bash
   make build
   ```

3. **macOS "semawakeup" fix:**
   If you encounter a crash with `semawakeup on Darwin signal stack`, it's usually due to a conflict between Go's signal handling and the CozoDB library on macOS. We've mitigated this in the latest version by using static linking. Ensure you are using a binary built with `make build` or the one provided by `install.sh`.

---

### Issue: Go Version Too Old

**Symptoms:**
- `go: module requires Go 1.24 or later`
- Build errors mentioning language features not available
- Installation fails during `go install`

**Cause:**
CIE requires Go 1.24 or newer for language features and standard library APIs. Older Go versions lack required functionality.

**Solution:**

**Check current version:**
```bash
go version
# If < 1.24, upgrade:
```

**macOS:**
```bash
brew upgrade go
# or download from: https://go.dev/dl/
```

**Linux:**
```bash
# Remove old version
sudo rm -rf /usr/local/go

# Download latest
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz

# Add to PATH (add to ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin
```

**Verify:**
```bash
go version
# Should show: go version go1.24.X ...
```

**Related:**
- [Go Installation Guide](https://go.dev/doc/install)
- [CIE Installation Guide](./getting-started.md#installation)

---

### Issue: CGO_ENABLED Not Set

**Symptoms:**
- `undefined reference to 'cozo_open'` and other `cozo_*` functions
- Build succeeds but binary crashes with library errors
- Linking errors during `go build`

**Cause:**
CIE uses CozoDB's C bindings (CGO). CGO must be enabled during build. If `CGO_ENABLED=0`, Go will not link against C libraries.

**Solution:**

**Build with CGO enabled:**
```bash
# Set for current session
export CGO_ENABLED=1

# Install CIE
go install github.com/kraklabs/cie/cmd/cie@latest

# Or build from source (after cloning the repo)
CGO_ENABLED=1 go build -o cie ./cmd/cie
```

**Make permanent (add to ~/.bashrc or ~/.zshrc):**
```bash
echo 'export CGO_ENABLED=1' >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc
```

**Verify:**
```bash
go env CGO_ENABLED
# Should output: 1

cie --version
# Should work without errors
```

**Related:**
- [Go CGO Documentation](https://pkg.go.dev/cmd/cgo)
- [Build Instructions](./getting-started.md#building-from-source)

---

### Issue: Library Path Not Set on Linux

**Symptoms:**
- `error while loading shared libraries: libcozo_c.so: cannot open shared object file`
- Library exists at `/usr/local/lib/libcozo_c.so` but not found
- `cie --version` works as root but not as regular user

**Cause:**
Linux requires libraries to be in the dynamic linker's search path. Even if the library is installed, the system may not know where to find it at runtime.

**Solution:**

**Option 1: Update LD_LIBRARY_PATH (temporary):**
```bash
export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
cie --version
```

**Option 2: Update ldconfig (permanent, recommended):**
```bash
# Create config file
echo "/usr/local/lib" | sudo tee /etc/ld.so.conf.d/local.conf

# Refresh cache
sudo ldconfig

# Verify library is found
ldconfig -p | grep cozo
# Should show: libcozo_c.so (libc6,x86-64) => /usr/local/lib/libcozo_c.so
```

**Option 3: Install to standard location:**
```bash
# Move library to system library path
sudo mv /usr/local/lib/libcozo_c.so /usr/lib/libcozo_c.so
# or
sudo mv /usr/local/lib/libcozo_c.so /usr/lib64/libcozo_c.so  # RHEL/Fedora
```

**Verify:**
```bash
cie --version
# Should work without LD_LIBRARY_PATH
```

**Related:**
- [Linux Shared Library Configuration](https://man7.org/linux/man-pages/man8/ldconfig.8.html)
- [CIE Installation Guide](./getting-started.md#linux)

---

## Indexing Problems

### Issue: No Functions Found

**Symptoms:**
- `Functions indexed: 0` after running `cie index`
- `cie status` shows 0 functions
- Search returns no results even though code files exist

**Cause:**
This typically happens when:
1. No supported files exist in the project (only supports `.go`, `.py`, `.js`, `.ts`, `.tsx`)
2. Exclusion patterns are too broad and exclude all code files
3. Tree-sitter parsers fail to extract functions from files
4. Files are outside the indexed directory path

**Solution:**

1. **Check supported file extensions:**
   ```bash
   # Count supported files
   find . -type f \( -name "*.go" -o -name "*.py" -o -name "*.js" -o -name "*.ts" -o -name "*.tsx" \) | wc -l
   ```

   If count is 0, your project uses unsupported languages.

2. **Check exclusion patterns:**
   ```bash
   cie config show | grep exclude
   ```

   Common overly broad patterns:
   - `**/*` (excludes everything!)
   - `*` (excludes all files in root)

   Fix: Edit `.cie/project.yaml`, set sensible exclusions:
   ```yaml
   exclude:
     - "node_modules/**"
     - "vendor/**"
     - "*.test.go"
     - "**/*_test.go"
   ```

3. **Check indexing logs:**
   ```bash
   cie index --debug
   # Look for parse errors or excluded file messages
   ```

4. **Verify index path:**
   ```bash
   cie status
   # Check "Project Path" matches your code directory
   ```

   If wrong, reinitialize:
   ```bash
   cd /path/to/your/code
   cie init
   cie index
   ```

**Verify:**
```bash
cie status
# Should show: Functions indexed: > 0
```

**Related:**
- [Supported Languages](./getting-started.md#supported-languages)
- [Configuration Reference](./configuration.md#exclusion-patterns)

---

### Issue: Tree-sitter Parse Errors

**Symptoms:**
- `[ERROR] Failed to parse file: syntax error at line X`
- Some files indexed but others skipped
- Function count lower than expected

**Cause:**
Tree-sitter parsers encounter invalid syntax in source files. This can happen with:
- Syntax errors in the code
- Experimental language features not supported by tree-sitter
- Macro-heavy code (C preprocessor, Rust macros)
- Files with incorrect extensions (.js file containing TypeScript)

**Solution:**

1. **Review parse errors:**
   ```bash
   cie index --debug 2>&1 | grep "Failed to parse"
   # Note which files fail
   ```

2. **Check syntax of failing files:**
   ```bash
   # For Go
   go build ./path/to/failing/file.go

   # For TypeScript
   tsc --noEmit path/to/failing/file.ts

   # For Python
   python -m py_compile path/to/failing/file.py
   ```

3. **Exclude problematic files if unfixable:**
   ```yaml
   # .cie/project.yaml
   exclude:
     - "generated/**"         # Generated code often has parse issues
     - "**/*.generated.go"
     - "path/to/macro-heavy/file.c"
   ```

4. **Check tree-sitter grammar version:**
   ```bash
   cie --version
   # Shows tree-sitter grammar versions
   ```

   If outdated, update CIE to latest version:
   ```bash
   go install github.com/kraklabs/cie/cmd/cie@latest
   ```

**Note:** CIE gracefully skips unparseable files and continues indexing. Parse errors reduce index completeness but don't block indexing.

**Verify:**
```bash
cie index --debug 2>&1 | grep -c "Failed to parse"
# Should decrease after fixes
```

**Related:**
- [Indexing Process](./architecture.md#parsing-stage)
- [Language Support](./getting-started.md#language-support)

---

### Issue: Embedding Timeout (Ollama Connection Failed)

**Symptoms:**
- `failed to connect to Ollama at http://localhost:11434`
- `context deadline exceeded` during indexing
- Indexing hangs at embedding generation stage

**Cause:**
CIE cannot reach the Ollama embedding provider. Common causes:
- Ollama is not running
- Ollama is running on a different port
- Firewall blocking connection
- Ollama crashed or out of memory

**Solution:**

1. **Check if Ollama is running:**
   ```bash
   curl http://localhost:11434/api/tags
   ```

   If connection refused:
   ```bash
   # Start Ollama
   ollama serve

   # Or on macOS with Homebrew service:
   brew services start ollama
   ```

2. **Verify Ollama model is available:**
   ```bash
   ollama list
   # Should show nomic-embed-text or configured model
   ```

   If model missing:
   ```bash
   ollama pull nomic-embed-text
   ```

3. **Check Ollama port in config:**
   ```bash
   cie config show | grep ollama
   # Verify URL matches where Ollama is running
   ```

   Fix if wrong:
   ```yaml
   # .cie/project.yaml
   embedding:
     provider: ollama
     ollama:
       url: "http://localhost:11434"  # Match your Ollama port
       model: "nomic-embed-text"
   ```

4. **Check firewall rules:**
   ```bash
   # macOS
   sudo /usr/libexec/ApplicationFirewall/socketfilterfw --getblockall

   # Linux
   sudo iptables -L -n | grep 11434
   ```

**Verify:**
```bash
curl http://localhost:11434/api/tags
# Should return JSON: {"models":[...]}

cie index
# Should proceed without timeout
```

**Related:**
- [Embedding Providers](./configuration.md#embedding-providers)
- [Ollama Setup](./getting-started.md#ollama-setup)

---

### Issue: Out of Memory During Indexing

**Symptoms:**
- `cie index` killed with `Killed` message
- System becomes unresponsive during indexing
- dmesg shows OOM killer messages: `Out of memory: Kill process`

**Cause:**
Indexing large projects generates many embeddings simultaneously, consuming significant RAM. Default concurrency may be too high for available memory.

**Solution:**

1. **Reduce embedding worker concurrency:**
   ```yaml
   # .cie/project.yaml
   indexing:
     embed_workers: 1        # Reduce from default 4
     batch_size: 50          # Reduce from default 100
   ```

2. **Increase system swap (Linux):**
   ```bash
   # Check current swap
   swapon --show

   # Add 4GB swap file
   sudo fallocate -l 4G /swapfile
   sudo chmod 600 /swapfile
   sudo mkswap /swapfile
   sudo swapon /swapfile

   # Make permanent (add to /etc/fstab)
   echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
   ```

3. **Index in stages for large projects:**
   ```bash
   # Index subdirectories separately
   cd backend/
   cie init
   cie index

   cd ../frontend/
   cie init
   cie index
   ```

4. **Use lighter embedding model:**
   ```yaml
   # .cie/project.yaml - Use smaller model
   embedding:
     provider: ollama
     ollama:
       model: "mxbai-embed-large"  # Smaller than nomic-embed-text
   ```

5. **Monitor memory during indexing:**
   ```bash
   # Watch memory usage
   watch -n 1 free -h

   # Or with htop
   htop
   ```

**Verify:**
```bash
cie index
# Should complete without being killed
```

**Related:**
- [Performance Tuning](./configuration.md#performance-tuning)
- [System Requirements](./getting-started.md#system-requirements)

---

### Issue: Mutation Batch Too Large

**Symptoms:**
- `mutation statement exceeds max size: X bytes (limit: Y)`
- Error shows statement preview with CozoDB mutation
- Indexing stops partway through

**Cause:**
CIE batches mutations (inserts) to CozoDB for performance. If a single function's AST or embedding is very large, the batch may exceed CozoDB's mutation size limit (typically 10MB).

This happens with:
- Extremely long functions (>1000 lines)
- Functions with huge string literals
- Large generated code files

**Solution:**

1. **Reduce batch size:**
   ```yaml
   # .cie/project.yaml
   indexing:
     batch_size: 50   # Reduce from default 100
   ```

2. **Exclude problematic files:**

   The error message shows a statement preview. Look for the filename:
   ```
   Statement preview: {put nodes [[path/to/huge_file.go ...
   ```

   Exclude it:
   ```yaml
   # .cie/project.yaml
   exclude:
     - "path/to/huge_file.go"
     - "**/*.generated.go"  # Often has huge functions
   ```

3. **Refactor large functions (if you own the code):**

   Functions >500 lines are hard to index and hard to understand. Consider breaking them up:
   ```go
   // Before: 1000-line function
   func ProcessEverything() { ... }

   // After: Smaller functions
   func ProcessStep1() { ... }
   func ProcessStep2() { ... }
   func ProcessEverything() {
       ProcessStep1()
       ProcessStep2()
   }
   ```

**Verify:**
```bash
cie index
# Should complete without mutation size errors
```

**Related:**
- [Batch Size Configuration](./configuration.md#batch-size)
- [CozoDB Documentation](https://docs.cozodb.org/)

---

### Issue: Slow Indexing

**Symptoms:**
- Indexing takes >5 minutes for a small project (<10k LOC)
- Progress appears to stall at embedding generation
- CPU usage low but indexing not progressing

**Cause:**
Most often caused by slow embedding generation:
- Using a remote embedding API with high latency
- Ollama running on CPU instead of GPU
- Network issues with API provider
- Embedding model is very large

**Solution:**

1. **Check embedding provider latency:**
   ```bash
   # Test Ollama response time
   time curl -X POST http://localhost:11434/api/embeddings \
     -H "Content-Type: application/json" \
     -d '{"model":"nomic-embed-text","prompt":"test"}'

   # Should respond in <1 second for Ollama
   ```

2. **Switch to faster provider:**

   **Fastest (local Ollama):**
   ```yaml
   # .cie/project.yaml
   embedding:
     provider: ollama
     ollama:
       url: "http://localhost:11434"
       model: "nomic-embed-text"
   ```

   **Fast (OpenAI - requires API key):**
   ```yaml
   embedding:
     provider: openai
     openai:
       api_key: "${OPENAI_API_KEY}"
       model: "text-embedding-3-small"  # Smaller = faster
   ```

3. **Increase embedding workers (if provider is fast):**
   ```yaml
   # .cie/project.yaml
   indexing:
     embed_workers: 4  # Increase from default 1
   ```

   **Note:** Only increase if embeddings are fast (<100ms each). Otherwise you'll just queue up slow requests.

4. **Use indexing debug mode to identify bottleneck:**
   ```bash
   cie index --debug
   # Look for which stage is slow:
   # - "Parsing..." stage (parser issue)
   # - "Generating embeddings..." stage (embedding provider issue)
   # - "Writing to database..." stage (disk I/O issue)
   ```

5. **Check disk I/O (less common):**
   ```bash
   # Monitor disk usage during indexing
   iostat -x 1

   # If disk is bottleneck, move .cie/db to SSD
   ```

**Verify:**
```bash
time cie index
# Should complete in <2 minutes for project with <50k LOC
```

**Related:**
- [Performance Tuning](./configuration.md#performance-tuning)
- [Embedding Providers Comparison](./configuration.md#embedding-provider-comparison)

---

### Issue: File Exclusion Patterns Not Working

**Symptoms:**
- `node_modules/` or `vendor/` files still being indexed
- Test files included despite `**/*_test.go` exclusion
- Unexpected files in index

**Cause:**
Exclusion patterns are glob patterns, not regex. Common mistakes:
- Using regex syntax in glob patterns
- Wrong pattern syntax for nested directories
- Pattern doesn't match file path structure
- Patterns case-sensitive but filenames use different case

**Solution:**

1. **Check current exclusion patterns:**
   ```bash
   cie config show | grep -A 10 exclude
   ```

2. **Use correct glob syntax:**

   **Common patterns:**
   ```yaml
   # .cie/project.yaml
   exclude:
     # Directories (note the /**)
     - "node_modules/**"       # All files under node_modules/
     - "vendor/**"
     - ".git/**"

     # File patterns
     - "**/*_test.go"          # All Go test files
     - "**/*.test.js"          # All JS test files
     - "*.md"                  # Markdown files in root only
     - "**/*.md"               # All markdown files (recursive)

     # Specific files
     - "README.md"             # Specific file in root
     - "docs/generated.md"     # Specific file with path
   ```

   **Common mistakes:**
   ```yaml
   # No Wrong
   exclude:
     - "node_modules"     # Only matches file named "node_modules", not directory contents
     - "*.test.*"         # Only matches root level
     - "test/.*"          # Regex syntax doesn't work

   # Yes Correct
   exclude:
     - "node_modules/**"  # Matches all contents
     - "**/*.test.*"      # Matches all test files recursively
     - "test/**"          # Glob syntax
   ```

3. **Test patterns before full reindex:**
   ```bash
   # List files that will be indexed
   find . -type f \( -name "*.go" -o -name "*.py" -o -name "*.js" -o -name "*.ts" -o -name "*.tsx" \) \
     -not -path "./node_modules/*" \
     -not -path "./vendor/*"
   ```

4. **Reindex after fixing patterns:**
   ```bash
   # Full reindex
   rm -rf .cie/db
   cie index

   # Or incremental (may not catch all changes)
   cie index
   ```

**Verify:**
```bash
cie status
# Check file count matches expectations

# Check for specific excluded files
cie query "?[name, file_path] := *cie_function{name, file_path}, file_path ~ 'node_modules'"
# Should return empty if node_modules excluded correctly
```

**Related:**
- [Exclusion Patterns Guide](./configuration.md#exclusion-patterns)
- [Glob Pattern Syntax](https://en.wikipedia.org/wiki/Glob_(programming))

---

## Query Errors

### Issue: Index Not Found

**Symptoms:**
- `database not initialized`
- `no such table: cie_function`
- `cie status` shows `Index: Not found`

**Cause:**
CIE hasn't indexed the project yet, or the `.cie/db` directory was deleted/corrupted.

**Solution:**

1. **Check if index exists:**
   ```bash
   ls -la .cie/db
   # Should show database files
   ```

   If directory doesn't exist:
   ```bash
   cie init    # Initialize project
   cie index   # Create index
   ```

2. **If index exists but corrupted:**
   ```bash
   # Backup corrupted index (optional)
   mv .cie/db .cie/db.backup

   # Rebuild index
   cie index
   ```

3. **Check you're in correct directory:**
   ```bash
   pwd
   # Should be project root with .cie/ directory

   # If wrong directory:
   cd /path/to/your/project
   cie status
   ```

4. **Check config file exists:**
   ```bash
   cat .cie/project.yaml
   # Should show config
   ```

   If missing:
   ```bash
   cie init
   ```

**Verify:**
```bash
cie status
# Should show:
# Index: OK (X functions, last indexed: ...)
```

**Related:**
- [Initialization Guide](./getting-started.md#initializing-a-project)
- [Index Management](./getting-started.md#reindexing)

---

### Issue: Connection Refused to Edge Cache

**Symptoms:**
- `connection refused` when querying
- `http://localhost:8080 connection refused`
- MCP tools timeout

**Cause:**
CIE Edge Cache server is not running. The Edge Cache serves queries in server mode.

**Solution:**

**For local CLI use:**

CIE CLI doesn't require Edge Cache server. It uses embedded database directly:

```bash
# This works without server:
cie status
cie index
```

**For MCP server mode:**

1. **Check if Edge Cache is running:**
   ```bash
   curl http://localhost:8080/health
   ```

2. **Start Edge Cache (if needed):**
   ```bash
   # In separate terminal:
   cie-edge-cache --port 8080

   # Or as background process:
   cie-edge-cache --port 8080 &
   ```

3. **Check config points to correct URL:**
   ```bash
   cie config show | grep base_url
   ```

   Fix if wrong:
   ```yaml
   # .cie/project.yaml
   edge_cache:
     base_url: "http://localhost:8080"
   ```

**Note:** Most users don't need Edge Cache. Only required for:
- MCP server mode in distributed setup
- Multiple projects sharing one index
- Network-accessible query server

**Verify:**
```bash
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

**Related:**
- [Architecture: Edge Cache](./architecture.md#edge-cache)
- [Deployment Modes](./getting-started.md#deployment-modes)

---

### Issue: Empty Search Results

**Symptoms:**
- `cie_semantic_search` returns no results
- `cie_find_function` finds nothing
- `cie status` shows functions indexed but queries return empty

**Cause:**
Multiple possible causes:
1. Query doesn't match indexed content
2. Minimum similarity threshold too high
3. Index missing embeddings
4. Wrong project path
5. Query syntax error

**Solution:**

1. **Check index has content:**
   ```bash
   cie status
   # Should show: Functions indexed: > 0
   ```

2. **Try broader queries:**

   **For semantic search:**
   ```bash
   # Too specific (may find nothing)
   cie query --semantic "Redis connection pool with retry logic"

   # More general (better)
   cie query --semantic "database connection"
   ```

3. **Lower similarity threshold:**
   ```bash
   # Default min_similarity is 0.7 (70%)
   cie query --semantic "authentication" --min-similarity 0.5
   ```

4. **Check if embeddings were generated:**
   ```bash
   cie index --debug 2>&1 | grep "Generating embeddings"
   # Should show embedding generation completed
   ```

   If embeddings missing:
   ```bash
   # Reindex with embeddings
   rm -rf .cie/db
   cie index
   ```

5. **Use English queries (important!):**

   CIE keyword boosting matches English function names:
   ```bash
   # No May find nothing
   cie query --semantic "lógica de autenticación"

   # Yes Better
   cie query --semantic "authentication logic"
   ```

6. **Try different query types:**
   ```bash
   # Semantic search
   cie query --semantic "http handler"

   # Text search (literal matching)
   cie query --text "Handler" --mode substring

   # Function name
   cie query --function "HandleAuth"

   # List all functions
   cie query "?[name, file_path] := *cie_function{name, file_path}"
   ```

**Verify:**
```bash
# Should return results:
cie query --semantic "function" --min-similarity 0.3
```

**Related:**
- [Query Types](./tools-reference.md#query-types)
- [Semantic Search Tips](./tools-reference.md#cie_semantic_search)
- [Query Syntax](./architecture.md#query-language)

---

### Issue: Timeout Errors

**Symptoms:**
- `query timeout exceeded`
- `context deadline exceeded (30s)`
- Queries hang and eventually fail

**Cause:**
Complex queries on large indexes can exceed default 30-second timeout. Common with:
- Semantic search across >100k functions
- Complex Datalog queries with many joins
- Full-text search without filters

**Solution:**

1. **Narrow query scope with filters:**
   ```bash
   # Too broad (may timeout)
   cie query --semantic "handler"

   # Narrower (faster)
   cie query --semantic "handler" --path "internal/http"
   ```

2. **Use more specific queries:**
   ```bash
   # Broad (slow)
   cie query --text "func"

   # Specific (fast)
   cie query --function "HandleAuth"
   ```

3. **Limit result count:**
   ```bash
   # Returns first 10 instead of all matches
   cie query --semantic "handler" --limit 10
   ```

4. **Optimize index (rebuild):**
   ```bash
   # Compact database
   rm -rf .cie/db
   cie index
   ```

5. **Check index size:**
   ```bash
   du -sh .cie/db
   # If >1GB, consider excluding more files
   ```

6. **Increase timeout (if query is legitimately complex):**
   ```yaml
   # .cie/project.yaml
   query:
     timeout: 60s  # Increase from default 30s
   ```

**Verify:**
```bash
# Should complete quickly:
time cie query --semantic "handler" --limit 10
# Should be <5 seconds
```

**Related:**
- [Performance Tuning](./configuration.md#query-performance)
- [Query Optimization](./architecture.md#query-optimization)

---

### Issue: CozoDB Query Errors

**Symptoms:**
- `compilation error in Datalog`
- `relation not found: cie_function`
- `type mismatch in query`
- `invalid CozoScript syntax`

**Cause:**
Raw CozoDB queries use Datalog syntax. Common errors:
- Wrong relation name (typo in table name)
- Missing fields in query
- Type mismatch (e.g., comparing string to number)
- Invalid CozoScript syntax

**Solution:**

1. **Use CIE query tools instead of raw CozoScript:**

   Instead of:
   ```bash
   # No Raw CozoScript (error-prone)
   cie query "?[name] := *cie_func{name}"  # Wrong table name
   ```

   Use:
   ```bash
   # Yes CIE query functions (validated)
   cie query --function "HandleAuth"
   cie query --semantic "authentication"
   ```

2. **Check relation names:**

   Available relations:
   - `cie_function` - Function definitions
   - `cie_call` - Function calls (caller → callee)
   - `cie_type` - Type definitions
   - `cie_file` - File metadata

   ```bash
   # List all functions
   cie query "?[name, file_path] := *cie_function{name, file_path}"
   ```

3. **Verify field names:**
   ```bash
   # Show schema
   cie query "?[relations] := show_relations{relations}"

   # Show fields in cie_function
   cie query "?[columns] := show_columns{table: 'cie_function', columns}"
   ```

4. **Check query syntax:**

   Common syntax rules:
   - Variables start with lowercase or `_`
   - Relations use `*table_name{field1, field2}`
   - Patterns use `:-` (implies)
   - Multiple conditions use `,` (and) or `;` (or)

   ```bash
   # Yes Correct syntax
   cie query "?[name, file] := *cie_function{name, file_path: file}"

   # No Wrong
   cie query "?[name, file] := *cie_function{name file}"  # Missing :
   ```

**Verify:**
```bash
# Should return results without errors:
cie query "?[name] := *cie_function{name} :limit 5"
```

**Related:**
- [CozoDB Query Language](https://docs.cozodb.org/en/latest/queries.html)
- [CIE Schema](./architecture.md#database-schema)
- [Query Examples](./tools-reference.md#query-examples)

---

## MCP Integration Issues

### Issue: MCP Server Won't Start

**Symptoms:**
- `cie --mcp` hangs without output
- `cie --mcp` exits immediately with error
- MCP server starts but tools don't appear in Claude Code/Cursor

**Cause:**
Common issues:
- Config file not found
- Port already in use
- Invalid MCP config in Claude Code/Cursor
- Project not indexed

**Solution:**

1. **Check project is initialized:**
   ```bash
   ls .cie/project.yaml
   # Should exist

   cie status
   # Should show index exists
   ```

   If not:
   ```bash
   cie init
   cie index
   ```

2. **Test server directly:**
   ```bash
   cie --mcp
   # Should output:
   # [MCP] Server started on stdio
   # [MCP] Project: /path/to/project
   # [MCP] Waiting for requests...
   ```

   If errors, check:
   ```bash
   cie config validate
   # Should pass
   ```

3. **Check Claude Code/Cursor config:**

   **Claude Code** (`~/.claude/mcp.json` or project `.claude/mcp.json`):
   ```json
   {
     "mcpServers": {
       "cie": {
         "command": "cie",
         "args": ["--mcp"],
         "cwd": "/absolute/path/to/your/project"
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
         "args": ["--mcp"]
       }
     }
   }
   ```

4. **Test MCP protocol manually:**
   ```bash
   # Send test request (MCP uses JSON-RPC over stdio)
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | cie --mcp
   # Should return list of tools
   ```

5. **Check server logs:**
   ```bash
   # Run with debug logging
   cie --mcp --debug
   ```

**Verify:**
```bash
# In Claude Code/Cursor, you should see CIE tools:
# - cie_semantic_search
# - cie_find_function
# - cie_list_endpoints
# (and 20+ more tools)
```

**Related:**
- [MCP Integration Guide](./mcp-integration.md)
- [Claude Code Configuration](./mcp-integration.md#claude-code-setup)
- [Cursor Configuration](./mcp-integration.md#cursor-setup)

---

### Issue: Port Already in Use

**Symptoms:**
- `listen tcp :8080: bind: address already in use`
- Server fails to start with port conflict error
- MCP server can't bind to configured port

**Cause:**
Another process is using the port CIE needs. Common culprits:
- Previous CIE instance still running
- Another development server (webpack, vite, etc.)
- Another MCP server

**Solution:**

1. **Find process using the port:**
   ```bash
   # macOS/Linux
   lsof -i :8080
   # Shows: COMMAND PID USER ...

   # Or with netstat
   netstat -tulpn | grep :8080
   ```

2. **Kill the process:**
   ```bash
   # Replace PID with actual process ID from lsof
   kill <PID>

   # Or forcefully:
   kill -9 <PID>
   ```

3. **Change CIE port:**
   ```yaml
   # .cie/project.yaml
   server:
     port: 8081  # Use different port
   ```

   Then update MCP config:
   ```json
   {
     "mcpServers": {
       "cie": {
         "command": "cie",
         "args": ["--mcp", "--port", "8081"]
       }
     }
   }
   ```

4. **Check for zombie processes:**
   ```bash
   # Find all cie processes
   ps aux | grep cie

   # Kill all cie processes
   pkill cie
   ```

**Note:** Most users use stdio mode (`cie --mcp`) which doesn't require a port. Only network mode needs ports.

**Verify:**
```bash
# Port should be free:
lsof -i :8080
# Should show nothing

cie --mcp
# Should start without port conflict
```

**Related:**
- [Server Configuration](./configuration.md#server-configuration)
- [MCP Transport Modes](./mcp-integration.md#transport-modes)

---

### Issue: Config Not Found in MCP Mode

**Symptoms:**
- `config file not found: .cie/project.yaml`
- MCP server starts but has no project context
- Tools fail with "project not initialized"

**Cause:**
Working directory is not the project root. MCP servers need to run from the directory containing `.cie/project.yaml`.

**Solution:**

1. **Check working directory:**
   ```bash
   ls .cie/project.yaml
   # Should exist
   ```

2. **Set `cwd` in MCP config:**

   **Claude Code** (`~/.claude/mcp.json`):
   ```json
   {
     "mcpServers": {
       "cie": {
         "command": "cie",
         "args": ["--mcp"],
         "cwd": "/absolute/path/to/your/project"  // ← Important!
       }
     }
   }
   ```

   **Cursor** (run from project root or use shell script):
   ```bash
   # Create wrapper script: ~/bin/cie-mcp.sh
   #!/bin/bash
   cd /absolute/path/to/your/project
   exec cie --mcp "$@"
   ```

   Then in `.cursor/mcp.json`:
   ```json
   {
     "mcpServers": {
       "cie": {
         "command": "/Users/yourname/bin/cie-mcp.sh"
       }
     }
   }
   ```

3. **Initialize project if needed:**
   ```bash
   cd /path/to/your/project
   cie init
   cie index
   ```

**Verify:**
```bash
cd /path/to/your/project
cie --mcp
# Should show: Project: /path/to/your/project
```

**Related:**
- [MCP Configuration](./mcp-integration.md#configuration)
- [Project Initialization](./getting-started.md#initializing-a-project)

---

### Issue: Tools Not Available in AI Assistant

**Symptoms:**
- MCP server starts successfully
- No CIE tools appear in Claude Code/Cursor
- Assistant doesn't recognize CIE commands
- Tools list is empty

**Cause:**
Common issues:
- MCP config syntax error (JSON invalid)
- Config file in wrong location
- Tool discovery failed
- Index not built (tools work but return empty results)

**Solution:**

1. **Validate MCP config JSON:**
   ```bash
   # Check JSON syntax
   cat ~/.claude/mcp.json | jq .
   # Or for Cursor:
   cat .cursor/mcp.json | jq .

   # Should parse without errors
   ```

2. **Verify config location:**

   **Claude Code** looks in:
   - `$PWD/.claude/mcp.json` (project-level)
   - `~/.claude/mcp.json` (global)

   **Cursor** looks in:
   - `$PWD/.cursor/mcp.json` (project-level)

3. **Restart the AI assistant:**

   Changes to MCP config require restart:
   - Claude Code: Restart CLI session
   - Cursor: Restart Cursor application

4. **Test tool discovery:**
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | cie --mcp
   # Should return JSON with tools array containing 20+ tools
   ```

5. **Check server logs:**
   ```bash
   # Claude Code logs (macOS)
   tail -f ~/Library/Logs/Claude/mcp.log

   # Cursor logs (check Cursor's developer tools)
   ```

6. **Verify index exists:**
   ```bash
   cie status
   # Should show functions indexed
   ```

   Tools will load but return empty results if index is empty.

**Verify:**
In Claude Code/Cursor, type:
```
Use CIE to find functions related to authentication
```

Assistant should respond with CIE tool results.

**Related:**
- [MCP Setup Guide](./mcp-integration.md)
- [Troubleshooting MCP](./mcp-integration.md#troubleshooting)
- [Tool Reference](./tools-reference.md)

---

## Performance Problems

### Issue: Slow Semantic Search

**Symptoms:**
- `cie_semantic_search` takes >10 seconds
- MCP tool requests timeout
- Queries fast in CLI but slow in MCP

**Cause:**
Semantic search requires:
1. Embedding generation for query (50-200ms)
2. Vector similarity computation across all functions (depends on index size)
3. Ranking and filtering results

Large indexes (>50k functions) with no filters can be slow.

**Solution:**

1. **Use filters to narrow search:**
   ```bash
   # Slow (searches everything)
   cie_semantic_search query="authentication"

   # Fast (searches specific path)
   cie_semantic_search query="authentication" path_pattern="internal/auth"
   ```

2. **Use role filters:**
   ```bash
   # Only search handlers (smaller subset)
   cie_semantic_search query="user login" role="handler"

   # Exclude tests (reduces search space)
   cie_semantic_search query="database query" role="source"
   ```

3. **Lower result limit:**
   ```bash
   # Default returns 10, but still computes similarity for all functions
   # Lower limit doesn't help much, but raising it makes it worse:
   cie_semantic_search query="handler" limit=5  # Still slow if index is large
   ```

4. **Optimize embedding provider:**

   **Use local Ollama (fastest):**
   ```yaml
   # .cie/project.yaml
   embedding:
     provider: ollama
     ollama:
       url: "http://localhost:11434"
       model: "nomic-embed-text"
   ```

   **If using OpenAI, use smaller model:**
   ```yaml
   embedding:
     provider: openai
     openai:
       model: "text-embedding-3-small"  # Faster than 3-large
   ```

5. **Exclude large directories from index:**
   ```yaml
   # .cie/project.yaml
   exclude:
     - "vendor/**"
     - "node_modules/**"
     - "test/fixtures/**"
     - "**/*.test.go"
   ```

   Then reindex:
   ```bash
   rm -rf .cie/db
   cie index
   ```

6. **Check index size:**
   ```bash
   cie status
   # If Functions indexed: >100k, consider excluding more files
   ```

**Verify:**
```bash
time cie query --semantic "authentication" --path "internal"
# Should complete in <5 seconds
```

**Related:**
- [Query Performance](./configuration.md#query-performance)
- [Semantic Search Tips](./tools-reference.md#semantic-search-tips)

---

### Issue: High Memory Usage

**Symptoms:**
- CIE process uses >4GB RAM
- System slows down when querying
- Out of memory errors during queries

**Cause:**
Large indexes load significant data into memory:
- Function embeddings (768-dimensional vectors)
- AST data
- CozoDB database cache

**Solution:**

1. **Check index size:**
   ```bash
   cie status
   du -sh .cie/db

   # If >1GB, consider optimization
   ```

2. **Reduce indexed functions:**
   ```yaml
   # .cie/project.yaml - Exclude more aggressively
   exclude:
     - "**/*_test.go"          # Exclude all tests
     - "**/*.test.js"
     - "vendor/**"
     - "node_modules/**"
     - "third_party/**"
     - "docs/**"               # Exclude non-code
     - "examples/**"
   ```

3. **Use smaller embedding dimension:**
   ```yaml
   # .cie/project.yaml
   embedding:
     provider: ollama
     ollama:
       model: "mxbai-embed-large"  # 1024-dim
       # Instead of nomic-embed-text (768-dim)
   ```

   Note: This requires reindexing and may affect search quality.

4. **Limit database cache size:**
   ```yaml
   # .cie/project.yaml
   database:
     cache_size_mb: 512  # Default 1024
   ```

5. **Close CIE when not in use:**
   ```bash
   # If running MCP server:
   pkill cie

   # Restart when needed
   cie --mcp
   ```

**Verify:**
```bash
# Monitor memory usage
ps aux | grep cie
# RSS column shows memory in KB
```

**Related:**
- [System Requirements](./getting-started.md#system-requirements)
- [Performance Tuning](./configuration.md#performance-tuning)

---

### Issue: Large Index Size

**Symptoms:**
- `.cie/db/` directory is >1GB
- Disk space running low
- Backup/sync takes long time

**Cause:**
Index stores:
- Function ASTs (syntax trees)
- Embeddings (768-1024 dimensions × number of functions)
- Call graph relationships
- Metadata

Large projects with many functions create large indexes.

**Solution:**

1. **Check what's taking space:**
   ```bash
   du -sh .cie/db/*
   # Identify large components
   ```

2. **Exclude test files:**
   ```yaml
   # .cie/project.yaml
   exclude:
     - "**/*_test.go"
     - "**/*.test.js"
     - "**/*.test.ts"
     - "**/*.spec.ts"
   ```

   Tests can be 30-50% of codebase.

3. **Exclude generated code:**
   ```yaml
   exclude:
     - "**/*.pb.go"           # Protobuf generated
     - "**/*.generated.go"
     - "**/mock_*.go"         # Mocks
     - "**/*.gen.ts"
   ```

4. **Exclude vendor/dependencies:**
   ```yaml
   exclude:
     - "vendor/**"
     - "node_modules/**"
     - "third_party/**"
   ```

5. **Compact database (doesn't usually help much):**
   ```bash
   # Rebuild index (may reduce fragmentation)
   rm -rf .cie/db
   cie index
   ```

6. **Use `.cieignore` (if available):**
   ```bash
   # Similar to .gitignore
   echo "**/*_test.go" >> .cieignore
   echo "vendor/" >> .cieignore
   ```

**Typical index sizes:**
- Small project (5k LOC): ~10MB
- Medium project (50k LOC): ~50-100MB
- Large project (500k LOC): ~500MB-1GB

**Verify:**
```bash
du -sh .cie/db
# Should decrease after excluding files
```

**Related:**
- [Index Management](./getting-started.md#index-management)
- [Exclusion Patterns](./configuration.md#exclusion-patterns)

---

## Advanced Debugging

### Verbose Logging

Enable debug logging for detailed diagnostics:

```bash
# Debug indexing
cie index --debug

# Debug queries
cie query --debug "?[name] := *cie_function{name}"

# Debug MCP server
cie --mcp --debug
```

### Database Inspection

Query CozoDB directly for debugging:

```bash
# List all relations (tables)
cie query "?[relations] := show_relations{relations}"

# Show schema for a relation
cie query "?[columns] := show_columns{table: 'cie_function', columns}"

# Count functions by language
cie query "?[language, count] := *cie_function{language}, count = count(*)"

# Find functions without embeddings
cie query "?[name, file] := *cie_function{name, file_path: file, embedding}, is_null(embedding)"
```

### Configuration Debugging

```bash
# Show effective configuration
cie config show

# Validate configuration
cie config validate

# Show environment variables
env | grep CIE_

# Test embedding provider
curl -X POST http://localhost:11434/api/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model":"nomic-embed-text","prompt":"test"}'
```

### Performance Profiling

```bash
# Profile indexing
time cie index

# Profile query
time cie query --semantic "handler" --limit 10

# Memory profiling (Go)
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Log Files

Check system logs for CIE errors:

**macOS:**
```bash
# System logs
log show --predicate 'process == "cie"' --last 1h

# Claude Code logs
tail -f ~/Library/Logs/Claude/mcp.log
```

**Linux:**
```bash
# Systemd journal
journalctl -u cie -f

# Syslog
tail -f /var/log/syslog | grep cie
```

---

## Getting Help

If you can't find a solution here:

### 1. Search Existing Issues

Check if your problem is already reported:

[CIE GitHub Issues](https://github.com/kraklabs/cie/issues?q=is%3Aissue+label%3Acie)

### 2. Ask in Discussions

For questions and support:

[CIE Discussions](https://github.com/kraklabs/cie/discussions)

### 3. Report a Bug

If you've found a bug, please report it:

[Create New Issue](https://github.com/kraklabs/cie/issues/new?labels=cie,bug)

**Include this information:**
```bash
# System info
cie --version
go version
uname -a

# Configuration
cie config show

# Error log
cie index --debug 2>&1 | tail -100

# Index status
cie status
```

---

## Quick Reference

### Common Commands

```bash
# Diagnostics
cie --version
cie status
cie config show
cie config validate

# Infrastructure management
cie start                    # Start Docker containers
cie stop                     # Stop containers (preserves data)
cie reset --yes              # Delete indexed data
cie reset --yes --docker     # Full reset including Docker volumes

# Index management
cie init
cie index
cie index --debug
cie reset --yes && cie index  # Full reindex

# Querying
cie query --semantic "query text"
cie query --function "FunctionName"
cie query --text "literal text"

# MCP server
cie --mcp
cie --mcp --debug
```

### Common Fixes

| Symptom | Quick Fix |
|---------|-----------|
| Library not found | Download libcozo_c from [CozoDB releases](https://github.com/cozodb/cozo/releases), copy to `/usr/local/lib/` |
| No functions indexed | Check file extensions (`.go`, `.py`, `.js`, `.ts`, `.tsx`) |
| Ollama connection failed | `cie start` or check Docker is running |
| Index corrupted | `cie reset --yes --docker && cie start && cie index` |
| Slow queries | Add `path_pattern` filter to narrow scope |
| Empty results | Lower `min_similarity` to 0.5 or try English query |
| Port conflict | `cie stop` then `cie start` |
| Config not found | `cd /path/to/project && cie init` |
| Stop infrastructure | `cie stop` (preserves data) |
| Full reset | `cie reset --yes --docker` (deletes everything) |

---

**Document Version:** 1.0
**Last Updated:** 2026-01-13
**CIE Version:** v0.1.0+
