# CIE Testing Guide

## Overview

CIE uses a two-tier testing approach that provides flexibility for different testing scenarios:

1. **Unit Tests** - Fast in-memory tests that don't require CozoDB installation
2. **Integration Tests** - Tests with real CozoDB using Docker containers

This approach ensures tests can run in CI/CD without complex setup while also supporting comprehensive integration testing when needed.

## Running Tests

### Unit Tests Only

Unit tests use in-memory CozoDB backends and run by default:

```bash
cd modules/cie
go test ./...
```

Or with the short flag:

```bash
go test -short ./...
```

### Integration Tests

Integration tests require the `cozodb` build tag and CozoDB availability:

```bash
# Build test container image (first time only)
make docker-build-cie-test

# Run integration tests
cd modules/cie
go test -tags=cozodb ./...
```

### Running Tests in Container

If CozoDB is not available locally, tests can run inside a Docker container:

```bash
# From project root
make test-cie-integration-pkg PKG=./modules/cie/pkg/tools/...
```

The testcontainer infrastructure automatically:
- Builds the test image if missing
- Mounts the project directory
- Executes tests inside the container
- Cleans up after completion

## Coverage

### Checking Coverage

Generate and view test coverage reports:

```bash
# Generate coverage report
cd modules/cie
go test -coverprofile=coverage.out ./...

# View coverage summary
go tool cover -func=coverage.out

# View total coverage
go tool cover -func=coverage.out | grep total

# View coverage in browser
make test-coverage
```

### Coverage Targets

| Package | Target | Description |
|---------|--------|-------------|
| pkg/tools | >80% | Core CIE tools (semantic search, grep, analysis) |
| pkg/ingestion | >60% | Code parsing and indexing pipeline |
| pkg/storage | >80% | Storage backend abstractions |
| pkg/llm | >50% | LLM provider integrations |

### Coverage in CI

The CI pipeline automatically generates coverage reports:

```yaml
- name: Test with coverage
  run: |
    cd modules/cie
    go test -race -coverprofile=coverage.out -covermode=atomic ./...

- name: Upload to Codecov
  uses: codecov/codecov-action@v4
  with:
    files: coverage.out
    flags: unittests
```

## Writing Tests

### Unit Tests (No CozoDB Installation Required)

For most tests, use the in-memory backend provided by the testing helpers:

```go
package mypackage

import (
    "testing"

    cietest "github.com/kraklabs/cie/internal/testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMyFeature(t *testing.T) {
    // Create in-memory backend with schema
    backend := cietest.SetupTestBackend(t)

    // Insert test data
    cietest.InsertTestFunction(t, backend, "func1", "HandleAuth", "auth.go", 10, 25)
    cietest.InsertTestFile(t, backend, "file1", "auth.go", "abc123", "go", 1234)

    // Run your tests
    result := cietest.QueryFunctions(t, backend)
    require.Len(t, result.Rows, 1)
    assert.Equal(t, "func1", result.Rows[0][0])
    assert.Equal(t, "HandleAuth", result.Rows[0][1])
}
```

**Key features:**
- No build tags needed
- Runs everywhere (local, CI, Docker)
- Fast execution (in-memory)
- Automatic cleanup with `t.Cleanup()`

### Integration Tests (Require CozoDB)

For tests that need full CozoDB features, use the `cozodb` build tag:

```go
//go:build cozodb
// +build cozodb

package mypackage

import (
    "testing"

    cozodbtest "github.com/kraklabs/kraken/internal/testing/cozodb"
    cietest "github.com/kraklabs/cie/internal/testing"
)

func TestIntegrationFeature(t *testing.T) {
    // Skip if CozoDB unavailable
    cozodbtest.RequireCozoDB(t)

    // Create backend (will use real CozoDB if available)
    backend := cietest.SetupTestBackend(t)

    // Run integration tests...
}
```

