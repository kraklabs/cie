package ingestion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTestFile is a helper that reads a test fixture and parses it.
func parseTestFile(t *testing.T, fixturePath string) *ParseResult {
	t.Helper()

	code, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "Failed to read test fixture: %s", fixturePath)

	// Write to temp file for parser
	tmpFile := filepath.Join(t.TempDir(), filepath.Base(fixturePath))
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err, "Failed to write temp file")

	parser := NewTreeSitterParser(nil)
	result, err := parser.ParseFile(FileInfo{
		Path:     filepath.Base(fixturePath),
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: "go",
	})
	require.NoError(t, err, "Parser should not error on valid Go code")

	return result
}

// TestGoParser_Functions tests basic function extraction from Go files.
func TestGoParser_Functions(t *testing.T) {
	result := parseTestFile(t, "testdata/go/simple_function.go")

	// Verify function count
	assert.Len(t, result.Functions, 2, "Should extract 2 functions")

	// Verify function names
	assert.Equal(t, "Add", result.Functions[0].Name)
	assert.Equal(t, "Subtract", result.Functions[1].Name)

	// Verify signatures
	assert.Contains(t, result.Functions[0].Signature, "func Add(a, b int) int")
	assert.Contains(t, result.Functions[1].Signature, "func Subtract(a, b int) int")

	// Verify line numbers (1-indexed)
	assert.Equal(t, 4, result.Functions[0].StartLine)
	assert.Greater(t, result.Functions[0].EndLine, result.Functions[0].StartLine)

	// Verify code text is captured
	assert.NotEmpty(t, result.Functions[0].CodeText)
	assert.Contains(t, result.Functions[0].CodeText, "return a + b")
}

// TestGoParser_Methods tests method extraction with receivers.
func TestGoParser_Methods(t *testing.T) {
	result := parseTestFile(t, "testdata/go/method_receiver.go")

	// Should extract 2 methods
	assert.Len(t, result.Functions, 2, "Should extract 2 methods")

	// Find pointer receiver method
	var handleRequest *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "Handler.HandleRequest" {
			handleRequest = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, handleRequest, "Should find Handler.HandleRequest method")

	// Verify method properties
	assert.Equal(t, "Handler.HandleRequest", handleRequest.Name)
	assert.Contains(t, handleRequest.Signature, "func (h *Handler) HandleRequest")

	// Find value receiver method
	var getName *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "Handler.GetName" {
			getName = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, getName, "Should find Handler.GetName method")
	assert.Contains(t, getName.Signature, "func (h Handler) GetName")
}

// TestGoParser_Generics tests generic function and type extraction (Go 1.18+).
func TestGoParser_Generics(t *testing.T) {
	result := parseTestFile(t, "testdata/go/generics.go")

	// Should extract Map function and Container.Get method
	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract at least 2 functions")

	// Find generic Map function
	var mapFunc *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "Map" {
			mapFunc = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, mapFunc, "Should find Map function")
	assert.Contains(t, mapFunc.Signature, "[T, U any]", "Should capture generic type parameters")

	// Should extract Container type
	assert.GreaterOrEqual(t, len(result.Types), 1, "Should extract Container type")

	var containerType *TypeEntity
	for i := range result.Types {
		if result.Types[i].Name == "Container" {
			containerType = &result.Types[i]
			break
		}
	}
	require.NotNil(t, containerType, "Should find Container type")
	assert.Equal(t, "struct", containerType.Kind)
}

// TestGoParser_Interfaces tests interface extraction.
func TestGoParser_Interfaces(t *testing.T) {
	result := parseTestFile(t, "testdata/go/interface_impl.go")

	// Should extract 3 interfaces
	assert.Len(t, result.Types, 3, "Should extract 3 interfaces")

	// Verify interface types
	for _, typ := range result.Types {
		assert.Equal(t, "interface", typ.Kind, "All extracted types should be interfaces")
	}

	// Find specific interfaces
	typeNames := make(map[string]bool)
	for _, typ := range result.Types {
		typeNames[typ.Name] = true
	}
	assert.True(t, typeNames["Reader"], "Should find Reader interface")
	assert.True(t, typeNames["Writer"], "Should find Writer interface")
	assert.True(t, typeNames["ReadWriter"], "Should find ReadWriter interface")
}

