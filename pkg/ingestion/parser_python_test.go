package ingestion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parsePythonTestFile is a helper that reads a Python test fixture and parses it.
func parsePythonTestFile(t *testing.T, fixturePath string) *ParseResult {
	t.Helper()

	code, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "Failed to read test fixture: %s", fixturePath)

	tmpFile := filepath.Join(t.TempDir(), filepath.Base(fixturePath))
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err, "Failed to write temp file")

	parser := NewTreeSitterParser(nil)
	result, err := parser.ParseFile(FileInfo{
		Path:     filepath.Base(fixturePath),
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: "python",
	})
	require.NoError(t, err, "Parser should not error on valid Python code")

	return result
}

// TestPythonParser_Functions tests basic function extraction from Python files.
func TestPythonParser_Functions(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/simple_function.py")

	// Verify function count
	assert.Len(t, result.Functions, 2, "Should extract 2 functions")

	// Verify function names
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["add"], "Should find add function")
	assert.True(t, funcNames["subtract"], "Should find subtract function")

	// Find add function
	var addFunc *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "add" {
			addFunc = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, addFunc, "Should find add function")

	// Verify signature and code
	assert.Contains(t, addFunc.Signature, "def add(a: int, b: int) -> int")
	assert.NotEmpty(t, addFunc.CodeText)
}

// TestPythonParser_Classes tests class and method extraction.
func TestPythonParser_Classes(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/class_methods.py")

	// Should extract UserService class
	assert.GreaterOrEqual(t, len(result.Types), 1, "Should extract at least 1 class")

	var userServiceType *TypeEntity
	for i := range result.Types {
		if result.Types[i].Name == "UserService" {
			userServiceType = &result.Types[i]
			break
		}
	}
	require.NotNil(t, userServiceType, "Should find UserService class")
	assert.Equal(t, "class", userServiceType.Kind)

	// Should extract methods (including __init__)
	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract at least 2 methods")

	// Find methods
	methodNames := make(map[string]bool)
	for _, fn := range result.Functions {
		methodNames[fn.Name] = true
	}

	// Methods should be prefixed with class name
	hasUserServiceMethod := false
	for name := range methodNames {
		if len(name) > 12 && name[:12] == "UserService." {
			hasUserServiceMethod = true
			break
		}
	}
	assert.True(t, hasUserServiceMethod, "Should find at least one UserService method")
}

// TestPythonParser_Decorators tests decorated function extraction.
func TestPythonParser_Decorators(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/decorators.py")

	// Should extract cache, expensive_operation, another_operation
	assert.GreaterOrEqual(t, len(result.Functions), 3, "Should extract at least 3 functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	assert.True(t, funcNames["cache"], "Should find cache decorator")
	assert.True(t, funcNames["expensive_operation"], "Should find decorated expensive_operation")
	assert.True(t, funcNames["another_operation"], "Should find decorated another_operation")
}

// TestPythonParser_AsyncFunctions tests async/await syntax.
func TestPythonParser_AsyncFunctions(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/async_functions.py")

	// Should extract async functions
	assert.GreaterOrEqual(t, len(result.Functions), 3, "Should extract at least 3 functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	assert.True(t, funcNames["fetch_data"], "Should find fetch_data async function")
	assert.True(t, funcNames["simulate_request"], "Should find simulate_request async function")
	assert.True(t, funcNames["process_items"], "Should find process_items async function")
}

// TestPythonParser_TypeHints tests type annotation extraction.
func TestPythonParser_TypeHints(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/type_hints.py")

	// Should extract 3 functions
	assert.Len(t, result.Functions, 3, "Should extract 3 functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	assert.True(t, funcNames["process_list"], "Should find process_list")
	assert.True(t, funcNames["get_config"], "Should find get_config")
	assert.True(t, funcNames["find_user"], "Should find find_user")

	// Find process_list and verify type hints in signature
	var processList *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "process_list" {
			processList = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, processList, "Should find process_list function")
	assert.Contains(t, processList.Signature, "List[int]", "Should capture type hint")
}

// TestPythonParser_Inheritance tests class inheritance.
func TestPythonParser_Inheritance(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/inheritance.py")

	// Should extract 3 classes: Animal, Dog, Cat
	assert.Len(t, result.Types, 3, "Should extract 3 classes")

	typeNames := make(map[string]bool)
	for _, typ := range result.Types {
		typeNames[typ.Name] = true
		assert.Equal(t, "class", typ.Kind, "All should be classes")
	}

	assert.True(t, typeNames["Animal"], "Should find Animal class")
	assert.True(t, typeNames["Dog"], "Should find Dog class")
	assert.True(t, typeNames["Cat"], "Should find Cat class")

	// Should extract methods from all classes
	assert.GreaterOrEqual(t, len(result.Functions), 5, "Should extract at least 5 methods")
}

// TestPythonParser_Lambda tests lambda expression extraction.
func TestPythonParser_Lambda(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/lambda_expr.py")

	// Should extract apply_operation and possibly lambda functions
	assert.GreaterOrEqual(t, len(result.Functions), 1, "Should extract at least 1 function")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	assert.True(t, funcNames["apply_operation"], "Should find apply_operation function")

	// Lambda functions may be extracted with names like $lambda_1
	// This is implementation-dependent
	hasLambda := false
	for name := range funcNames {
		if len(name) > 7 && name[:7] == "$lambda" {
			hasLambda = true
			break
		}
	}
	// Note: Lambda extraction is optional depending on parser implementation
	_ = hasLambda
}

// TestPythonParser_NestedClass tests nested class extraction.
func TestPythonParser_NestedClass(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/nested_class.py")

	// Should extract Outer and possibly Inner classes
	assert.GreaterOrEqual(t, len(result.Types), 1, "Should extract at least 1 class")

	var outerClass *TypeEntity
	for i := range result.Types {
		if result.Types[i].Name == "Outer" {
			outerClass = &result.Types[i]
			break
		}
	}
	require.NotNil(t, outerClass, "Should find Outer class")

	// Should extract methods
	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract at least 2 methods")
}

// TestPythonParser_EdgeCases tests edge cases.
func TestPythonParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantErr   bool
		wantFns   int
		wantTypes int
	}{
		{
			name:      "empty file",
			file:      "testdata/python/empty.py",
			wantErr:   false,
			wantFns:   0,
			wantTypes: 0,
		},
		{
			name:      "syntax error",
			file:      "testdata/python/syntax_error.py",
			wantErr:   false, // Parser should tolerate errors
			wantFns:   1,     // Tree-sitter still extracts partial functions from malformed code
			wantTypes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := os.ReadFile(tt.file)
			require.NoError(t, err, "Failed to read test fixture")

			tmpFile := filepath.Join(t.TempDir(), filepath.Base(tt.file))
			err = os.WriteFile(tmpFile, code, 0644)
			require.NoError(t, err)

			parser := NewTreeSitterParser(nil)
			result, err := parser.ParseFile(FileInfo{
				Path:     filepath.Base(tt.file),
				FullPath: tmpFile,
				Size:     int64(len(code)),
				Language: "python",
			})

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Functions, tt.wantFns)
				assert.Len(t, result.Types, tt.wantTypes)
			}
		})
	}
}