**Build tags explained:**
- `//go:build cozodb` - Go 1.17+ syntax
- `// +build cozodb` - Backward compatibility

Tests with these tags only run when: `go test -tags=cozodb`

## Test Infrastructure

### Root-Level Infrastructure

Located at `/internal/testing/cozodb/`:

| Function | Purpose |
|----------|---------|
| `StartContainer(ctx)` | Start a CozoDB Docker container |
| `RequireCozoDB(t)` | Skip test if CozoDB unavailable |
| `RunInContainer(t, pkg, args...)` | Execute tests in Docker |
| `isDockerAvailable()` | Check if Docker is running |

**Key features:**
- Testcontainers-go integration
- Auto-builds Docker image if missing
- Colima support for macOS
- Graceful test skipping

### CIE Module Helpers

Located at `/modules/cie/internal/testing/`:

| Function | Purpose |
|----------|---------|
| `SetupTestBackend(t)` | Create in-memory CIE backend |
| `InsertTestFunction(t, backend, ...)` | Add test function |
| `InsertTestFile(t, backend, ...)` | Add test file |
| `InsertTestType(t, backend, ...)` | Add test type |
| `InsertTestDefines(t, backend, ...)` | Link file to function |
| `InsertTestCalls(t, backend, ...)` | Link caller to callee |
| `InsertTestImport(t, backend, ...)` | Add import statement |
| `QueryFunctions(t, backend)` | Get all functions |
| `QueryFiles(t, backend)` | Get all files |
| `QueryTypes(t, backend)` | Get all types |

## Testing Patterns

### Pattern 1: Simple Unit Test

```go
func TestParser(t *testing.T) {
    backend := cietest.SetupTestBackend(t)

    // Test parsing logic
    cietest.InsertTestFunction(t, backend, "f1", "Parse", "parser.go", 10, 20)

    result := cietest.QueryFunctions(t, backend)
    require.Len(t, result.Rows, 1)
}
```

### Pattern 2: Test with Relationships

```go
func TestCallGraph(t *testing.T) {
    backend := cietest.SetupTestBackend(t)

    // Setup: Create caller and callee
    cietest.InsertTestFunction(t, backend, "caller", "Main", "main.go", 5, 10)
    cietest.InsertTestFunction(t, backend, "callee", "Helper", "util.go", 15, 20)
    cietest.InsertTestCalls(t, backend, "call1", "caller", "callee")

    // Test call graph traversal
    // ...
}
```

### Pattern 3: Test with File Context

```go
func TestFileAnalysis(t *testing.T) {
    backend := cietest.SetupTestBackend(t)

    // Create file and function
    cietest.InsertTestFile(t, backend, "file1", "auth.go", "hash123", "go", 500)
    cietest.InsertTestFunction(t, backend, "func1", "Auth", "auth.go", 10, 30)
    cietest.InsertTestDefines(t, backend, "def1", "file1", "func1")

    // Test file-level analysis
    // ...
}
```

### Pattern 4: Integration Test with Container

```go
//go:build cozodb
// +build cozodb

func TestFullPipeline(t *testing.T) {
    // Option 1: Require CozoDB locally
    cozodbtest.RequireCozoDB(t)

    backend := cietest.SetupTestBackend(t)
    // Full integration test...
}

func TestInContainer(t *testing.T) {
    // Option 2: Run in container if local CozoDB unavailable
    cozodbtest.RunInContainer(t, "./pkg/pipeline/...", "-run", "TestFullPipeline")
}
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: CIE Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - name: Run unit tests
        run: |
          cd modules/cie
          go test ./...

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      - name: Build test image
        run: make docker-build-cie-test
      - name: Run integration tests
        run: |
          cd modules/cie
          go test -tags=cozodb ./...
```

### Makefile Targets

```makefile
# Run all unit tests
test-cie:
	cd modules/cie && go test ./...

# Run integration tests
test-cie-integration:
	make docker-build-cie-test
	cd modules/cie && go test -tags=cozodb ./...

# Run specific package in container
test-cie-integration-pkg:
	cd modules/cie && go test -tags=cozodb $(PKG)
```