// TestGoParser_MultipleReturns tests functions with multiple return values.
func TestGoParser_MultipleReturns(t *testing.T) {
	result := parseTestFile(t, "testdata/go/multiple_returns.go")

	// Should extract 2 functions
	assert.Len(t, result.Functions, 2)

	// Find Divide function with named returns
	var divide *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "Divide" {
			divide = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, divide, "Should find Divide function")
	assert.Contains(t, divide.Signature, "quotient, remainder int, err error",
		"Should capture named return parameters")
}

// TestGoParser_EmbeddedStructs tests embedded struct extraction.
func TestGoParser_EmbeddedStructs(t *testing.T) {
	result := parseTestFile(t, "testdata/go/embedded_struct.go")

	// Should extract Base and Extended types
	assert.Len(t, result.Types, 2, "Should extract 2 struct types")

	// Should extract methods
	assert.Len(t, result.Functions, 2, "Should extract 2 methods")

	// Verify methods are associated with their types
	methodNames := make(map[string]bool)
	for _, fn := range result.Functions {
		methodNames[fn.Name] = true
	}
	assert.True(t, methodNames["Base.GetID"], "Should find Base.GetID method")
	assert.True(t, methodNames["Extended.GetName"], "Should find Extended.GetName method")
}

// TestGoParser_InitFunction tests init() function extraction.
func TestGoParser_InitFunction(t *testing.T) {
	result := parseTestFile(t, "testdata/go/init_function.go")

	// Should extract init and main functions
	assert.Len(t, result.Functions, 2, "Should extract init and main")

	// Find init function
	var initFunc *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "init" {
			initFunc = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, initFunc, "Should find init function")

	// Find main function
	var mainFunc *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "main" {
			mainFunc = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, mainFunc, "Should find main function")
}

// TestGoParser_AnonymousFunctions tests closure and anonymous function extraction.
func TestGoParser_AnonymousFunctions(t *testing.T) {
	result := parseTestFile(t, "testdata/go/anonymous_function.go")

	// Should extract ProcessData, Filter, and possibly anonymous functions
	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract at least 2 functions")

	// Find named functions
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["ProcessData"], "Should find ProcessData function")
	assert.True(t, funcNames["Filter"], "Should find Filter function")

	// Anonymous functions may have generated names like $anon_1 or $lit_1
	// This is implementation-dependent, so we just verify we got the named functions
}

// TestGoParser_Imports tests import statement extraction.
func TestGoParser_Imports(t *testing.T) {
	result := parseTestFile(t, "testdata/go/imports.go")

	// Should extract imports
	assert.GreaterOrEqual(t, len(result.Imports), 3, "Should extract at least 3 imports")

	// Verify specific imports exist
	importPaths := make(map[string]bool)
	for _, imp := range result.Imports {
		importPaths[imp.ImportPath] = true
	}
	assert.True(t, importPaths["context"], "Should find context import")
	assert.True(t, importPaths["fmt"], "Should find fmt import")

	// Check for named import (strings as str)
	var hasNamedImport bool
	for _, imp := range result.Imports {
		if imp.ImportPath == "strings" && imp.Alias == "str" {
			hasNamedImport = true
			break
		}
	}
	assert.True(t, hasNamedImport, "Should capture named import alias")
}

// TestGoParser_Calls tests function call extraction.
func TestGoParser_Calls(t *testing.T) {
	result := parseTestFile(t, "testdata/go/calls.go")

	// Should extract 3 functions
	assert.Len(t, result.Functions, 3, "Should extract helper, Process, Chain")

	// Should extract call edges
	// Process calls helper, Chain calls Process and helper
	assert.GreaterOrEqual(t, len(result.Calls), 1, "Should extract at least 1 call edge")

	// Verify specific call exists: Process -> helper
	var processCallsHelper bool
	for _, call := range result.Calls {
		// Find Process function ID
		var processID string
		for _, fn := range result.Functions {
			if fn.Name == "Process" {
				processID = fn.ID
				break
			}
		}

		// Find helper function ID
		var helperID string
		for _, fn := range result.Functions {
			if fn.Name == "helper" {
				helperID = fn.ID
				break
			}
		}

		if call.CallerID == processID && call.CalleeID == helperID {
			processCallsHelper = true
			break
		}
	}
	assert.True(t, processCallsHelper, "Process should call helper")
}