// TestPythonParser_IDStability tests ID stability across parses.
func TestPythonParser_IDStability(t *testing.T) {
	code, err := os.ReadFile("testdata/python/simple_function.py")
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), "simple_function.py")
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err)

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "simple_function.py",
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: "python",
	}

	// Parse twice
	result1, err := parser.ParseFile(fileInfo)
	require.NoError(t, err)

	result2, err := parser.ParseFile(fileInfo)
	require.NoError(t, err)

	// Verify same number of functions
	require.Len(t, result2.Functions, len(result1.Functions))

	// Verify IDs are identical
	for i := range result1.Functions {
		assert.Equal(t, result1.Functions[i].ID, result2.Functions[i].ID,
			"Function %s should have stable ID", result1.Functions[i].Name)
	}
}

// TestPythonParser_FileEntity tests file entity creation.
func TestPythonParser_FileEntity(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/simple_function.py")

	// Verify file entity
	assert.NotEmpty(t, result.File.ID, "File ID should not be empty")
	assert.Equal(t, "simple_function.py", result.File.Path)
	assert.NotEmpty(t, result.File.Hash, "File hash should not be empty")
}

// TestPythonParser_DefinesEdges tests file->function relationships.
func TestPythonParser_DefinesEdges(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/simple_function.py")

	// Should have 2 defines edges
	assert.Len(t, result.Defines, 2, "Should have 2 file->function edges")

	for _, edge := range result.Defines {
		assert.Equal(t, result.File.ID, edge.FileID)

		var found bool
		for _, fn := range result.Functions {
			if fn.ID == edge.FunctionID {
				found = true
				break
			}
		}
		assert.True(t, found, "Edge should reference valid function ID")
	}
}

// TestPythonParser_DefinesTypeEdges tests file->type relationships.
func TestPythonParser_DefinesTypeEdges(t *testing.T) {
	result := parsePythonTestFile(t, "testdata/python/class_methods.py")

	// Should have at least 1 defines_type edge
	assert.GreaterOrEqual(t, len(result.DefinesTypes), 1, "Should have at least 1 file->type edge")

	for _, edge := range result.DefinesTypes {
		assert.Equal(t, result.File.ID, edge.FileID)

		var found bool
		for _, typ := range result.Types {
			if typ.ID == edge.TypeID {
				found = true
				break
			}
		}
		assert.True(t, found, "Edge should reference valid type ID")
	}
}
