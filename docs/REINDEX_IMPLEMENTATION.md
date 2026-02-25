# CIE Reindex and Auto-Reindex Implementation

This document describes the implementation of MCP tools for reindexing and smart auto-reindex capabilities that work while the MCP server is running.

## Overview

The implementation solves the core issue where CozoDB (with RocksDB backend) locks the database, preventing `cie index` commands from other terminals while the MCP server is running.

### Solution: Cooperative Lock/Release with State Machine

The implementation uses a **proper state machine** to ensure safe transitions:

```
Ready → Draining → Indexing → Reopening → Ready
```

**State Transitions:**
1. **Ready**: Normal operation, queries allowed
2. **Draining**: New queries blocked, waiting for active queries to complete
3. **Indexing**: All queries finished, DB lock released, reindex running
4. **Reopening**: Reindex complete, reopening DB connection

## Concurrency Safety

### Query Draining

All database operations (`Query` and `Execute`) now:
1. Call `coordinator.AcquireRead(ctx)` before running
2. Run the actual database operation
3. Call `coordinator.ReleaseRead()` when done (via defer)

This ensures:
- **No in-flight queries** are interrupted during reindex
- **Graceful degradation**: New queries fail fast with "Indexing in progress" during reindex
- **Thread safety**: Uses sync.WaitGroup to track active queries

### State Machine Mutex

The `ReindexCoordinator` uses:
- **RWMutex** for state transitions
- **sync.Cond** for efficient waiting
- **30-second timeout** for draining (prevents indefinite hangs)

## New MCP Tools

### 1. `cie_reindex`

Triggers full or incremental reindex from the MCP client.

**Parameters:**
- `force` (bool): If true, performs a full reindex. Default: false (incremental)
- `paths` (string array, optional): Specific file paths to reindex
- `cancel_job_id` (string, optional): Cancel a running reindex

**Returns:**
- Job ID
- Status (completed/failed/cancelled)
- Statistics (files processed, functions found, types found, duration)

**Examples:**
```json
// Start incremental reindex
{
  "name": "cie_reindex",
  "arguments": {
    "force": false
  }
}

// Start full reindex
{
  "name": "cie_reindex",
  "arguments": {
    "force": true
  }
}

// Cancel running reindex
{
  "name": "cie_reindex",
  "arguments": {
    "cancel_job_id": "reindex-1234567890"
  }
}
```

### 2. `cie_index_config`

Configures smart auto-reindex behavior.

**Parameters:**
- `auto_reindex` (bool): Enable file watching
- `debounce_ms` (int): Delay before triggering reindex after file change (100-60000ms)
- `exclude_patterns` (string array): Glob patterns to ignore
- `watch_extensions` (string array): File extensions to watch
- `get_only` (bool): Return current config without modifying

**Example:**
```json
{
  "name": "cie_index_config",
  "arguments": {
    "auto_reindex": true,
    "debounce_ms": 5000,
    "exclude_patterns": ["*.log", "temp/**"]
  }
}
```

### 3. Extended `cie_index_status`

The existing tool now shows detailed state:

```
## Indexing State

✅ **Status:** Ready
- **State:** ready
📡 **Auto-reindex:** Enabled
⏳ **Pending Changes:** 3 files detected
🕐 **Last Reindex:** 2026-02-25T10:30:00Z
```

## Architecture

### Components

1. **`pkg/storage/reindex.go`**: ReindexCoordinator with state machine
2. **`pkg/ingestion/watcher.go`**: FileWatcher (detects only, no DB access)
3. **`pkg/tools/reindex.go`**: Reindex tool with cancellation
4. **`pkg/tools/index_config.go`**: Index config tool
5. **`pkg/storage/embedded.go`**: Modified for AcquireRead/ReleaseRead

### State Machine Flow

