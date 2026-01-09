// Copyright 2025 KrakLabs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package ingestion

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// =============================================================================
// GO PARSER - Primary focus (90% of codebase)
// =============================================================================

// goFunctionContext holds context during Go AST walking.
type goFunctionContext struct {
	functions    []goFunctionWithNode
	funcNameToID map[string]string // Simple name -> ID for call resolution
	content      []byte
	filePath     string
	anonCounter  int // Counter for anonymous functions
}

// goFunctionWithNode pairs a function entity with its AST node for call extraction.
type goFunctionWithNode struct {
	entity FunctionEntity
	node   *sitter.Node
}

// goParseResult contains all extracted data from Go parsing.
type goParseResult struct {
	Functions       []FunctionEntity
	Types           []TypeEntity
	Calls           []CallsEdge
	Imports         []ImportEntity
	UnresolvedCalls []UnresolvedCall
	PackageName     string
}

// parseGoAST extracts functions from Go source using Tree-sitter.
// This is the primary parser for the codebase.
func (p *TreeSitterParser) parseGoAST(content []byte, filePath string) (*goParseResult, error) {
	tree, err := p.goParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode.HasError() {
		if errorCount := countErrors(rootNode); errorCount > 0 {
			p.logger.Warn("parser.treesitter.go.syntax_errors",
				"path", filePath,
				"error_count", errorCount,
			)
		}
		// Continue parsing - Tree-sitter is error-tolerant
	}

	// Extract package name
	packageName := p.extractGoPackageName(rootNode, content)

	// Extract imports (before function extraction)
	imports := p.extractGoImports(rootNode, content, filePath)

	// Initialize context
	ctx := &goFunctionContext{
		functions:    make([]goFunctionWithNode, 0),
		funcNameToID: make(map[string]string),
		content:      content,
		filePath:     filePath,
		anonCounter:  0,
	}

	// First pass: extract all functions with their AST nodes
	p.walkGoAST(rootNode, ctx)

	// Second pass: extract calls within each function using V2 (returns unresolved calls)
	var calls []CallsEdge
	var unresolvedCalls []UnresolvedCall
	for _, fnWithNode := range ctx.functions {
		localCalls, unresolved := p.extractGoCallsFromNodeV2(
			fnWithNode.node, content, fnWithNode.entity.ID,
			ctx.funcNameToID, filePath)
		calls = append(calls, localCalls...)
		unresolvedCalls = append(unresolvedCalls, unresolved...)
	}

	// Extract just the entities
	functions := make([]FunctionEntity, len(ctx.functions))
	for i, fn := range ctx.functions {
		functions[i] = fn.entity
	}

	// Extract types (structs, interfaces, type aliases)
	types := p.extractGoTypes(rootNode, content, filePath)

	return &goParseResult{
		Functions:       functions,
		Types:           types,
		Calls:           calls,
		Imports:         imports,
		UnresolvedCalls: unresolvedCalls,
		PackageName:     packageName,
	}, nil
}

// walkGoAST recursively walks the Go AST to find all function declarations.
func (p *TreeSitterParser) walkGoAST(node *sitter.Node, ctx *goFunctionContext) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		fn := p.extractGoFunctionDeclaration(node, ctx)
		if fn != nil {
			ctx.functions = append(ctx.functions, goFunctionWithNode{entity: *fn, node: node})
			// Store by simple name for call resolution
			ctx.funcNameToID[fn.Name] = fn.ID
		}

	case "method_declaration":
		fn := p.extractGoMethodDeclaration(node, ctx)
		if fn != nil {
			ctx.functions = append(ctx.functions, goFunctionWithNode{entity: *fn, node: node})
			// Store by simple name (without receiver) for call resolution
			// e.g., for "(s *Server) Start()", store "Start" -> ID
			simpleName := extractSimpleName(fn.Name)
			ctx.funcNameToID[simpleName] = fn.ID
		}

	case "func_literal":
		fn := p.extractGoFuncLiteral(node, ctx)
		if fn != nil {
			ctx.functions = append(ctx.functions, goFunctionWithNode{entity: *fn, node: node})
			// Anonymous functions aren't called by name, so don't add to funcNameToID
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkGoAST(child, ctx)
	}
}