// TestGoParser_EdgeCases tests edge cases like empty files and syntax errors.
func TestGoParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantErr   bool
		wantFns   int
		wantTypes int
	}{
		{
			name:      "empty file",
			file:      "testdata/go/empty.go",
			wantErr:   false,
			wantFns:   0,
			wantTypes: 0,
		},
		{
			name:      "syntax error",
			file:      "testdata/go/syntax_error.go",
			wantErr:   false, // Parser should tolerate errors
			wantFns:   0,     // May not extract any functions due to errors
			wantTypes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := os.ReadFile(tt.file)
			require.NoError(t, err, "Failed to read test fixture")

			// Write to temp file
			tmpFile := filepath.Join(t.TempDir(), filepath.Base(tt.file))
			err = os.WriteFile(tmpFile, code, 0644)
			require.NoError(t, err)

			parser := NewTreeSitterParser(nil)
			result, err := parser.ParseFile(FileInfo{
				Path:     filepath.Base(tt.file),
				FullPath: tmpFile,
				Size:     int64(len(code)),
				Language: "go",
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

// TestGoParser_IDStability tests that IDs are stable across multiple parses.
func TestGoParser_IDStability(t *testing.T) {
	code, err := os.ReadFile("testdata/go/simple_function.go")
	require.NoError(t, err)

	// Write to temp file
	tmpFile := filepath.Join(t.TempDir(), "simple_function.go")
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err)

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "simple_function.go",
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: "go",
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
			"Function %s should have stable ID across parses", result1.Functions[i].Name)
	}

	// Verify type IDs are stable (if any types)
	if len(result1.Types) > 0 {
		require.Len(t, result2.Types, len(result1.Types))
		for i := range result1.Types {
			assert.Equal(t, result1.Types[i].ID, result2.Types[i].ID,
				"Type %s should have stable ID across parses", result1.Types[i].Name)
		}
	}
}

// TestGoParser_PackageName tests package name extraction.
func TestGoParser_PackageName(t *testing.T) {
	result := parseTestFile(t, "testdata/go/simple_function.go")

	// Verify package name is extracted
	assert.Equal(t, "sample", result.PackageName, "Should extract package name")
}

// TestGoParser_FileEntity tests file entity creation.
func TestGoParser_FileEntity(t *testing.T) {
	result := parseTestFile(t, "testdata/go/simple_function.go")

	// Verify file entity
	assert.NotEmpty(t, result.File.ID, "File ID should not be empty")
	assert.Equal(t, "simple_function.go", result.File.Path)
	assert.NotEmpty(t, result.File.Hash, "File hash should not be empty")
}

// TestGoParser_DefinesEdges tests that file->function relationships are created.
func TestGoParser_DefinesEdges(t *testing.T) {
	result := parseTestFile(t, "testdata/go/simple_function.go")

	// Should have 2 defines edges (file -> Add, file -> Subtract)
	assert.Len(t, result.Defines, 2, "Should have 2 file->function edges")

	// Verify edges point to correct functions
	for _, edge := range result.Defines {
		assert.Equal(t, result.File.ID, edge.FileID, "Edge should reference file")

		// Verify function ID exists
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

// TestGoParser_DefinesTypeEdges tests that file->type relationships are created.
func TestGoParser_DefinesTypeEdges(t *testing.T) {
	result := parseTestFile(t, "testdata/go/interface_impl.go")

	// Should have 3 defines_type edges
	assert.Len(t, result.DefinesTypes, 3, "Should have 3 file->type edges")

	// Verify edges point to correct types
	for _, edge := range result.DefinesTypes {
		assert.Equal(t, result.File.ID, edge.FileID, "Edge should reference file")

		// Verify type ID exists
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