```
Tool Query:
  AcquireRead()
  ├─ If Ready: proceed with query
  ├─ If Draining/Reopening: wait
  └─ If Indexing: fail fast
  [Run Query]
  ReleaseRead()

Reindex Request:
  StartReindex()
  ├─ Set state to Draining
  ├─ Wait for activeQueries == 0 (or timeout)
  ├─ Set state to Indexing
  └─ Return job ID
  [Close DB, Run Reindex, Reopen DB]
  FinishReindex()
  └─ Set state to Ready, signal waiters
```

### File Watcher

The FileWatcher **only detects file changes** and queues them via `coordinator.QueueReindex()`. It **never accesses the database directly**.

The coordinator's job processor runs queued jobs when safe.

## Configuration

Auto-reindex configuration is stored in `.cie/project.yaml`:

```yaml
version: "1"
project_id: my-project

auto_reindex:
  enabled: true
  debounce_ms: 2000
  exclude_patterns:
    - "*.log"
    - "temp/**"
  watch_extensions:
    - ".go"
    - ".js"
    - ".ts"
```

## Usage

### Enable Auto-Reindex

```bash
# Via MCP tool
cie_index_config(auto_reindex=true, debounce_ms=5000)
```

### Manual Reindex

```bash
# Incremental (faster, only changed files)
cie_reindex(force=false)

# Full (complete rebuild)
cie_reindex(force=true)

# Cancel running reindex
cie_reindex(cancel_job_id="reindex-1234567890")
```

### Check Status

```bash
cie_index_status()
```

## Behavior During Indexing

When indexing is in progress:

1. **Query tools** (cie_grep, cie_search_text, etc.):
   - Return error immediately: "Indexing in progress (job: XYZ, started: ...)"
   - Include instructions to cancel if needed

2. **Allowed tools** (cie_index_status, cie_reindex, cie_index_config):
   - Continue to work normally
   - Can cancel running reindex

3. **File watcher**:
   - Continues detecting changes
   - Records pending changes
   - Queues reindex jobs for after current reindex completes

## Dependencies

- `github.com/fsnotify/fsnotify v1.7.0` - File system notifications

## Testing

### Manual Test Steps

1. **Start MCP server:**
   ```bash
   cie --mcp
   ```

2. **Check initial status:**
   ```
   cie_index_status()
   ```

3. **Start long-running query in another terminal:**
   ```
   cie_semantic_search(query="authentication")
   ```

4. **Trigger reindex while query running:**
   ```
   cie_reindex(force=false)
   ```
   - Should show "Draining active queries..." state
   - Should wait for query to complete before starting reindex

5. **Try query during reindex:**
   ```
   cie_grep(text="func main")
   ```
   - Should return "Indexing in progress" error immediately

6. **Cancel reindex:**
   ```
   cie_reindex(cancel_job_id="reindex-1234567890")
   ```

7. **Enable auto-reindex and test:**
   ```
   cie_index_config(auto_reindex=true)
   ```
   - Modify a file
   - Watch for auto-reindex to trigger after debounce period

### Concurrency Test

```bash
# Terminal 1: Start MCP server
cie --mcp

# Terminal 2: Continuous queries while reindexing
for i in {1..100}; do
  echo '{"name": "cie_grep", "arguments": {"text": "func"}}' | cie --mcp
done

# Terminal 3: Trigger reindex mid-queries
cie_reindex(force=true)
```

Expected: No crashes, queries either complete or fail gracefully with "Indexing in progress"

## Implementation Notes

### Race Condition Prevention

1. **No mid-query interruption**: AcquireRead blocks new queries during draining
2. **Timeout protection**: 30-second max wait for queries to complete
3. **Backend reference management**: CloseAndRelease returns reopen function to ensure atomicity
4. **Job queue**: File watcher queues jobs, doesn't spawn multiple concurrent reindexes

### Cancellation

- Uses `context.WithCancel` for the reindex operation
- Cancelled jobs clean up and reopen database
- Cancellation is cooperative (checks ctx.Done() periodically)

### Error Handling

- Database reopen failures are treated as critical errors
- File watcher errors are logged but non-fatal
- State machine prevents invalid transitions