// extractGoFunctionDeclaration extracts a Go function declaration.
// Handles: func foo(), func foo[T any](), func init()
func (p *TreeSitterParser) extractGoFunctionDeclaration(node *sitter.Node, ctx *goFunctionContext) *FunctionEntity {
	// Get function name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(ctx.content[nameNode.StartByte():nameNode.EndByte()])

	// Get type parameters (generics)
	typeParamsNode := node.ChildByFieldName("type_parameters")
	var typeParams string
	if typeParamsNode != nil {
		typeParams = string(ctx.content[typeParamsNode.StartByte():typeParamsNode.EndByte()])
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(ctx.content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	// Get return type (result)
	resultNode := node.ChildByFieldName("result")
	var result string
	if resultNode != nil {
		result = string(ctx.content[resultNode.StartByte():resultNode.EndByte()])
	}

	// Build signature: func Name[T](...) result
	var sigBuilder strings.Builder
	sigBuilder.WriteString("func ")
	sigBuilder.WriteString(name)
	if typeParams != "" {
		sigBuilder.WriteString(typeParams)
	}
	sigBuilder.WriteString(params)
	if result != "" {
		sigBuilder.WriteString(" ")
		sigBuilder.WriteString(result)
	}
	signature := sigBuilder.String()

	return p.createGoFunctionEntity(node, ctx, name, signature)
}

// extractGoMethodDeclaration extracts a Go method declaration.
// Handles: func (r *Receiver) Method(), func (r Receiver) Method[T any]()
func (p *TreeSitterParser) extractGoMethodDeclaration(node *sitter.Node, ctx *goFunctionContext) *FunctionEntity {
	// Get method name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	methodName := string(ctx.content[nameNode.StartByte():nameNode.EndByte()])

	// Get receiver
	receiverNode := node.ChildByFieldName("receiver")
	var receiver string
	var receiverType string
	if receiverNode != nil {
		receiver = string(ctx.content[receiverNode.StartByte():receiverNode.EndByte()])
		receiverType = extractReceiverType(receiverNode, ctx.content)
	}

	// Get type parameters (generics)
	typeParamsNode := node.ChildByFieldName("type_parameters")
	var typeParams string
	if typeParamsNode != nil {
		typeParams = string(ctx.content[typeParamsNode.StartByte():typeParamsNode.EndByte()])
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(ctx.content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	// Get return type
	resultNode := node.ChildByFieldName("result")
	var result string
	if resultNode != nil {
		result = string(ctx.content[resultNode.StartByte():resultNode.EndByte()])
	}

	// Build full name: ReceiverType.MethodName
	var fullName string
	if receiverType != "" {
		fullName = receiverType + "." + methodName
	} else {
		fullName = methodName
	}

	// Build signature: func (r *Type) Method[T](...) result
	var sigBuilder strings.Builder
	sigBuilder.WriteString("func ")
	sigBuilder.WriteString(receiver)
	sigBuilder.WriteString(" ")
	sigBuilder.WriteString(methodName)
	if typeParams != "" {
		sigBuilder.WriteString(typeParams)
	}
	sigBuilder.WriteString(params)
	if result != "" {
		sigBuilder.WriteString(" ")
		sigBuilder.WriteString(result)
	}
	signature := sigBuilder.String()

	return p.createGoFunctionEntity(node, ctx, fullName, signature)
}

// extractGoFuncLiteral extracts an anonymous function/closure.
// Handles: func() {}, func(x int) int {}
func (p *TreeSitterParser) extractGoFuncLiteral(node *sitter.Node, ctx *goFunctionContext) *FunctionEntity {
	// Anonymous functions use position-based naming
	ctx.anonCounter++
	name := fmt.Sprintf("$anon_%d", ctx.anonCounter)

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(ctx.content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	// Get return type
	resultNode := node.ChildByFieldName("result")
	var result string
	if resultNode != nil {
		result = string(ctx.content[resultNode.StartByte():resultNode.EndByte()])
	}

	// Build signature
	var sigBuilder strings.Builder
	sigBuilder.WriteString("func")
	sigBuilder.WriteString(params)
	if result != "" {
		sigBuilder.WriteString(" ")
		sigBuilder.WriteString(result)
	}
	signature := sigBuilder.String()

	return p.createGoFunctionEntity(node, ctx, name, signature)
}

// createGoFunctionEntity creates a FunctionEntity from parsed data.
func (p *TreeSitterParser) createGoFunctionEntity(node *sitter.Node, ctx *goFunctionContext, name, signature string) *FunctionEntity {
	// Get positions (1-indexed)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	// Get code text
	codeText := string(ctx.content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	// Generate deterministic ID
	id := GenerateFunctionID(ctx.filePath, name, signature, startLine, endLine, startCol, endCol)

	return &FunctionEntity{
		ID:        id,
		Name:      name,
		Signature: signature,
		FilePath:  ctx.filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// extractReceiverType extracts the type name from a receiver parameter.
// e.g., from "(s *Server)" extracts "Server", from "(s Server[T])" extracts "Server"
func extractReceiverType(receiverNode *sitter.Node, content []byte) string {
	if receiverNode == nil {
		return ""
	}

	// Walk through receiver to find the type
	// Structure: parameter_list > parameter_declaration > type
	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child.Type() == "parameter_declaration" {
			typeNode := child.ChildByFieldName("type")
			if typeNode != nil {
				return extractBaseTypeName(typeNode, content)
			}
		}
	}
	return ""
}

// extractBaseTypeName extracts the base type name, handling pointers and generics.
// e.g., *Server -> Server, Server[T] -> Server, *Server[T] -> Server
func extractBaseTypeName(typeNode *sitter.Node, content []byte) string {
	if typeNode == nil {
		return ""
	}

	nodeType := typeNode.Type()

	// Pointer type: *T
	if nodeType == "pointer_type" {
		// Get the underlying type
		for i := 0; i < int(typeNode.ChildCount()); i++ {
			child := typeNode.Child(i)
			if child.Type() != "*" {
				return extractBaseTypeName(child, content)
			}
		}
	}

	// Generic type: T[U]
	if nodeType == "generic_type" {
		typeNameNode := typeNode.ChildByFieldName("type")
		if typeNameNode != nil {
			return string(content[typeNameNode.StartByte():typeNameNode.EndByte()])
		}
	}

	// Simple type identifier
	if nodeType == "type_identifier" {
		return string(content[typeNode.StartByte():typeNode.EndByte()])
	}

	// Fallback: return the whole thing
	typeName := string(content[typeNode.StartByte():typeNode.EndByte()])
	// Strip pointer prefix if present
	typeName = strings.TrimPrefix(typeName, "*")
	// Strip generic suffix if present
	if idx := strings.Index(typeName, "["); idx > 0 {
		typeName = typeName[:idx]
	}
	return typeName
}

// extractSimpleName extracts the simple method name from a full name.
// e.g., "Server.Start" -> "Start"
func extractSimpleName(fullName string) string {
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		return fullName[idx+1:]
	}
	return fullName
}

// extractGoCallsFromNode extracts function calls from a function's AST node.
func (p *TreeSitterParser) extractGoCallsFromNode(fnNode *sitter.Node, content []byte, callerID string, funcNameToID map[string]string) []CallsEdge {
	var calls []CallsEdge

	if fnNode == nil {
		return calls
	}

	// Find the body of the function
	bodyNode := fnNode.ChildByFieldName("body")
	if bodyNode == nil {
		// For func_literal, look for block child
		for i := 0; i < int(fnNode.ChildCount()); i++ {
			child := fnNode.Child(i)
			if child.Type() == "block" {
				bodyNode = child
				break
			}
		}
	}
	if bodyNode == nil {
		return calls
	}

	// Walk to find call expressions
	p.walkGoCallExpressions(bodyNode, content, callerID, funcNameToID, &calls)

	return calls
}

// walkGoCallExpressions finds call expressions within a node.
func (p *TreeSitterParser) walkGoCallExpressions(node *sitter.Node, content []byte, callerID string, funcNameToID map[string]string, calls *[]CallsEdge) {
	if node == nil {
		return
	}

	if node.Type() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			calleeName := p.extractGoCalleeName(funcNode, content)
			if calleeName != "" {
				// Look up callee ID
				if calleeID, exists := funcNameToID[calleeName]; exists {
					// Avoid self-calls (can happen with recursion, but we only want distinct edges)
					if calleeID != callerID {
						*calls = append(*calls, CallsEdge{
							CallerID: callerID,
							CalleeID: calleeID,
						})
					}
				}
			}
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkGoCallExpressions(child, content, callerID, funcNameToID, calls)
	}
}

// extractGoCalleeName extracts the function name from a Go call expression.
func (p *TreeSitterParser) extractGoCalleeName(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	nodeType := node.Type()

	// Simple identifier: foo()
	if nodeType == "identifier" {
		return string(content[node.StartByte():node.EndByte()])
	}

	// Selector expression: pkg.Foo() or obj.Method()
	if nodeType == "selector_expression" {
		fieldNode := node.ChildByFieldName("field")
		if fieldNode != nil {
			return string(content[fieldNode.StartByte():fieldNode.EndByte()])
		}
	}

	// Generic function call: foo[T]()
	if nodeType == "type_arguments" {
		// This is the type args, we need the parent's function
		return ""
	}

	// Index expression with type args: foo[int]()
	if nodeType == "index_expression" {
		operandNode := node.ChildByFieldName("operand")
		if operandNode != nil {
			return p.extractGoCalleeName(operandNode, content)
		}
	}

	return ""
}

// extractGoCalleeNameFull extracts the full call expression (e.g., "pkg.Foo" or "foo").
// Used for cross-package call resolution.
func (p *TreeSitterParser) extractGoCalleeNameFull(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	nodeType := node.Type()

	// Simple identifier: foo()
	if nodeType == "identifier" {
		return string(content[node.StartByte():node.EndByte()])
	}

	// Selector expression: pkg.Foo() or obj.Method()
	if nodeType == "selector_expression" {
		// Return the full expression: "pkg.Foo"
		return string(content[node.StartByte():node.EndByte()])
	}

	// Index expression with type args: foo[int]()
	if nodeType == "index_expression" {
		operandNode := node.ChildByFieldName("operand")
		if operandNode != nil {
			return p.extractGoCalleeNameFull(operandNode, content)
		}
	}

	return ""
}

// extractGoPackageName extracts the package name from a Go source file.
func (p *TreeSitterParser) extractGoPackageName(rootNode *sitter.Node, content []byte) string {
	if rootNode == nil {
		return ""
	}

	// Find package_clause node at the top level
	for i := 0; i < int(rootNode.ChildCount()); i++ {
		child := rootNode.Child(i)
		if child.Type() == "package_clause" {
			// Package clause structure: package <name>
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				return string(content[nameNode.StartByte():nameNode.EndByte()])
			}
			// Fallback: look for identifier child
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild.Type() == "package_identifier" {
					return string(content[grandchild.StartByte():grandchild.EndByte()])
				}
			}
		}
	}
	return ""
}

// extractGoImports extracts all import declarations from a Go source file.
func (p *TreeSitterParser) extractGoImports(rootNode *sitter.Node, content []byte, filePath string) []ImportEntity {
	var imports []ImportEntity

	if rootNode == nil {
		return imports
	}

	// Find import_declaration nodes at the top level
	for i := 0; i < int(rootNode.ChildCount()); i++ {
		child := rootNode.Child(i)
		if child.Type() == "import_declaration" {
			imports = append(imports, p.extractGoImportDeclaration(child, content, filePath)...)
		}
	}

	return imports
}

// extractGoImportDeclaration extracts imports from a single import declaration.
// Handles both single imports and import blocks.
func (p *TreeSitterParser) extractGoImportDeclaration(node *sitter.Node, content []byte, filePath string) []ImportEntity {
	var imports []ImportEntity

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type() {
		case "import_spec":
			// Single import: import "fmt" or import alias "pkg/path"
			imp := p.extractGoImportSpec(child, content, filePath)
			if imp != nil {
				imports = append(imports, *imp)
			}
		case "import_spec_list":
			// Import block: import ( ... )
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "import_spec" {
					imp := p.extractGoImportSpec(spec, content, filePath)
					if imp != nil {
						imports = append(imports, *imp)
					}
				}
			}
		}
	}

	return imports
}

// extractGoImportSpec extracts a single import spec.
func (p *TreeSitterParser) extractGoImportSpec(node *sitter.Node, content []byte, filePath string) *ImportEntity {
	// Get the import path (required)
	pathNode := node.ChildByFieldName("path")
	if pathNode == nil {
		// Fallback: look for interpreted_string_literal child
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "interpreted_string_literal" {
				pathNode = child
				break
			}
		}
	}
	if pathNode == nil {
		return nil
	}

	// Remove quotes from path
	importPath := string(content[pathNode.StartByte():pathNode.EndByte()])
	importPath = strings.Trim(importPath, `"`)

	// Get optional alias (name field)
	alias := ""
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		alias = string(content[nameNode.StartByte():nameNode.EndByte()])
	} else {
		// Check for dot import or blank import
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "dot" || child.Type() == "." {
				alias = "."
				break
			}
			if child.Type() == "blank_identifier" {
				alias = "_"
				break
			}
			if child.Type() == "package_identifier" {
				alias = string(content[child.StartByte():child.EndByte()])
				break
			}
		}
	}

	return &ImportEntity{
		ID:         GenerateImportID(filePath, importPath),
		FilePath:   filePath,
		ImportPath: importPath,
		Alias:      alias,
		StartLine:  int(node.StartPoint().Row) + 1,
	}
}

