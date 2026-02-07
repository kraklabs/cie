package ingestion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildImplementsIndex_Basic(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Writer",
			Kind:     "interface",
			CodeText: "Writer interface {\n\tWrite(data []byte) error\n\tFlush() error\n}",
		},
	}
	functions := []FunctionEntity{
		{Name: "CozoDB.Write", FilePath: "store/cozodb.go"},
		{Name: "CozoDB.Flush", FilePath: "store/cozodb.go"},
		{Name: "FileStore.Write", FilePath: "store/filestore.go"},
		{Name: "FileStore.Flush", FilePath: "store/filestore.go"},
		{Name: "Unrelated.DoSomething", FilePath: "other/unrelated.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	// CozoDB and FileStore both implement Writer (both have Write + Flush)
	assert.Len(t, edges, 2, "Should find 2 implementations of Writer")

	implTypes := make(map[string]bool)
	for _, e := range edges {
		implTypes[e.TypeName] = true
		assert.Equal(t, "Writer", e.InterfaceName)
	}
	assert.True(t, implTypes["CozoDB"], "CozoDB should implement Writer")
	assert.True(t, implTypes["FileStore"], "FileStore should implement Writer")

	// Unrelated should NOT be in the results
	assert.False(t, implTypes["Unrelated"], "Unrelated should NOT implement Writer")
}

func TestBuildImplementsIndex_PartialDoesNotMatch(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Writer",
			Kind:     "interface",
			CodeText: "Writer interface {\n\tWrite(data []byte) error\n\tFlush() error\n}",
		},
	}
	functions := []FunctionEntity{
		// Partial has Write but NOT Flush
		{Name: "Partial.Write", FilePath: "store/partial.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	assert.Len(t, edges, 0, "Partial implementation should not produce an edge")
}

func TestBuildImplementsIndex_NoSelfMatch(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Writer",
			Kind:     "interface",
			CodeText: "Writer interface {\n\tWrite(data []byte) error\n}",
		},
	}
	functions := []FunctionEntity{
		// Even if there were a method "Writer.Write", the interface
		// should not match itself. However, interfaces don't have
		// receiver methods, so this tests the guard.
		{Name: "Writer.Write", FilePath: "iface.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	// Writer should not "implement" itself
	for _, e := range edges {
		assert.NotEqual(t, e.TypeName, e.InterfaceName,
			"Interface should not implement itself")
	}
}

func TestBuildImplementsIndex_EmptyInterface(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Empty",
			Kind:     "interface",
			CodeText: "Empty interface {}",
		},
	}
	functions := []FunctionEntity{
		{Name: "Foo.Bar", FilePath: "foo.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	// Empty interface with no methods should not match anything
	// (the regex won't find method names, so methods slice is empty)
	assert.Len(t, edges, 0, "Empty interface should not produce edges")
}

func TestBuildImplementsIndex_EmbeddedInterface(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Reader",
			Kind:     "interface",
			CodeText: "Reader interface {\n\tRead(p []byte) (int, error)\n}",
		},
		{
			Name:     "Writer",
			Kind:     "interface",
			CodeText: "Writer interface {\n\tWrite(p []byte) (int, error)\n}",
		},
		{
			Name: "ReadWriter",
			Kind: "interface",
			CodeText: "ReadWriter interface {\n\tReader\n\tWriter\n\tFlush() error\n}",
		},
	}
	functions := []FunctionEntity{
		// BufferedRW implements all three methods: Read, Write, Flush
		{Name: "BufferedRW.Read", FilePath: "io/buf.go"},
		{Name: "BufferedRW.Write", FilePath: "io/buf.go"},
		{Name: "BufferedRW.Flush", FilePath: "io/buf.go"},
		// SimpleWriter only has Write
		{Name: "SimpleWriter.Write", FilePath: "io/simple.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	implMap := make(map[string][]string) // interface → []type
	for _, e := range edges {
		implMap[e.InterfaceName] = append(implMap[e.InterfaceName], e.TypeName)
	}

	// BufferedRW should implement Reader, Writer, AND ReadWriter
	assert.Contains(t, implMap["Reader"], "BufferedRW", "BufferedRW should implement Reader")
	assert.Contains(t, implMap["Writer"], "BufferedRW", "BufferedRW should implement Writer")
	assert.Contains(t, implMap["ReadWriter"], "BufferedRW", "BufferedRW should implement ReadWriter (via embedded methods)")

	// SimpleWriter should implement Writer but NOT ReadWriter (missing Read and Flush)
	assert.Contains(t, implMap["Writer"], "SimpleWriter", "SimpleWriter should implement Writer")
	assert.NotContains(t, implMap["ReadWriter"], "SimpleWriter", "SimpleWriter should NOT implement ReadWriter")
}

func TestBuildImplementsIndex_EmbeddedInterface_StdlibFallback(t *testing.T) {
	// Test that embedding io.Stringer falls back to the stdlib map
	types := []TypeEntity{
		{
			Name: "Named",
			Kind: "interface",
			CodeText: "Named interface {\n\tfmt.Stringer\n\tName() string\n}",
		},
	}
	functions := []FunctionEntity{
		// MyType has both String() and Name()
		{Name: "MyType.String", FilePath: "pkg/types.go"},
		{Name: "MyType.Name", FilePath: "pkg/types.go"},
		// PartialType only has Name()
		{Name: "PartialType.Name", FilePath: "pkg/types.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	implMap := make(map[string]bool)
	for _, e := range edges {
		implMap[e.TypeName] = true
	}

	assert.True(t, implMap["MyType"], "MyType should implement Named (has String+Name)")
	assert.False(t, implMap["PartialType"], "PartialType should NOT implement Named (missing String)")
}

func TestBuildImplementsIndex_MultipleInterfaces(t *testing.T) {
	types := []TypeEntity{
		{
			Name:     "Writer",
			Kind:     "interface",
			CodeText: "Writer interface {\n\tWrite(data []byte) error\n}",
		},
		{
			Name:     "Flusher",
			Kind:     "interface",
			CodeText: "Flusher interface {\n\tFlush() error\n}",
		},
	}
	functions := []FunctionEntity{
		{Name: "CozoDB.Write", FilePath: "store.go"},
		{Name: "CozoDB.Flush", FilePath: "store.go"},
	}

	edges := BuildImplementsIndex(types, functions)

	// CozoDB implements both Writer and Flusher
	assert.Len(t, edges, 2, "CozoDB should implement both Writer and Flusher")

	ifaceMap := make(map[string]bool)
	for _, e := range edges {
		ifaceMap[e.InterfaceName] = true
		assert.Equal(t, "CozoDB", e.TypeName)
	}
	assert.True(t, ifaceMap["Writer"])
	assert.True(t, ifaceMap["Flusher"])
}
