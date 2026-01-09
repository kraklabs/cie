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

// parseTypeScriptAST extracts functions and types from TypeScript source using Tree-sitter.
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

// walkTSFunctions walks TypeScript AST (extends JS walker with TS-specific nodes).
func (p *TreeSitterParser) walkTSFunctions(node *sitter.Node, content []byte, filePath string, functions *[]FunctionEntity, funcNameToID map[string]string, anonCounter *int) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// All JS function types
	if nodeType == "function_declaration" {
		fn := p.extractJSFunction(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	if nodeType == "variable_declarator" {
		nameNode := node.ChildByFieldName("name")
		valueNode := node.ChildByFieldName("value")
		if nameNode != nil && valueNode != nil {
			valueType := valueNode.Type()
			if valueType == "arrow_function" || valueType == "function_expression" || valueType == "function" {
				fn := p.extractJSArrowOrExpressionFunction(nameNode, valueNode, content, filePath)
				if fn != nil {
					*functions = append(*functions, *fn)
					funcNameToID[fn.Name] = fn.ID
				}
			}
		}
	}

	if nodeType == "method_definition" {
		fn := p.extractJSMethod(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	// TypeScript-specific: method_signature in interfaces
	if nodeType == "method_signature" {
		fn := p.extractTSMethodSignature(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	// TypeScript-specific: function_signature in declarations
	if nodeType == "function_signature" {
		fn := p.extractTSFunctionSignature(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	if nodeType == "arrow_function" {
		parent := node.Parent()
		if parent == nil || parent.Type() != "variable_declarator" {
			*anonCounter++
			fn := p.extractJSAnonymousArrow(node, content, filePath, *anonCounter)
			if fn != nil {
				*functions = append(*functions, *fn)
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkTSFunctions(child, content, filePath, functions, funcNameToID, anonCounter)
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