// extractGoCallsFromNodeV2 extracts calls from a function, returning both
// resolved (same-file) calls and unresolved (cross-package) calls.
func (p *TreeSitterParser) extractGoCallsFromNodeV2(
	fnNode *sitter.Node, content []byte, callerID string,
	funcNameToID map[string]string, filePath string,
) ([]CallsEdge, []UnresolvedCall) {
	var localCalls []CallsEdge
	var unresolvedCalls []UnresolvedCall

	if fnNode == nil {
		return localCalls, unresolvedCalls
	}

	// Find the body of the function
	bodyNode := fnNode.ChildByFieldName("body")
	if bodyNode == nil {
		// For func_literal, look for block child
		for i := 0; i < int(fnNode.ChildCount()); i++ {
			child := fnNode.Child(i)
			if child.Type() == "block" {
				bodyNode = child
				break
			}
		}
	}
	if bodyNode == nil {
		return localCalls, unresolvedCalls
	}

	// Track seen edges to avoid duplicates
	seenLocal := make(map[string]bool)
	seenUnresolved := make(map[string]bool)

	// Walk to find call expressions
	p.walkGoCallExpressionsV2(bodyNode, content, callerID, funcNameToID, filePath,
		&localCalls, &unresolvedCalls, seenLocal, seenUnresolved)

	return localCalls, unresolvedCalls
}

