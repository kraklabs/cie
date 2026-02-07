# Exit Codes

CIE uses semantic exit codes following Unix conventions to enable robust scripting and CI/CD integration.

## Quick Reference

| Code | Name | Description | Example Scenario |
|------|------|-------------|------------------|
| 0 | Success | Command completed successfully | Indexing finished, query returned results |
| 1 | Config Error | Configuration issues | Missing `.cie/project.yaml`, invalid YAML syntax |
| 2 | Database Error | Database operations failed | Database locked, corrupted index, failed transaction |
| 3 | Network Error | Network/API connectivity issues | Embedding API timeout, connection refused |
| 4 | Input Error | Invalid user input | Invalid project name, malformed query |
| 5 | Permission Error | Insufficient permissions | Cannot write to index directory, read-only filesystem |
| 6 | Not Found | Resource not found | Project not indexed, function doesn't exist |
| 10 | Internal Error | Bug or unexpected error | Please report these at github.com/kraklabs/cie/issues |

## Detailed Descriptions

### Exit Code 0: Success

The command executed successfully. All requested operations completed without errors.

```bash
cie index
echo $?  # Output: 0
```

### Exit Code 1: Config Error

Configuration-related errors such as missing or malformed configuration files.

**Common causes:**
- Missing `.cie/project.yaml` file
- Invalid YAML syntax in configuration
- Missing required configuration fields
- Unsupported configuration values

**Example error output:**
```
Error: Cannot load CIE configuration
Cause: The config file .cie/project.yaml is missing
Fix:   Run 'cie init' to create a new configuration
```

**Resolution:**
```bash
# Initialize a new project
cie init

# Or check existing configuration
cat .cie/project.yaml
```

### Exit Code 2: Database Error

Database operation failures including file locks, corruption, or transaction errors.

**Common causes:**
- Database file locked by another CIE instance
- Corrupted index data
- Disk full or write failures
- RocksDB checkpoint issues

**Example error output:**
```
Error: Cannot open CIE database
Cause: The database file is locked by another process
Fix:   Close other CIE instances or run: cie reset --yes
```

**Resolution:**
```bash
# Check for running CIE processes
pgrep -f cie

# Force reset if needed (destroys index)
cie reset --yes
```

### Exit Code 3: Network Error

Network connectivity or API communication failures.

**Common causes:**
- Embedding API (Ollama, OpenAI) unreachable
- Connection timeout
- DNS resolution failures
- API rate limiting

**Example error output:**
```
Error: Cannot connect to embedding API
Cause: Connection timed out after 30 seconds
Fix:   Check your network connection and verify OLLAMA_HOST is correct
```

**Resolution:**
```bash
# Verify embedding service is running
curl http://localhost:11434/api/tags  # For Ollama

# Check environment variables
echo $OLLAMA_HOST
```

### Exit Code 4: Input Error

Invalid user input or argument validation failures.

**Common causes:**
- Invalid project name format
- Malformed query syntax
- Missing required arguments
- Out-of-range parameter values

**Example error output:**
```
Error: Invalid project name
Cause: Project name must contain only alphanumeric characters and hyphens
Fix:   Use a name like 'my-project' or 'myproject123'
```

**Resolution:**
```bash
# Check command help for valid arguments
cie index --help
```

### Exit Code 5: Permission Error

Insufficient filesystem or operation permissions.

**Common causes:**
- Cannot write to `~/.cie/` directory
- Read-only project directory
- Insufficient privileges for database operations

**Example error output:**
```
Error: Cannot write to index directory
Cause: Permission denied for ~/.cie/data/
Fix:   Run with appropriate permissions or change the index directory
```

**Resolution:**
```bash
# Check directory permissions
ls -la ~/.cie/

# Fix permissions if needed
chmod 755 ~/.cie
```

### Exit Code 6: Not Found

Requested resource does not exist.

**Common causes:**
- Project not yet indexed
- Function or type not in index
- Referenced file doesn't exist

**Example error output:**
```
Error: Project not found
Cause: No project named 'myproject' exists in the index
Fix:   Run 'cie status' to list indexed projects
```

