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
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
)

// =============================================================================
// TYPESCRIPT PARSER
// =============================================================================

// parseTypeScriptAST extracts functions, classes, interfaces, and call relationships from TypeScript source using Tree-sitter.
//
// Extracts:
//   - Function declarations (function foo() {})
//   - Arrow functions (const foo = () => {})
//   - Function expressions (const foo = function() {})
//   - Classes (class Foo {})
//   - Interfaces (interface Bar {})
//   - Type aliases (type Baz = ...)
//   - Methods (within classes)
//   - Async functions
//   - Function calls within the file
//
// Handles TypeScript-specific syntax including interfaces and type aliases.
func (p *TreeSitterParser) parseTypeScriptAST(content []byte, filePath string) ([]FunctionEntity, []TypeEntity, []CallsEdge, error) {
	tree, err := p.tsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode.HasError() {
		if errorCount := countErrors(rootNode); errorCount > 0 {
			p.logger.Warn("parser.treesitter.typescript.syntax_errors",
				"path", filePath,
				"error_count", errorCount,
			)
		}
	}

	var functions []FunctionEntity
	funcNameToID := make(map[string]string)
	anonCounter := 0

	p.walkTSFunctions(rootNode, content, filePath, &functions, funcNameToID, &anonCounter)

	// Extract types (interfaces, classes, type aliases)
	types := p.extractTSTypes(rootNode, content, filePath)

	// Extract calls
	var calls []CallsEdge
	for _, fn := range functions {
		fnCalls := p.extractJSCalls(rootNode, content, fn, funcNameToID)
		calls = append(calls, fnCalls...)
	}

	return functions, types, calls, nil
}

// tsWalkContext holds context for TypeScript AST walking.
type tsWalkContext struct {
	content      []byte
	filePath     string
	functions    *[]FunctionEntity
	funcNameToID map[string]string
	anonCounter  *int
}

// walkTSFunctions walks TypeScript AST (extends JS walker with TS-specific nodes).
func (p *TreeSitterParser) walkTSFunctions(node *sitter.Node, content []byte, filePath string, functions *[]FunctionEntity, funcNameToID map[string]string, anonCounter *int) {
	if node == nil {
		return
	}

	ctx := &tsWalkContext{
		content:      content,
		filePath:     filePath,
		functions:    functions,
		funcNameToID: funcNameToID,
		anonCounter:  anonCounter,
	}

	p.processTSNode(node, ctx)

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkTSFunctions(child, content, filePath, functions, funcNameToID, anonCounter)
	}
}

// processTSNode handles a single TypeScript node during AST walk.
func (p *TreeSitterParser) processTSNode(node *sitter.Node, ctx *tsWalkContext) {
	switch node.Type() {
	case "function_declaration":
		p.handleTSFunctionDecl(node, ctx)
	case "variable_declarator":
		p.handleTSVariableDeclarator(node, ctx)
	case "method_definition":
		p.handleTSMethodDef(node, ctx)
	case "method_signature":
		p.handleTSMethodSig(node, ctx)
	case "function_signature":
		p.handleTSFunctionSig(node, ctx)
	case "arrow_function":
		p.handleTSArrowFunction(node, ctx)
	}
}

// handleTSFunctionDecl handles a function declaration node.
func (p *TreeSitterParser) handleTSFunctionDecl(node *sitter.Node, ctx *tsWalkContext) {
	fn := p.extractJSFunction(node, ctx.content, ctx.filePath)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
		ctx.funcNameToID[fn.Name] = fn.ID
	}
}

// handleTSVariableDeclarator handles a variable declarator node (arrow/function expressions).
func (p *TreeSitterParser) handleTSVariableDeclarator(node *sitter.Node, ctx *tsWalkContext) {
	nameNode := node.ChildByFieldName("name")
	valueNode := node.ChildByFieldName("value")
	if nameNode == nil || valueNode == nil {
		return
	}

	valueType := valueNode.Type()
	if valueType != "arrow_function" && valueType != "function_expression" && valueType != "function" {
		return
	}

	fn := p.extractJSArrowOrExpressionFunction(nameNode, valueNode, ctx.content, ctx.filePath)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
		ctx.funcNameToID[fn.Name] = fn.ID
	}
}

// handleTSMethodDef handles a method definition node.
func (p *TreeSitterParser) handleTSMethodDef(node *sitter.Node, ctx *tsWalkContext) {
	fn := p.extractJSMethod(node, ctx.content, ctx.filePath)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
		ctx.funcNameToID[fn.Name] = fn.ID
	}
}