// walkGoCallExpressionsV2 finds call expressions and categorizes them as local or unresolved.
func (p *TreeSitterParser) walkGoCallExpressionsV2(
	node *sitter.Node, content []byte, callerID string,
	funcNameToID map[string]string, filePath string,
	localCalls *[]CallsEdge, unresolvedCalls *[]UnresolvedCall,
	seenLocal, seenUnresolved map[string]bool,
) {
	if node == nil {
		return
	}

	if node.Type() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			// Get simple name for local resolution
			simpleName := p.extractGoCalleeName(funcNode, content)
			// Get full name for cross-package calls (e.g., "pkg.Foo")
			fullName := p.extractGoCalleeNameFull(funcNode, content)

			if simpleName != "" {
				// Try local resolution first
				if calleeID, exists := funcNameToID[simpleName]; exists {
					// Found in same file
					if calleeID != callerID {
						edgeKey := callerID + "->" + calleeID
						if !seenLocal[edgeKey] {
							seenLocal[edgeKey] = true
							*localCalls = append(*localCalls, CallsEdge{
								CallerID: callerID,
								CalleeID: calleeID,
							})
						}
					}
				} else if fullName != "" {
					// Unresolved - store for cross-package resolution
					callLine := int(node.StartPoint().Row) + 1
					key := callerID + "->" + fullName
					if !seenUnresolved[key] {
						seenUnresolved[key] = true
						*unresolvedCalls = append(*unresolvedCalls, UnresolvedCall{
							CallerID:   callerID,
							CalleeName: fullName,
							FilePath:   filePath,
							Line:       callLine,
						})
					}
				}
			}
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkGoCallExpressionsV2(child, content, callerID, funcNameToID, filePath,
			localCalls, unresolvedCalls, seenLocal, seenUnresolved)
	}
}

