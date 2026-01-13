# Test Fixtures

This directory contains test fixtures for `pkg/tools` tests.

## Directory Structure

```
testdata/
├── README.md                  # This file
├── code_samples/              # Sample code files for parsing
│   ├── sample.go              # Go code with functions, types, methods
│   ├── sample.py              # Python code with classes, decorators
│   └── sample.ts              # TypeScript with interfaces, async
├── responses/                 # Mock API response fixtures
│   ├── semantic_search.json   # Sample semantic search result
│   ├── grep_result.json       # Sample grep result
│   └── function_info.json     # Sample function metadata
├── queries/                   # Sample query fixtures
│   ├── valid_query.txt        # Example valid search query
│   └── invalid_query.txt      # Example malformed query for error testing
├── sample_project/            # Multi-file Go project
│   ├── go.mod                 # Module definition
│   ├── main.go                # Entry point
│   ├── handlers/              # HTTP handlers
│   └── internal/db/           # Database layer
├── functions.json             # Legacy: Sample function data
└── search_results.json        # Legacy: Sample search results
```

## Usage

### Loading Code Samples

```go
func TestParseGoFile(t *testing.T) {
    code, err := os.ReadFile("testdata/code_samples/sample.go")
    if err != nil {
        t.Fatalf("failed to read fixture: %v", err)
    }

    result, err := parser.Parse("sample.go", code)
    require.NoError(t, err)
    // ... assertions
}
```

### Loading JSON Responses

```go
func TestProcessSearchResult(t *testing.T) {
    data, err := os.ReadFile("testdata/responses/semantic_search.json")
    require.NoError(t, err)

    var result SearchResult
    err = json.Unmarshal(data, &result)
    require.NoError(t, err)
    // ... assertions
}
```

### Loading Query Fixtures

```go
func TestValidateQuery(t *testing.T) {
    query, err := os.ReadFile("testdata/queries/valid_query.txt")
    require.NoError(t, err)

    err = ValidateQuery(string(query))
    assert.NoError(t, err)
}
```

### Using Sample Project

```go
func TestAnalyzeProject(t *testing.T) {
    projectDir := "testdata/sample_project"

    result, err := AnalyzeDirectory(projectDir)
    require.NoError(t, err)

    assert.Greater(t, len(result.Files), 0)
    assert.Greater(t, len(result.Functions), 0)
}
```

## Adding New Fixtures

When adding new test fixtures:

1. **Keep fixtures minimal but realistic** - 5-20 lines of code
2. **Use representative patterns** - Real-world code styles, not contrived examples
3. **Include edge cases** - Empty files, syntax errors, etc.
4. **Document purpose** - Add comments explaining what the fixture tests
5. **Validate fixtures** - Ensure code compiles/parses correctly

## Fixture Guidelines

### Code Samples
- Valid syntax for the target language
- Include comments and documentation
- Use realistic naming conventions
- Cover common patterns (functions, classes, types, interfaces)

### Mock Responses
- Valid JSON format
- Representative data structure
- Include typical fields and values
- Document the response schema

### Queries
- Both valid and invalid examples
- Cover edge cases (empty, malformed, special characters)
- Include comments explaining the test case

## Notes

- All files in `testdata/` are ignored by `go build`
- Fixtures are version-controlled (committed to git)
- Update this README when adding new fixture categories
