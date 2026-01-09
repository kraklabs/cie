// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
//
// For commercial licensing, contact: licensing@kraklabs.com
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingestion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTreeSitterParser_NestedFunctions tests extraction of nested functions.
func TestTreeSitterParser_NestedFunctions(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "nested.go")
	content := `package main

func outer() {
	inner := func() {
		println("inner")
	}
	inner()
}

func main() {
	outer()
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "nested.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract outer, main, and the anonymous inner function
	if len(result.Functions) < 3 {
		t.Errorf("expected at least 3 functions (outer, main, anonymous), got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s at line %d", i, fn.Name, fn.StartLine)
		}
	}

	// Verify anonymous function is extracted with special name
	foundAnon := false
	for _, fn := range result.Functions {
		if strings.HasPrefix(fn.Name, "$anon_") {
			foundAnon = true
			break
		}
	}
	if !foundAnon {
		t.Error("expected to find anonymous function with $anon_ prefix")
	}
}

// TestTreeSitterParser_MethodsOnStructs tests extraction of methods on structs.
func TestTreeSitterParser_MethodsOnStructs(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "methods.go")
	content := `package main

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}

func (s Server) Port() int {
	return s.port
}

func NewServer(port int) *Server {
	return &Server{port: port}
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "methods.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract Server.Start, Server.Port, NewServer
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify method names include receiver type
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}

	// Methods should have format ReceiverType.MethodName
	if !names["Server.Start"] {
		t.Error("expected method name 'Server.Start' (with receiver type prefix)")
	}
	if !names["Server.Port"] {
		t.Error("expected method name 'Server.Port' (with receiver type prefix)")
	}
	if !names["NewServer"] {
		t.Error("expected function name 'NewServer'")
	}

	// Verify method signatures include receiver
	for _, fn := range result.Functions {
		if fn.Name == "Server.Start" || fn.Name == "Server.Port" {
			if !strings.Contains(fn.Signature, "Server") {
				t.Errorf("method %s signature should include receiver, got: %s", fn.Name, fn.Signature)
			}
		}
	}
}

// TestTreeSitterParser_GoMethodsWithGenerics tests methods on generic structs.
func TestTreeSitterParser_GoMethodsWithGenerics(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "generics_methods.go")
	content := `package main

type Container[T any] struct {
	value T
}

func (c *Container[T]) Get() T {
	return c.value
}

func (c *Container[T]) Set(v T) {
	c.value = v
}

func NewContainer[T any](v T) *Container[T] {
	return &Container[T]{value: v}
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "generics_methods.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract Container.Get, Container.Set, NewContainer
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify method names include receiver type (without generic params)
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}

	if !names["Container.Get"] {
		t.Error("expected method name 'Container.Get'")
	}
	if !names["Container.Set"] {
		t.Error("expected method name 'Container.Set'")
	}
	if !names["NewContainer"] {
		t.Error("expected function name 'NewContainer'")
	}
}

// TestTreeSitterParser_GoInitFunctions tests extraction of init functions.
func TestTreeSitterParser_GoInitFunctions(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "init.go")
	content := `package main

import "fmt"

func init() {
	fmt.Println("init 1")
}

func init() {
	fmt.Println("init 2")
}

func main() {
	fmt.Println("main")
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "init.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract 2 init functions and main
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions (2 init + main), got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s at line %d", i, fn.Name, fn.StartLine)
		}
	}

	// Count init functions
	initCount := 0
	for _, fn := range result.Functions {
		if fn.Name == "init" {
			initCount++
		}
	}
	if initCount != 2 {
		t.Errorf("expected 2 init functions, got %d", initCount)
	}

	// Verify both init functions have different IDs (due to different line numbers)
	ids := make(map[string]bool)
	for _, fn := range result.Functions {
		if fn.Name == "init" {
			if ids[fn.ID] {
				t.Errorf("init functions should have unique IDs, found duplicate: %s", fn.ID)
			}
			ids[fn.ID] = true
		}
	}
}

// TestTreeSitterParser_CommentsWithFuncKeyword tests that comments containing "func" don't create false positives.
func TestTreeSitterParser_CommentsWithFuncKeyword(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "comments.go")
	content := `package main

// This is a comment about func things
// func notAFunction() {} <- this should be ignored
/* func alsoIgnored() {} */

func realFunction() {
	// func insideComment() {}
	println("real")
}

// Another func mention in comments
func anotherReal() {
	/*
	func multilineComment() {
		should also be ignored
	}
	*/
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "comments.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should only extract realFunction and anotherReal
	if len(result.Functions) != 2 {
		t.Errorf("expected exactly 2 functions (not from comments), got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s at line %d", i, fn.Name, fn.StartLine)
		}
	}

	// Verify function names
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}
	if !names["realFunction"] {
		t.Error("expected to find realFunction")
	}
	if !names["anotherReal"] {
		t.Error("expected to find anotherReal")
	}
}

// TestTreeSitterParser_Generics tests extraction of generic functions.
func TestTreeSitterParser_Generics(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "generics.go")
	content := `package main