// parseGoFile extracts functions from Go source code.
// Uses simplified brace counting and pattern matching.
// Limitations:
//   - May not correctly handle functions nested in structs/interfaces
//   - Complex generic signatures may be incomplete
//   - Call graph extraction is same-file only using regex matching
//
// For more accurate parsing, use Tree-sitter parser (ParserModeTreeSitter).
func (p *Parser) parseGoFile(content, filePath string) ([]FunctionEntity, []CallsEdge) {
	var functions []FunctionEntity

	lines := strings.Split(content, "\n")

	// Simple pattern matching for function declarations
	// Pattern: func [Receiver] Name([params]) [return_type] {
	inFunction := false
	var currentFn *FunctionEntity
	var fnStartLine int
	var fnLines []string

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function start
		if strings.HasPrefix(trimmed, "func ") {
			// Save previous function if any
			if currentFn != nil {
				currentFn.EndLine = fnStartLine + len(fnLines) - 1
				codeText := strings.Join(fnLines, "\n")
				currentFn.CodeText = p.truncateCodeText(codeText)
				functions = append(functions, *currentFn)
			}

			// Parse new function
			fnName, signature := p.extractGoFunctionSignature(trimmed)
			if fnName != "" {
				currentFn = &FunctionEntity{
					ID:        GenerateFunctionID(filePath, fnName, signature, lineNum, lineNum, 1, len(line)),
					Name:      fnName,
					Signature: signature,
					FilePath:  filePath,
					StartLine: lineNum,
					EndLine:   lineNum,
					StartCol:  1,
					EndCol:    len(line),
				}
				fnStartLine = lineNum
				fnLines = []string{line}
				inFunction = true
			}
		} else if inFunction {
			fnLines = append(fnLines, line)
			// Detect function end (simple heuristic: closing brace at start of line)
			if trimmed == "}" && len(fnLines) > 1 {
				// Check if this is the function's closing brace
				braceCount := 0
				for _, l := range fnLines {
					braceCount += strings.Count(l, "{") - strings.Count(l, "}")
				}
				if braceCount == 0 {
					inFunction = false
					if currentFn != nil {
						currentFn.EndLine = lineNum
						codeText := strings.Join(fnLines, "\n")
						currentFn.CodeText = p.truncateCodeText(codeText)
						functions = append(functions, *currentFn)
						currentFn = nil
					}
				}
			}
		}
	}

	// Save last function if any
	if currentFn != nil {
		currentFn.EndLine = len(lines)
		codeText := strings.Join(fnLines, "\n")
		currentFn.CodeText = p.truncateCodeText(codeText)
		functions = append(functions, *currentFn)
	}

	// Extract calls (same-file function calls only)
	calls := p.extractGoCallsSimplified(functions, content)

	return functions, calls
}

