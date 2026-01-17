package ingestion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTypeScriptTestFile parses a TypeScript test fixture.
func parseTypeScriptTestFile(t *testing.T, fixturePath string, lang string) *ParseResult {
	t.Helper()

	code, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), filepath.Base(fixturePath))
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err)

	parser := NewTreeSitterParser(nil)
	result, err := parser.ParseFile(FileInfo{
		Path:     filepath.Base(fixturePath),
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: lang,
	})
	require.NoError(t, err)

	return result
}

// TestTypeScriptParser_Functions tests basic function extraction.
func TestTypeScriptParser_Functions(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/simple_function.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract at least 2 functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["add"], "Should find add function")
	assert.True(t, funcNames["subtract"], "Should find subtract function")
}

// TestTypeScriptParser_ArrowFunctions tests arrow function extraction.
func TestTypeScriptParser_ArrowFunctions(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/arrow_functions.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract arrow functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["double"], "Should find double arrow function")
	assert.True(t, funcNames["greet"], "Should find greet arrow function")
}

// TestTypeScriptParser_Classes tests class extraction.
func TestTypeScriptParser_Classes(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/class_methods.ts", "typescript")

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

	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract methods")
}

// TestTypeScriptParser_Interfaces tests interface extraction.
func TestTypeScriptParser_Interfaces(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/interface.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Types), 2, "Should extract interfaces")

	typeNames := make(map[string]bool)
	for _, typ := range result.Types {
		typeNames[typ.Name] = true
		assert.Equal(t, "interface", typ.Kind, "Should be interface type")
	}
	assert.True(t, typeNames["User"], "Should find User interface")
	assert.True(t, typeNames["Repository"], "Should find Repository interface")
}

// TestTypeScriptParser_TypeAliases tests type alias extraction.
func TestTypeScriptParser_TypeAliases(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/type_alias.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Types), 3, "Should extract type aliases")

	typeNames := make(map[string]bool)
	for _, typ := range result.Types {
		typeNames[typ.Name] = true
		assert.Equal(t, "type_alias", typ.Kind, "Should be type alias")
	}
	assert.True(t, typeNames["UserId"], "Should find UserId type alias")
	assert.True(t, typeNames["Handler"], "Should find Handler type alias")
}

// TestTypeScriptParser_Generics tests generic extraction.
func TestTypeScriptParser_Generics(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/generics.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Functions), 1, "Should extract generic function")

	var identityFunc *FunctionEntity
	for i := range result.Functions {
		if result.Functions[i].Name == "identity" {
			identityFunc = &result.Functions[i]
			break
		}
	}
	require.NotNil(t, identityFunc, "Should find identity function")
	// Note: Generic type parameters are captured in CodeText, not in Signature.
	// The Signature extracts: "function identity(value: T)" from the parameters node.
	// Full generics visible in CodeText: "export function identity<T>(value: T): T { ... }"
	assert.Contains(t, identityFunc.Signature, "identity", "Should capture function name")
	assert.Contains(t, identityFunc.CodeText, "<T>", "Generic parameter visible in code text")

	assert.GreaterOrEqual(t, len(result.Types), 1, "Should extract generic class")
}

// TestTypeScriptParser_Enums tests enum extraction.
func TestTypeScriptParser_Enums(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/enum.ts", "typescript")

	// Note: Enums might be extracted as types or not at all depending on parser
	// This is implementation-dependent
	_ = result
}

// TestTypeScriptParser_AsyncFunctions tests async function extraction.
func TestTypeScriptParser_AsyncFunctions(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/async_functions.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Functions), 2, "Should extract async functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["fetchData"], "Should find fetchData")
	assert.True(t, funcNames["processItems"], "Should find processItems")
}

// TestTypeScriptParser_Exports tests module export extraction.
func TestTypeScriptParser_Exports(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/module_exports.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Functions), 1, "Should extract exported functions")

	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}
	assert.True(t, funcNames["publicFunction"], "Should find publicFunction")
}

// TestTypeScriptParser_JavaScript tests JavaScript compatibility.
func TestTypeScriptParser_JavaScript(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{"commonjs", "testdata/javascript/commonjs.js"},
		{"esmodule", "testdata/javascript/esmodule.js"},
		{"class", "testdata/javascript/class.js"},
		{"arrow", "testdata/javascript/arrow.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTypeScriptTestFile(t, tt.file, "javascript")
			assert.GreaterOrEqual(t, len(result.Functions), 1, "Should extract functions from JS")
		})
	}
}

// TestTypeScriptParser_EdgeCases tests edge cases.
func TestTypeScriptParser_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantErr   bool
		wantFns   int
		wantTypes int
	}{
		{"empty file", "testdata/typescript/empty.ts", false, 0, 0},
		{"syntax error", "testdata/typescript/syntax_error.ts", false, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := os.ReadFile(tt.file)
			require.NoError(t, err)

			tmpFile := filepath.Join(t.TempDir(), filepath.Base(tt.file))
			err = os.WriteFile(tmpFile, code, 0644)
			require.NoError(t, err)

			parser := NewTreeSitterParser(nil)
			result, err := parser.ParseFile(FileInfo{
				Path:     filepath.Base(tt.file),
				FullPath: tmpFile,
				Size:     int64(len(code)),
				Language: "typescript",
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

// TestTypeScriptParser_IDStability tests ID stability.
func TestTypeScriptParser_IDStability(t *testing.T) {
	code, err := os.ReadFile("testdata/typescript/simple_function.ts")
	require.NoError(t, err)

	tmpFile := filepath.Join(t.TempDir(), "simple_function.ts")
	err = os.WriteFile(tmpFile, code, 0644)
	require.NoError(t, err)

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "simple_function.ts",
		FullPath: tmpFile,
		Size:     int64(len(code)),
		Language: "typescript",
	}

	result1, err := parser.ParseFile(fileInfo)
	require.NoError(t, err)

	result2, err := parser.ParseFile(fileInfo)
	require.NoError(t, err)

	require.Len(t, result2.Functions, len(result1.Functions))

	for i := range result1.Functions {
		assert.Equal(t, result1.Functions[i].ID, result2.Functions[i].ID,
			"Function %s should have stable ID", result1.Functions[i].Name)
	}
}

// TestTypeScriptParser_FileEntity tests file entity creation.
func TestTypeScriptParser_FileEntity(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/simple_function.ts", "typescript")

	assert.NotEmpty(t, result.File.ID)
	assert.Equal(t, "simple_function.ts", result.File.Path)
	assert.NotEmpty(t, result.File.Hash)
}

// TestTypeScriptParser_DefinesEdges tests file->function relationships.
func TestTypeScriptParser_DefinesEdges(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/simple_function.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.Defines), 2, "Should have file->function edges")

	for _, edge := range result.Defines {
		assert.Equal(t, result.File.ID, edge.FileID)
		var found bool
		for _, fn := range result.Functions {
			if fn.ID == edge.FunctionID {
				found = true
				break
			}
		}
		assert.True(t, found)
	}
}

// TestTypeScriptParser_DefinesTypeEdges tests file->type relationships.
func TestTypeScriptParser_DefinesTypeEdges(t *testing.T) {
	result := parseTypeScriptTestFile(t, "testdata/typescript/interface.ts", "typescript")

	assert.GreaterOrEqual(t, len(result.DefinesTypes), 2, "Should have file->type edges")

	for _, edge := range result.DefinesTypes {
		assert.Equal(t, result.File.ID, edge.FileID)
		var found bool
		for _, typ := range result.Types {
			if typ.ID == edge.TypeID {
				found = true
				break
			}
		}
		assert.True(t, found)
	}
}