## Troubleshooting

### Docker not available

**Symptom:** Tests skip with message: `Docker not available, skipping integration tests`

**Solution:**
```bash
# Check Docker status
docker info

# For Colima users (macOS)
colima start

# Verify Docker socket
ls -la ~/.colima/default/docker.sock
```

### Container fails to start

**Symptom:** Error starting testcontainer

**Solution:**
```bash
# Build test image manually
make docker-build-cie-test

# Check image exists
docker images | grep cie-test

# Check Docker daemon
docker ps
```

### CozoDB library not found

**Symptom:** Tests fail with library loading errors

**Solution:**

For unit tests (no CozoDB needed):
```bash
# Use in-memory engine - no library required
go test ./...
```

For integration tests:
```bash
# Use Docker container - includes CozoDB
go test -tags=cozodb ./...
```

### Colima users (macOS)

The testcontainer infrastructure auto-detects Colima and configures `DOCKER_HOST` automatically.

If issues persist:
```bash
# Check Colima status
colima status

# Restart if needed
colima stop
colima start

# Verify socket
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
docker info
```

### Test data not isolated

**Symptom:** Tests interfere with each other

**Solution:** Each `SetupTestBackend(t)` creates a fresh in-memory database in a temp directory. Ensure you're creating a new backend per test:

```go
func TestA(t *testing.T) {
    backend := cietest.SetupTestBackend(t) // Fresh instance
    // ...
}

func TestB(t *testing.T) {
    backend := cietest.SetupTestBackend(t) // Another fresh instance
    // ...
}
```

## Best Practices

1. **Prefer unit tests** - Use in-memory backends for most tests
2. **Use helpers** - Don't write raw Datalog in tests, use the helper functions
3. **Isolate tests** - Each test should create its own backend
4. **Clean up automatically** - Use `t.Cleanup()` (built into `SetupTestBackend`)
5. **Use build tags** - Reserve `//go:build cozodb` for true integration tests
6. **Test data factories** - Create helper functions for common test scenarios:

```go
// TestDataFactory creates a typical test scenario
func TestDataFactory(t *testing.T, backend *storage.EmbeddedBackend) {
    cietest.InsertTestFile(t, backend, "file1", "main.go", "hash1", "go", 100)
    cietest.InsertTestFunction(t, backend, "func1", "main", "main.go", 1, 10)
    cietest.InsertTestDefines(t, backend, "def1", "file1", "func1")
}
```

## Examples

### Complete Test Example

```go
package ingestion_test

import (
    "testing"

    cietest "github.com/kraklabs/cie/internal/testing"
    "github.com/kraklabs/cie/pkg/ingestion"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestIngestionPipeline(t *testing.T) {
    backend := cietest.SetupTestBackend(t)

    // Setup test data
    cietest.InsertTestFile(t, backend, "file1", "service.go", "hash123", "go", 500)

    // Run ingestion
    pipeline := ingestion.NewPipeline(backend)
    err := pipeline.ProcessFile("service.go")
    require.NoError(t, err)

    // Verify results
    result := cietest.QueryFunctions(t, backend)
    assert.Greater(t, len(result.Rows), 0, "Should extract at least one function")

    // Verify specific function
    found := false
    for _, row := range result.Rows {
        if row[1] == "NewService" {
            found = true
            break
        }
    }
    assert.True(t, found, "Should find NewService function")
}
```

## Additional Resources

- [CozoDB Documentation](https://docs.cozodb.org/)
- [Testcontainers-Go Documentation](https://golang.testcontainers.org/)
- [Root testcontainer infrastructure](/internal/testing/cozodb/)
- [CIE testing helpers source](/modules/cie/internal/testing/)
- [Example integration test](/modules/cie/pkg/tools/client_test_cozodb.go)