// extractGoCallsSimplified extracts same-file function calls using pattern matching.
// This is a simplified implementation that detects identifier() patterns and matches
// them against known function names in the file.
func (p *Parser) extractGoCallsSimplified(functions []FunctionEntity, content string) []CallsEdge {
	var calls []CallsEdge

	// Build map of function names to IDs
	funcNameToID := make(map[string]string)
	for _, fn := range functions {
		// Extract simple name (without receiver prefix)
		simpleName := fn.Name
		if idx := strings.LastIndex(fn.Name, "."); idx >= 0 {
			simpleName = fn.Name[idx+1:]
		}
		funcNameToID[simpleName] = fn.ID
	}

	// For each function, find calls to other functions in the file
	for _, caller := range functions {
		callerBody := caller.CodeText

		// Skip the function signature line
		if idx := strings.Index(callerBody, "{"); idx >= 0 {
			callerBody = callerBody[idx+1:]
		}

		// Find all potential function calls using pattern matching
		calledFuncs := p.findGoCalls(callerBody)

		// Match against known functions
		seenCalls := make(map[string]bool)
		for _, calledName := range calledFuncs {
			if calleeID, exists := funcNameToID[calledName]; exists {
				// Skip self-calls and duplicates
				if calleeID == caller.ID {
					continue
				}
				edgeKey := caller.ID + "->" + calleeID
				if seenCalls[edgeKey] {
					continue
				}
				seenCalls[edgeKey] = true
				calls = append(calls, CallsEdge{
					CallerID: caller.ID,
					CalleeID: calleeID,
				})
			}
		}
	}

	return calls
}