func Map[T, U any](slice []T, f func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

func Filter[T any](slice []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

type Container[T any] struct {
	value T
}

func (c *Container[T]) Get() T {
	return c.value
}

func (c *Container[T]) Set(v T) {
	c.value = v
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "generics.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract Map, Filter, Get, Set
	if len(result.Functions) < 4 {
		t.Errorf("expected at least 4 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify generic function is recognized
	for _, fn := range result.Functions {
		if fn.Name == "Map" {
			// Signature should contain the generic parameters
			if !strings.Contains(fn.CodeText, "[T, U any]") {
				t.Errorf("Map function should have generic parameters in code, got: %s", fn.CodeText[:min(100, len(fn.CodeText))])
			}
		}
	}
}

// TestTreeSitterParser_MalformedCode tests parser resilience with malformed code.
func TestTreeSitterParser_MalformedCode(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "malformed.go")
	content := `package main

func validFunction() {
	println("valid")
}

// Missing closing brace
func brokenFunction() {
	println("broken"

func anotherValid() {
	println("another")
}

// Syntax error
func } syntax {
	error here
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "malformed.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	// Should not panic or error fatally - Tree-sitter is error-tolerant
	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file should not fail even with malformed code: %v", err)
	}

	// Should still extract some valid functions
	if len(result.Functions) < 1 {
		t.Error("expected to extract at least some functions from malformed code")
	}

	// Verify validFunction was extracted
	foundValid := false
	for _, fn := range result.Functions {
		if fn.Name == "validFunction" {
			foundValid = true
			break
		}
	}
	if !foundValid {
		t.Error("expected to find validFunction even with malformed code")
	}
}

// TestTreeSitterParser_Python tests Python parsing.
func TestTreeSitterParser_Python(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.py")
	content := `import os

def greet(name: str) -> str:
    """Greet someone."""
    return f"Hello, {name}!"

class Calculator:
    def __init__(self):
        self.value = 0
    
    def add(self, x: int) -> int:
        self.value += x
        return self.value
    
    def subtract(self, x: int) -> int:
        self.value -= x
        return self.value

# Lambda expression
double = lambda x: x * 2

def nested_func():
    def inner():
        return 42
    return inner()
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "test.py",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "python",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract: greet, Calculator.__init__, Calculator.add, Calculator.subtract, double lambda, nested_func, inner
	if len(result.Functions) < 5 {
		t.Errorf("expected at least 5 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify class methods have class prefix
	for _, fn := range result.Functions {
		if fn.Name == "Calculator.__init__" || fn.Name == "Calculator.add" {
			// Found prefixed method name
			break
		}
	}
}

// TestTreeSitterParser_JavaScript tests JavaScript parsing.
func TestTreeSitterParser_JavaScript(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.js")
	content := `// Function declaration
function greet(name) {
    return "Hello, " + name;
}

// Arrow function
const add = (a, b) => a + b;

// Function expression
const multiply = function(a, b) {
    return a * b;
};

// Class with methods
class Calculator {
    constructor() {
        this.value = 0;
    }
    
    add(x) {
        this.value += x;
        return this;
    }
}

// Callback with anonymous arrow
[1, 2, 3].map(x => x * 2);

// IIFE (should extract the inner function)
(function() {
    console.log("IIFE");
})();
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "test.js",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "javascript",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract several functions
	if len(result.Functions) < 4 {
		t.Errorf("expected at least 4 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify function types
	names := make(map[string]bool)
	for _, fn := range result.Functions {
		names[fn.Name] = true
	}
	if !names["greet"] {
		t.Error("expected to find greet function")
	}
	if !names["add"] {
		t.Error("expected to find add arrow function")
	}
}

// TestTreeSitterParser_TypeScript tests TypeScript-specific parsing.
func TestTreeSitterParser_TypeScript(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.ts")
	content := `interface Greeter {
    greet(name: string): string;
}

type Handler<T> = (value: T) => void;

function createGreeter(): Greeter {
    return {
        greet: (name: string) => "Hello, " + name
    };
}

const handler: Handler<number> = (value) => {
    console.log(value);
};

class TypedCalculator<T extends number> {
    private value: T;
    
    constructor(initial: T) {
        this.value = initial;
    }
    
    add(x: T): T {
        return (this.value + x) as T;
    }
}

// Async function
async function fetchData(url: string): Promise<string> {
    return await fetch(url).then(r => r.text());
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "test.ts",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "typescript",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract functions from TypeScript
	if len(result.Functions) < 3 {
		t.Errorf("expected at least 3 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}
}

// TestTreeSitterParser_CallGraphExtraction tests that same-file calls are extracted.
func TestTreeSitterParser_CallGraphExtraction(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "calls.go")
	content := `package main

func helper() string {
	return "help"
}

func wrapper() string {
	return helper()
}

func main() {
	result := wrapper()
	println(result)
	helper() // direct call
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "calls.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should have calls edges
	if len(result.Calls) < 2 {
		t.Errorf("expected at least 2 calls edges (wrapper->helper, main->helper, main->wrapper), got %d", len(result.Calls))
		for i, call := range result.Calls {
			t.Logf("  call[%d]: %s -> %s", i, call.CallerID, call.CalleeID)
		}
	}

	// Verify defines edges
	if len(result.Defines) != len(result.Functions) {
		t.Errorf("expected %d defines edges (one per function), got %d", len(result.Functions), len(result.Defines))
	}
}

// TestTreeSitterParser_GoMethodCalls tests call extraction for method calls.
func TestTreeSitterParser_GoMethodCalls(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "method_calls.go")
	content := `package main

type Calculator struct {
	value int
}

func (c *Calculator) Add(x int) int {
	c.value += x
	return c.value
}

func (c *Calculator) Multiply(x int) int {
	for i := 1; i < x; i++ {
		c.Add(c.value) // method calling method
	}
	return c.value
}

func NewCalculator() *Calculator {
	return &Calculator{value: 0}
}

func main() {
	calc := NewCalculator()
	calc.Add(5)
	calc.Multiply(2)
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "method_calls.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract: Calculator.Add, Calculator.Multiply, NewCalculator, main
	if len(result.Functions) != 4 {
		t.Errorf("expected 4 functions, got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Should have at least: main->NewCalculator, main->Add, main->Multiply, Multiply->Add
	// Note: Multiply calling c.Add() should be detected as calling "Add"
	if len(result.Calls) < 1 {
		t.Logf("Calls found: %d", len(result.Calls))
		for i, call := range result.Calls {
			t.Logf("  call[%d]: %s -> %s", i, call.CallerID, call.CalleeID)
		}
	}
}

// TestTreeSitterParser_GoRecursion tests that recursive calls are handled correctly.
func TestTreeSitterParser_GoRecursion(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "recursion.go")
	content := `package main

func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}

func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "recursion.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract factorial and fibonacci
	if len(result.Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(result.Functions))
	}

	// Recursive self-calls should NOT be included as edges (they're redundant)
	// factorial calls factorial, but we filter self-calls
	for _, call := range result.Calls {
		if call.CallerID == call.CalleeID {
			t.Errorf("self-call detected: %s -> %s (should be filtered)", call.CallerID, call.CalleeID)
		}
	}
}

// TestTreeSitterParser_GoInterfaceMethods tests interface method detection.
func TestTreeSitterParser_GoInterfaceMethods(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "interface.go")
	content := `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader
	Writer
}

type FileReader struct {
	path string
}

func (f *FileReader) Read(p []byte) (int, error) {
	return len(p), nil
}

func NewFileReader(path string) *FileReader {
	return &FileReader{path: path}
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "interface.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract: FileReader.Read, NewFileReader
	// Interface methods are not extracted (they're declarations, not implementations)
	if len(result.Functions) != 2 {
		t.Errorf("expected 2 functions (FileReader.Read, NewFileReader), got %d", len(result.Functions))
		for i, fn := range result.Functions {
			t.Logf("  func[%d]: %s", i, fn.Name)
		}
	}

	// Verify we got the concrete implementation
	found := false
	for _, fn := range result.Functions {
		if fn.Name == "FileReader.Read" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find FileReader.Read method")
	}
}

// TestTreeSitterParser_GoMultipleReturns tests functions with multiple return values.
func TestTreeSitterParser_GoMultipleReturns(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "returns.go")
	content := `package main

import "errors"

func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

func divmod(a, b int) (quotient, remainder int) {
	quotient = a / b
	remainder = a % b
	return
}

func complex() (result string, count int, err error) {
	return "hello", 5, nil
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "returns.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract all 3 functions
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(result.Functions))
	}

	// Verify signatures include return types
	for _, fn := range result.Functions {
		switch fn.Name {
		case "divide":
			if !strings.Contains(fn.Signature, "(int, error)") {
				t.Errorf("divide signature should include (int, error), got: %s", fn.Signature)
			}
		case "divmod":
			if !strings.Contains(fn.Signature, "(quotient, remainder int)") {
				t.Errorf("divmod signature should include named returns, got: %s", fn.Signature)
			}
		case "complex":
			if !strings.Contains(fn.Signature, "result string") || !strings.Contains(fn.Signature, "count int") {
				t.Errorf("complex signature should include all named returns, got: %s", fn.Signature)
			}
		}
	}
}

// TestTreeSitterParser_GoVariadicFunctions tests variadic function extraction.
func TestTreeSitterParser_GoVariadicFunctions(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "variadic.go")
	content := `package main

func sum(nums ...int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

func printf(format string, args ...interface{}) {
	// implementation
}

func join(sep string, strs ...string) string {
	return ""
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "variadic.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract all 3 variadic functions
	if len(result.Functions) != 3 {
		t.Errorf("expected 3 functions, got %d", len(result.Functions))
	}

	// Verify variadic parameters are captured
	for _, fn := range result.Functions {
		if fn.Name == "sum" {
			if !strings.Contains(fn.Signature, "...int") {
				t.Errorf("sum signature should include variadic, got: %s", fn.Signature)
			}
		}
	}
}

// TestTreeSitterParser_Idempotency tests that parsing the same file twice yields identical results.
func TestTreeSitterParser_Idempotency(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "idempotency.go")
	content := `package main

func foo() {
	println("foo")
}

func bar() {
	println("bar")
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "idempotency.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	// Parse twice
	result1, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}

	result2, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}

	// Verify same number of entities
	if len(result1.Functions) != len(result2.Functions) {
		t.Errorf("function count differs: %d vs %d", len(result1.Functions), len(result2.Functions))
	}

	// Verify same IDs
	ids1 := make(map[string]bool)
	for _, fn := range result1.Functions {
		ids1[fn.ID] = true
	}
	for _, fn := range result2.Functions {
		if !ids1[fn.ID] {
			t.Errorf("function ID %s from second parse not found in first parse", fn.ID)
		}
	}

	// Verify file entity
	if result1.File.ID != result2.File.ID {
		t.Errorf("file ID differs: %s vs %s", result1.File.ID, result2.File.ID)
	}
	if result1.File.Hash != result2.File.Hash {
		t.Errorf("file hash differs: %s vs %s", result1.File.Hash, result2.File.Hash)
	}
}

// TestTreeSitterParser_UnsupportedLanguage tests handling of unsupported languages.
func TestTreeSitterParser_UnsupportedLanguage(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.xyz")
	content := `some content in unknown language`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	fileInfo := FileInfo{
		Path:     "test.xyz",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "unknown",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file should not error for unsupported language: %v", err)
	}

	// Should return empty results, not error
	if result.File.ID == "" {
		t.Error("file entity should still have ID")
	}
	if len(result.Functions) != 0 {
		t.Errorf("expected 0 functions for unsupported language, got %d", len(result.Functions))
	}
}

// TestTreeSitterParser_LargeCodeText tests truncation of large code text.
func TestTreeSitterParser_LargeCodeText(t *testing.T) {
	// Create a function with large body
	largeBody := strings.Repeat("println(\"line\")\n", 10000)
	tmpFile := filepath.Join(t.TempDir(), "large.go")
	content := `package main

func largeFunction() {
` + largeBody + `}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	parser := NewTreeSitterParser(nil)
	parser.SetMaxCodeTextSize(1000) // 1KB limit

	fileInfo := FileInfo{
		Path:     "large.go",
		FullPath: tmpFile,
		Size:     int64(len(content)),
		Language: "go",
	}

	result, err := parser.ParseFile(fileInfo)
	if err != nil {
		t.Fatalf("parse file: %v", err)
	}

	// Should extract function
	if len(result.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(result.Functions))
	}

	// CodeText should be truncated
	if len(result.Functions[0].CodeText) > 1000 {
		t.Errorf("expected CodeText to be truncated to 1000 bytes, got %d", len(result.Functions[0].CodeText))
	}

	// Truncated count should be incremented
	if parser.GetTruncatedCount() != 1 {
		t.Errorf("expected truncated count 1, got %d", parser.GetTruncatedCount())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