// handleTSMethodSig handles a method signature node (TypeScript interface method).
func (p *TreeSitterParser) handleTSMethodSig(node *sitter.Node, ctx *tsWalkContext) {
	fn := p.extractTSMethodSignature(node, ctx.content, ctx.filePath)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
		ctx.funcNameToID[fn.Name] = fn.ID
	}
}

// handleTSFunctionSig handles a function signature node (TypeScript declaration).
func (p *TreeSitterParser) handleTSFunctionSig(node *sitter.Node, ctx *tsWalkContext) {
	fn := p.extractTSFunctionSignature(node, ctx.content, ctx.filePath)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
		ctx.funcNameToID[fn.Name] = fn.ID
	}
}

// handleTSArrowFunction handles an anonymous arrow function node.
func (p *TreeSitterParser) handleTSArrowFunction(node *sitter.Node, ctx *tsWalkContext) {
	parent := node.Parent()
	if parent != nil && parent.Type() == "variable_declarator" {
		return // Already handled by variable_declarator case
	}

	*ctx.anonCounter++
	fn := p.extractJSAnonymousArrow(node, ctx.content, ctx.filePath, *ctx.anonCounter)
	if fn != nil {
		*ctx.functions = append(*ctx.functions, *fn)
	}
}

// extractTSMethodSignature extracts a TypeScript method signature (interface method).
func (p *TreeSitterParser) extractTSMethodSignature(node *sitter.Node, content []byte, filePath string) *FunctionEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	signature := string(content[node.StartByte():node.EndByte()])

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := signature
	codeText = p.truncateCodeText(codeText)

	id := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol, endCol)

	return &FunctionEntity{
		ID:        id,
		Name:      name,
		Signature: signature,
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// extractTSFunctionSignature extracts a TypeScript function signature (declaration).
func (p *TreeSitterParser) extractTSFunctionSignature(node *sitter.Node, content []byte, filePath string) *FunctionEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	signature := string(content[node.StartByte():node.EndByte()])

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := signature
	codeText = p.truncateCodeText(codeText)

	id := GenerateFunctionID(filePath, name, signature, startLine, endLine, startCol, endCol)

	return &FunctionEntity{
		ID:        id,
		Name:      name,
		Signature: signature,
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// =============================================================================
// TYPESCRIPT TYPE EXTRACTION
// =============================================================================

// extractTSTypes extracts all type declarations from TypeScript source.
// Handles: interface, class, and type alias declarations.
func (p *TreeSitterParser) extractTSTypes(rootNode *sitter.Node, content []byte, filePath string) []TypeEntity {
	var types []TypeEntity

	if rootNode == nil {
		return types
	}

	p.walkTSTypesAST(rootNode, content, filePath, &types)

	return types
}

// walkTSTypesAST recursively walks the TypeScript AST to find type declarations.
func (p *TreeSitterParser) walkTSTypesAST(node *sitter.Node, content []byte, filePath string, types *[]TypeEntity) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "interface_declaration":
		te := p.extractTSInterface(node, content, filePath)
		if te != nil {
			*types = append(*types, *te)
		}
	case "class_declaration":
		te := p.extractTSClass(node, content, filePath)
		if te != nil {
			*types = append(*types, *te)
		}
	case "type_alias_declaration":
		te := p.extractTSTypeAlias(node, content, filePath)
		if te != nil {
			*types = append(*types, *te)
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkTSTypesAST(child, content, filePath, types)
	}
}

// extractTSInterface extracts a TypeScript interface declaration.
func (p *TreeSitterParser) extractTSInterface(node *sitter.Node, content []byte, filePath string) *TypeEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	id := GenerateTypeID(filePath, name, startLine, endLine)

	return &TypeEntity{
		ID:        id,
		Name:      name,
		Kind:      "interface",
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// extractTSClass extracts a TypeScript class declaration.
func (p *TreeSitterParser) extractTSClass(node *sitter.Node, content []byte, filePath string) *TypeEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	id := GenerateTypeID(filePath, name, startLine, endLine)

	return &TypeEntity{
		ID:        id,
		Name:      name,
		Kind:      "class",
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// extractTSTypeAlias extracts a TypeScript type alias declaration.
func (p *TreeSitterParser) extractTSTypeAlias(node *sitter.Node, content []byte, filePath string) *TypeEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	id := GenerateTypeID(filePath, name, startLine, endLine)

	return &TypeEntity{
		ID:        id,
		Name:      name,
		Kind:      "type_alias",
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}