// findGoCalls extracts potential function call names from Go code.
// Looks for patterns like: identifier(, obj.method(, etc.
func (p *Parser) findGoCalls(code string) []string {
	var calls []string
	inString := false
	inComment := false
	inLineComment := false

	i := 0
	for i < len(code) {
		// Handle comments
		if !inString && i+1 < len(code) {
			if code[i] == '/' && code[i+1] == '/' {
				inLineComment = true
				i += 2
				continue
			}
			if code[i] == '/' && code[i+1] == '*' {
				inComment = true
				i += 2
				continue
			}
		}
		if inLineComment && code[i] == '\n' {
			inLineComment = false
			i++
			continue
		}
		if inComment && i+1 < len(code) && code[i] == '*' && code[i+1] == '/' {
			inComment = false
			i += 2
			continue
		}
		if inComment || inLineComment {
			i++
			continue
		}

		// Handle strings
		if code[i] == '"' && (i == 0 || code[i-1] != '\\') {
			inString = !inString
			i++
			continue
		}
		if code[i] == '`' {
			// Skip raw string
			i++
			for i < len(code) && code[i] != '`' {
				i++
			}
			i++
			continue
		}
		if inString {
			i++
			continue
		}

		// Look for identifier followed by (
		if isGoIdentStart(code[i]) {
			start := i
			for i < len(code) && isGoIdentChar(code[i]) {
				i++
			}
			name := code[start:i]

			// Skip whitespace
			for i < len(code) && (code[i] == ' ' || code[i] == '\t' || code[i] == '\n') {
				i++
			}

			// Check for ( - this is a function call
			if i < len(code) && code[i] == '(' {
				// Skip keywords
				if !isGoKeyword(name) {
					calls = append(calls, name)
				}
			}
			continue
		}

		i++
	}

	return calls
}

// isGoIdentStart checks if c can start a Go identifier.
func isGoIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// isGoIdentChar checks if c can be part of a Go identifier.
func isGoIdentChar(c byte) bool {
	return isGoIdentStart(c) || (c >= '0' && c <= '9')
}

// isGoKeyword checks if name is a Go keyword.
func isGoKeyword(name string) bool {
	keywords := map[string]bool{
		"break": true, "case": true, "chan": true, "const": true,
		"continue": true, "default": true, "defer": true, "else": true,
		"fallthrough": true, "for": true, "func": true, "go": true,
		"goto": true, "if": true, "import": true, "interface": true,
		"map": true, "package": true, "range": true, "return": true,
		"select": true, "struct": true, "switch": true, "type": true,
		"var": true, "make": true, "new": true, "append": true,
		"copy": true, "delete": true, "len": true, "cap": true,
		"close": true, "panic": true, "recover": true, "print": true,
		"println": true, "complex": true, "real": true, "imag": true,
	}
	return keywords[name]
}

// extractGoFunctionSignature extracts function name and signature from a Go function declaration line.
func (p *Parser) extractGoFunctionSignature(line string) (name, signature string) {
	// Remove "func " prefix
	rest := strings.TrimPrefix(line, "func ")
	if rest == line {
		return "", ""
	}

	// Handle receiver: (r *Receiver) or (r Receiver)
	if strings.HasPrefix(rest, "(") {
		// Find closing paren
		idx := strings.Index(rest, ")")
		if idx == -1 {
			return "", ""
		}
		rest = strings.TrimSpace(rest[idx+1:])
	}

	// Extract function name (up to opening paren)
	parenIdx := strings.Index(rest, "(")
	if parenIdx == -1 {
		return "", ""
	}

	name = strings.TrimSpace(rest[:parenIdx])
	signature = strings.TrimSpace(rest[:strings.Index(rest, "{")])

	return name, signature
}

