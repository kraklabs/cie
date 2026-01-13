# Test Fixtures

This directory contains test fixtures for `pkg/ingestion` parser tests.

## Directory Structure

```
testdata/
├── README.md                  # This file
├── go/                        # Go code fixtures (12 files)
│   ├── simple_function.go     # Basic function declaration
│   ├── method_receiver.go     # Methods with pointer/value receivers
│   ├── generics.go            # Generic types and functions
│   ├── interface_impl.go      # Interface definitions and implementations
│   ├── embedded_struct.go     # Struct embedding patterns
│   ├── anonymous_function.go  # Anonymous functions and closures
│   ├── calls.go               # Function call patterns
│   ├── imports.go             # Import declarations
│   ├── init_function.go       # Init functions
│   ├── multiple_returns.go    # Multiple return values
│   ├── empty.go               # Empty file (edge case)
│   └── syntax_error.go        # Syntax error (error handling test)
├── python/                    # Python code fixtures (10 files)
│   ├── simple_function.py     # Basic function with type hints
│   ├── class_methods.py       # Class methods and instance methods
│   ├── async_functions.py     # Async/await patterns
│   ├── decorators.py          # Decorator usage
│   ├── nested_class.py        # Nested class definitions
│   ├── inheritance.py         # Class inheritance
│   ├── type_hints.py          # Advanced type hints
│   ├── lambda_expr.py         # Lambda expressions
│   ├── empty.py               # Empty file (edge case)
│   └── syntax_error.py        # Syntax error (error handling test)
├── typescript/                # TypeScript code fixtures (11 files)
│   ├── simple_function.ts     # Basic function declaration
│   ├── class_methods.ts       # Class with methods
│   ├── async_functions.ts     # Async/await patterns
│   ├── arrow_functions.ts     # Arrow function syntax
│   ├── generics.ts            # Generic types and functions
│   ├── interface.ts           # Interface definitions
│   ├── module_exports.ts      # Module export patterns
│   ├── type_alias.ts          # Type aliases
│   ├── enum.ts                # Enum definitions
│   ├── empty.ts               # Empty file (edge case)
│   └── syntax_error.ts        # Syntax error (error handling test)
├── javascript/                # JavaScript code fixtures (4 files)
│   ├── arrow.js               # Arrow functions
│   ├── class.js               # ES6 class syntax
│   ├── commonjs.js            # CommonJS module pattern
│   └── esmodule.js            # ES module pattern
├── sample_proto.proto         # Protocol Buffers fixture
└── sample_project/            # Multi-file Go project
    ├── go.mod                 # Module definition
    ├── main.go                # Entry point
    └── handlers/
        └── handler.go         # HTTP handler
```

## Usage

### Loading Parser Fixtures

The parser tests use a helper function to load fixtures:

```go
func parseTestFile(t *testing.T, lang string, filename string) *ParseResult {
    t.Helper()

    path := filepath.Join("testdata", lang, filename)
    code, err := os.ReadFile(path)
    require.NoError(t, err, "failed to read fixture: %s", path)

    result, err := Parse(lang, string(code))
    require.NoError(t, err, "failed to parse fixture: %s", path)

    return result
}
```

### Example Test

```go
func TestParseGoFunction(t *testing.T) {
    result := parseTestFile(t, "go", "simple_function.go")

    assert.Equal(t, 1, len(result.Functions))
    assert.Equal(t, "Add", result.Functions[0].Name)
    assert.Equal(t, 2, len(result.Functions[0].Parameters))
}
```

### Testing Edge Cases

```go
func TestParseEmptyFile(t *testing.T) {
    result := parseTestFile(t, "go", "empty.go")
    assert.Equal(t, 0, len(result.Functions))
}

func TestParseSyntaxError(t *testing.T) {
    _, err := parseTestFile(t, "go", "syntax_error.go")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "syntax error")
}
```

## Fixture Organization

### Language-Specific Subdirectories

Each language has its own subdirectory with fixtures covering:

- **Basic patterns**: Simple, straightforward code
- **Language features**: Specific syntax and idioms
- **Edge cases**: Empty files, syntax errors
- **Common patterns**: Real-world code structures

### Naming Conventions

Fixtures follow a consistent naming pattern:
- `{feature}.{ext}` - Descriptive name indicating what pattern is tested
- `empty.{ext}` - Empty file for edge case testing
- `syntax_error.{ext}` - Invalid syntax for error handling tests

## Adding New Fixtures

When adding new parser test fixtures:

1. **Choose the appropriate language directory** - `go/`, `python/`, `typescript/`, or `javascript/`
2. **Use descriptive names** - Name should indicate what pattern is being tested
3. **Keep fixtures minimal** - Focus on one feature per fixture (5-20 lines)
4. **Include comments** - Explain what the fixture demonstrates
5. **Validate syntax** - Ensure fixtures are valid code (except syntax_error files)
6. **Test both success and failure** - Include edge cases and error conditions

## Fixture Content Guidelines

### Valid Code Fixtures
```go
// simple_function.go
package sample

// Add returns the sum of two numbers.
func Add(a, b int) int {
    return a + b
}
```

### Edge Case Fixtures
```go
// empty.go
package sample
```

```go
// syntax_error.go
package sample

func Broken( {
    return
}
```

## Parser Test Patterns

### Table-Driven Tests

```go
func TestParseGoFunctions(t *testing.T) {
    tests := []struct {
        name     string
        fixture  string
        wantFuncs int
    }{
        {"simple function", "simple_function.go", 1},
        {"method receiver", "method_receiver.go", 2},
        {"empty file", "empty.go", 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := parseTestFile(t, "go", tt.fixture)
            assert.Equal(t, tt.wantFuncs, len(result.Functions))
        })
    }
}
```

## Notes

- All fixtures are version-controlled (committed to git)
- Fixtures use realistic code patterns, not contrived examples
- Each language has ~10-12 fixtures covering common patterns
- Edge cases (empty files, syntax errors) are included for robustness
- Update this README when adding new fixture types or languages
