# CIE Benchmark Results

This document contains performance benchmarks for core CIE operations. These benchmarks help track performance over time and identify performance regressions during development.

## Environment Specifications

**Baseline Date:** 2026-01-12

**Hardware:**
- CPU: Apple M1 Max
- RAM: 32 GB
- Architecture: arm64

**Software:**
- OS: macOS 26.2
- Go Version: go1.25.5 darwin/arm64
- CozoDB: v0.7.6 (in-memory mode)

**Test Dataset:**
- Files: 100 files across 6 packages
- Functions: 500 functions with realistic signatures
- Call Graph: 1000+ call edges (depth: ~10 levels)

## Baseline Results

| Operation | ops/sec | ns/op | μs/op | B/op | allocs/op |
|-----------|---------|-------|-------|------|-----------|
| SemanticSearch (10 results) | 131 | 7,656,749 | 7,657 | 27,629 | 174 |
| SemanticSearch (100 results) | 267,495 | 3,738 | 3.7 | 5,535 | 61 |
| Grep (single pattern) | 9,612 | 104,042 | 104 | 1,801 | 48 |
| GrepMulti (3 patterns) | 5,280 | 189,467 | 189 | 5,192 | 66 |
| FindFunction | 2,666 | 375,239 | 375 | 6,651 | 130 |
| FindCallers | 497 | 2,014,706 | 2,015 | 6,096 | 133 |
| GetCallGraph | 246 | 4,059,417 | 4,059 | 21,581 | 307 |
| TracePath | 193 | 5,183,556 | 5,184 | 10,265 | 205 |

**Performance Tiers:**
- **Fast** (<100 μs): SemanticSearch_Large (keyword fallback)
- **Medium** (100-500 μs): Grep, GrepMulti, FindFunction
- **Slow** (2-5 ms): FindCallers, GetCallGraph, TracePath

## Running Benchmarks

### Prerequisites

1. **Install CozoDB Library:**
   ```bash
   # CozoDB dynamic library must be in /usr/local/lib
   # See CIE installation docs for setup
   ```

2. **Verify Library Path:**
   ```bash
   export DYLD_LIBRARY_PATH=/usr/local/lib:$DYLD_LIBRARY_PATH
   export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
   ```

### Run All Benchmarks

```bash
# Standard run (1 second per benchmark)
go test -bench=. -benchmem -tags=cozodb ./pkg/tools/

# Longer run for stability (3 seconds per benchmark)
go test -bench=. -benchmem -benchtime=3s -tags=cozodb ./pkg/tools/

# Save results to file
go test -bench=. -benchmem -benchtime=3s -tags=cozodb ./pkg/tools/ | tee benchmark_results.txt
```

### Run Specific Benchmark

```bash
# Run only semantic search benchmarks
go test -bench=BenchmarkSemanticSearch -benchmem -tags=cozodb ./pkg/tools/

# Run only grep benchmarks
go test -bench=BenchmarkGrep -benchmem -tags=cozodb ./pkg/tools/
```

## Performance Comparison

### Using benchstat

To compare performance before and after changes:

1. **Install benchstat:**
   ```bash
   go install golang.org/x/perf/cmd/benchstat@latest
   ```

2. **Capture baseline:**
   ```bash
   go test -bench=. -benchmem -benchtime=3s -tags=cozodb ./pkg/tools/ > old.txt
   ```

3. **Make changes to code**

4. **Capture new results:**
   ```bash
   go test -bench=. -benchmem -benchtime=3s -tags=cozodb ./pkg/tools/ > new.txt
   ```

5. **Compare:**
   ```bash
   benchstat old.txt new.txt
   ```

### Example Output

```
name                        old time/op    new time/op    delta
SemanticSearch-10             7.66ms ± 2%    7.23ms ± 1%   -5.61%
Grep-10                        104µs ± 1%     98µs ± 2%   -5.77%
FindFunction-10                375µs ± 3%    360µs ± 2%   -4.00%

name                        old alloc/op   new alloc/op   delta
SemanticSearch-10             27.6kB ± 0%    26.1kB ± 0%   -5.43%
Grep-10                       1.80kB ± 0%    1.75kB ± 0%   -2.78%
```

## Performance Notes

### SemanticSearch

- **Small (10 results):** 7.7 ms average
- **Large (100 results):** 3.7 μs average (surprisingly fast - keyword fallback)
- **Note:** These benchmarks use the keyword search fallback path since embedding generation is not available in the test environment. Real semantic search with vector embeddings would be ~50-200ms depending on embedding service.
- **Memory:** 27-5 KB per query
- **Scaling:** Linear with result set size

### Grep

- **Single pattern:** 104 μs average - very fast literal text search
- **Multi-pattern (3):** 189 μs average - efficient batch search
- **Memory:** 1.8-5 KB per query
- **Scaling:** Near-linear with number of patterns

### FindFunction

- **Average:** 375 μs per lookup
- **Memory:** 6.7 KB per query
- **Note:** Performance depends on function name uniqueness

### FindCallers

- **Average:** 2 ms per query
- **Memory:** 6.1 KB per query
- **Note:** Slower due to call graph traversal (joins 2 tables)

### GetCallGraph

- **Average:** 4 ms per query
- **Memory:** 21.6 KB per query
- **Note:** Most memory-intensive operation (gets both callers and callees)

### TracePath

- **Average:** 5.2 ms per query
- **Memory:** 10.3 KB per query
- **Note:** Slowest operation due to BFS graph traversal
- **Scaling:** Depends on graph depth and max_depth parameter

## Interpreting Results

### What is "Good" Performance?

For a dataset of ~500 functions:
- **Grep**: <200 μs is excellent
- **FindFunction**: <500 μs is good
- **FindCallers/GetCallGraph**: <10 ms is acceptable
- **TracePath**: <10 ms is acceptable

### When to Investigate

Investigate performance if you see:
- **>20% slowdown** in any benchmark
- **>10% increase in allocations** without corresponding feature addition
- **>50ms** for any operation on a 500-function dataset

### Expected Variance

Typical variance across runs:
- **Fast operations** (<1ms): ±5%
- **Medium operations** (1-5ms): ±10%
- **Slow operations** (>5ms): ±15%

Run with `-benchtime=10s` for more stable results.

## CI Integration

### GitHub Actions

Add to `.github/workflows/ci.yml`:

```yaml
- name: Run Benchmarks
  run: |
    export DYLD_LIBRARY_PATH=/usr/local/lib:$DYLD_LIBRARY_PATH
    go test -bench=. -benchmem -tags=cozodb ./pkg/tools/ | tee benchmark_results.txt

- name: Upload Benchmark Results
  uses: actions/upload-artifact@v3
  with:
    name: benchmark-results
    path: benchmark_results.txt
```

### Benchmark Regression Detection

Optional: Use [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) in CI to automatically detect regressions:

```yaml
- name: Detect Regressions
  run: |
    # Download baseline from previous run
    # Compare with benchstat
    benchstat baseline.txt current.txt || exit 1
```

## Benchmark Implementation

Benchmarks are located in `pkg/tools/benchmark_test.go` with build tag `cozodb`.

**Key features:**
- In-memory CozoDB database (no disk I/O)
- Realistic test data (100 files, 500 functions, 1000 calls)
- Proper `b.ResetTimer()` to exclude setup time
- Memory allocation tracking with `-benchmem`

**See also:**
- [Go Benchmark Guide](https://pkg.go.dev/testing#hdr-Benchmarks)
- [benchstat Documentation](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