// =============================================================================
// GO TYPE EXTRACTION
// =============================================================================

// extractGoTypes extracts all type declarations from Go source.
// Handles: struct types, interface types, and type aliases.
func (p *TreeSitterParser) extractGoTypes(rootNode *sitter.Node, content []byte, filePath string) []TypeEntity {
	var types []TypeEntity

	if rootNode == nil {
		return types
	}

	// Walk all top-level declarations looking for type declarations
	p.walkGoTypesAST(rootNode, content, filePath, &types)

	return types
}

// walkGoTypesAST recursively walks the Go AST to find type declarations.
func (p *TreeSitterParser) walkGoTypesAST(node *sitter.Node, content []byte, filePath string, types *[]TypeEntity) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Only process type_declaration at top level and inside type blocks
	if nodeType == "type_declaration" {
		p.extractGoTypeDeclaration(node, content, filePath, types)
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkGoTypesAST(child, content, filePath, types)
	}
}

// extractGoTypeDeclaration extracts types from a type declaration node.
// Handles both single type declarations and type blocks.
func (p *TreeSitterParser) extractGoTypeDeclaration(node *sitter.Node, content []byte, filePath string, types *[]TypeEntity) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)

		switch child.Type() {
		case "type_spec":
			// Single type: type Foo struct { ... }
			te := p.extractGoTypeSpec(child, content, filePath)
			if te != nil {
				*types = append(*types, *te)
			}
		case "type_spec_list":
			// Type block: type ( Foo struct { ... }; Bar interface { ... } )
			for j := 0; j < int(child.ChildCount()); j++ {
				spec := child.Child(j)
				if spec.Type() == "type_spec" {
					te := p.extractGoTypeSpec(spec, content, filePath)
					if te != nil {
						*types = append(*types, *te)
					}
				}
			}
		}
	}
}

// extractGoTypeSpec extracts a single type specification.
func (p *TreeSitterParser) extractGoTypeSpec(node *sitter.Node, content []byte, filePath string) *TypeEntity {
	// Get type name
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		// Fallback: first identifier child
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_identifier" {
				nameNode = child
				break
			}
		}
	}
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	// Get type definition to determine kind
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		// Fallback: look for struct_type, interface_type, or other type nodes
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			childType := child.Type()
			if childType == "struct_type" || childType == "interface_type" ||
				childType == "type_identifier" || childType == "pointer_type" ||
				childType == "array_type" || childType == "slice_type" ||
				childType == "map_type" || childType == "channel_type" ||
				childType == "function_type" || childType == "generic_type" {
				typeNode = child
				break
			}
		}
	}

	// Determine kind based on type definition
	kind := p.determineGoTypeKind(typeNode, content)
	if kind == "" {
		return nil // Skip if we can't determine the kind
	}

	// Get positions (1-indexed)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	// Get code text
	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	// Generate deterministic ID
	id := GenerateTypeID(filePath, name, startLine, endLine)

	return &TypeEntity{
		ID:        id,
		Name:      name,
		Kind:      kind,
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// determineGoTypeKind determines the kind of a Go type.
func (p *TreeSitterParser) determineGoTypeKind(typeNode *sitter.Node, content []byte) string {
	if typeNode == nil {
		return ""
	}

	nodeType := typeNode.Type()

	switch nodeType {
	case "struct_type":
		return "struct"
	case "interface_type":
		return "interface"
	case "type_identifier", "pointer_type", "array_type", "slice_type",
		"map_type", "channel_type", "function_type", "generic_type":
		// These are type aliases: type Foo = Bar, type Foo Bar, type Foo *Bar, etc.
		return "type_alias"
	default:
		// Unknown type, skip it
		return ""
	}
}