**Resolution:**
```bash
# List indexed projects
cie status

# Index the project if needed
cie index
```

### Exit Code 10: Internal Error

Unexpected errors indicating a bug in CIE. These should be reported.

**Common causes:**
- Assertion failures
- Unexpected nil values
- Unhandled error conditions
- Parser crashes

**Example error output:**
```
Error: Unexpected nil pointer
Cause: The function indexer returned nil unexpectedly
Fix:   This is a bug. Please report it at github.com/kraklabs/cie/issues
```

**Resolution:**
Report the issue with:
- CIE version (`cie --version`)
- Full error output
- Steps to reproduce
- Relevant log files

## Shell Script Examples

### Basic Error Handling

```bash
#!/bin/bash

cie index
exit_code=$?

if [ $exit_code -ne 0 ]; then
    echo "CIE indexing failed with exit code: $exit_code"
    exit $exit_code
fi

echo "Indexing completed successfully"
```

### Handling Specific Errors

```bash
#!/bin/bash

cie index 2>&1
exit_code=$?

case $exit_code in
    0)
        echo "Success: Indexing completed"
        ;;
    1)
        echo "Config error: Check .cie/project.yaml"
        cie init  # Attempt to fix
        ;;
    2)
        echo "Database error: Attempting reset..."
        cie reset --yes
        cie index
        ;;
    3)
        echo "Network error: Check embedding service"
        exit 1
        ;;
    6)
        echo "Not found: Initializing new project..."
        cie init
        cie index
        ;;
    10)
        echo "Internal error: Please report this bug"
        exit 1
        ;;
    *)
        echo "Unknown error: $exit_code"
        exit 1
        ;;
esac
```

### CI/CD Integration

```bash
#!/bin/bash
# ci-index.sh - CIE indexing for CI pipelines

set -e

# Initialize if needed
if [ ! -f ".cie/project.yaml" ]; then
    cie init
fi

# Index with explicit error handling
if ! cie index; then
    exit_code=$?

    # Transient errors (network) may succeed on retry
    if [ $exit_code -eq 3 ]; then
        echo "Network error, retrying in 5s..."
        sleep 5
        cie index
    else
        echo "CIE indexing failed (exit code: $exit_code)"
        exit $exit_code
    fi
fi

echo "CIE index ready"
```

### Retry Logic for Transient Errors

```bash
#!/bin/bash
# retry-index.sh - Retry CIE indexing for transient failures

MAX_RETRIES=3
RETRY_DELAY=5

# Exit codes that are worth retrying
TRANSIENT_ERRORS=(2 3)  # Database and Network errors

attempt=1
while [ $attempt -le $MAX_RETRIES ]; do
    cie index 2>&1
    exit_code=$?

    if [ $exit_code -eq 0 ]; then
        echo "Indexing succeeded on attempt $attempt"
        exit 0
    fi

    # Check if error is transient
    is_transient=0
    for code in "${TRANSIENT_ERRORS[@]}"; do
        if [ $exit_code -eq $code ]; then
            is_transient=1
            break
        fi
    done

    if [ $is_transient -eq 0 ]; then
        echo "Non-transient error (exit code: $exit_code), not retrying"
        exit $exit_code
    fi

    echo "Attempt $attempt failed (exit code: $exit_code), retrying in ${RETRY_DELAY}s..."
    sleep $RETRY_DELAY
    ((attempt++))
done

echo "Max retries exceeded"
exit 1
```

## Best Practices

1. **Always check exit codes** in scripts and CI/CD pipelines
2. **Distinguish transient vs permanent errors** - Network (3) and Database (2) errors may be transient; Config (1) and Input (4) errors are usually permanent
3. **Log full error output** - CIE provides structured error messages with cause and fix suggestions
4. **Report internal errors** - Exit code 10 indicates a bug; please report it
5. **Use `set -e` carefully** - Consider explicit error handling for better diagnostics
6. **Implement retries for network operations** - Embedding APIs may have temporary failures

## Related

- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions
- [Configuration](configuration.md) - Configuration reference
- [CLI Reference](cli-reference.md) - All CLI commands and options
