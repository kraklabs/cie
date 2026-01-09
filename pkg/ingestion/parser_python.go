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
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// =============================================================================
// PYTHON PARSER
// =============================================================================

// parsePythonAST extracts functions and types from Python source using Tree-sitter.
func (p *TreeSitterParser) parsePythonAST(content []byte, filePath string) ([]FunctionEntity, []TypeEntity, []CallsEdge, error) {
	tree, err := p.pyParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode.HasError() {
		if errorCount := countErrors(rootNode); errorCount > 0 {
			p.logger.Warn("parser.treesitter.python.syntax_errors",
				"path", filePath,
				"error_count", errorCount,
			)
		}
	}

	var functions []FunctionEntity
	funcNameToID := make(map[string]string)
	anonCounter := 0

	p.walkPythonFunctions(rootNode, content, filePath, &functions, funcNameToID, "", &anonCounter)

	// Extract types (classes in Python)
	types := p.extractPythonTypes(rootNode, content, filePath)

	// Extract calls using stored functions
	var calls []CallsEdge
	for _, fn := range functions {
		fnCalls := p.extractPythonCalls(rootNode, content, fn, funcNameToID)
		calls = append(calls, fnCalls...)
	}

	return functions, types, calls, nil
}

// walkPythonFunctions recursively walks the AST to find function definitions.
func (p *TreeSitterParser) walkPythonFunctions(node *sitter.Node, content []byte, filePath string, functions *[]FunctionEntity, funcNameToID map[string]string, classPrefix string, anonCounter *int) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Handle class definitions (for method prefixing)
	if nodeType == "class_definition" {
		className := ""
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			className = string(content[nameNode.StartByte():nameNode.EndByte()])
		}
		// Walk children with class prefix
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "block" {
				p.walkPythonFunctions(child, content, filePath, functions, funcNameToID, className, anonCounter)
			}
		}
		return
	}

	// Handle function definitions
	if nodeType == "function_definition" {
		fn := p.extractPythonFunction(node, content, filePath, classPrefix)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	// Handle lambda expressions
	if nodeType == "lambda" {
		*anonCounter++
		fn := p.extractPythonLambda(node, content, filePath, *anonCounter)
		if fn != nil {
			*functions = append(*functions, *fn)
		}
	}

	// Recurse into children (but not into class bodies - handled above)
	if nodeType != "class_definition" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			p.walkPythonFunctions(child, content, filePath, functions, funcNameToID, classPrefix, anonCounter)
		}
	}
}

// extractPythonFunction extracts a Python function from a Tree-sitter node.
func (p *TreeSitterParser) extractPythonFunction(node *sitter.Node, content []byte, filePath, classPrefix string) *FunctionEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	// Add class prefix for methods
	fullName := name
	if classPrefix != "" {
		fullName = classPrefix + "." + name
	}

	// Get parameters
	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	// Get return type annotation
	returnNode := node.ChildByFieldName("return_type")
	var returnType string
	if returnNode != nil {
		returnType = string(content[returnNode.StartByte():returnNode.EndByte()])
	}

	// Build signature
	signature := fmt.Sprintf("def %s%s", name, params)
	if returnType != "" {
		signature += " -> " + returnType
	}

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	id := GenerateFunctionID(filePath, fullName, signature, startLine, endLine, startCol, endCol)

	return &FunctionEntity{
		ID:        id,
		Name:      fullName,
		Signature: signature,
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  startCol,
		EndCol:    endCol,
	}
}

// extractPythonLambda extracts a Python lambda expression.
func (p *TreeSitterParser) extractPythonLambda(node *sitter.Node, content []byte, filePath string, index int) *FunctionEntity {
	name := fmt.Sprintf("$lambda_%d", index)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
	codeText = p.truncateCodeText(codeText)

	signature := codeText
	if len(signature) > 100 {
		signature = signature[:100] + "..."
	}

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

// extractPythonCalls extracts function calls within a Python function.
func (p *TreeSitterParser) extractPythonCalls(root *sitter.Node, content []byte, caller FunctionEntity, funcNameToID map[string]string) []CallsEdge {
	var calls []CallsEdge

	fnNode := findNodeAtPosition(root, uint32(caller.StartLine-1), uint32(caller.StartCol-1))
	if fnNode == nil {
		return calls
	}

	p.walkPythonCallExpressions(fnNode, content, caller.ID, funcNameToID, &calls)
	return calls
}

// walkPythonCallExpressions finds call expressions in Python.
func (p *TreeSitterParser) walkPythonCallExpressions(node *sitter.Node, content []byte, callerID string, funcNameToID map[string]string, calls *[]CallsEdge) {
	if node == nil {
		return
	}

	if node.Type() == "call" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			calleeName := p.extractPythonCalleeName(funcNode, content)
			if calleeName != "" {
				if calleeID, exists := funcNameToID[calleeName]; exists && calleeID != callerID {
					*calls = append(*calls, CallsEdge{
						CallerID: callerID,
						CalleeID: calleeID,
					})
				}
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkPythonCallExpressions(child, content, callerID, funcNameToID, calls)
	}
}

// extractPythonCalleeName extracts the function name from a Python call.
func (p *TreeSitterParser) extractPythonCalleeName(node *sitter.Node, content []byte) string {
	nodeType := node.Type()

	if nodeType == "identifier" {
		return string(content[node.StartByte():node.EndByte()])
	}

	if nodeType == "attribute" {
		attrNode := node.ChildByFieldName("attribute")
		if attrNode != nil {
			return string(content[attrNode.StartByte():attrNode.EndByte()])
		}
	}

	return ""
}

// =============================================================================
// PYTHON TYPE EXTRACTION
// =============================================================================

// extractPythonTypes extracts all type declarations from Python source.
// Python supports class definitions.
func (p *TreeSitterParser) extractPythonTypes(rootNode *sitter.Node, content []byte, filePath string) []TypeEntity {
	var types []TypeEntity

	if rootNode == nil {
		return types
	}

	p.walkPythonTypesAST(rootNode, content, filePath, &types)

	return types
}

// walkPythonTypesAST recursively walks the Python AST to find class definitions.
func (p *TreeSitterParser) walkPythonTypesAST(node *sitter.Node, content []byte, filePath string, types *[]TypeEntity) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	if nodeType == "class_definition" {
		te := p.extractPythonClass(node, content, filePath)
		if te != nil {
			*types = append(*types, *te)
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkPythonTypesAST(child, content, filePath, types)
	}
}

// extractPythonClass extracts a Python class definition.
func (p *TreeSitterParser) extractPythonClass(node *sitter.Node, content []byte, filePath string) *TypeEntity {
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

// parsePythonFile extracts functions from Python source code.
// Uses simplified indentation-based detection.
// Limitations: May not handle decorators, nested functions, or complex cases correctly.
// For more accurate parsing, use Tree-sitter parser (ParserModeTreeSitter).
func (p *Parser) parsePythonFile(content, filePath string) ([]FunctionEntity, []CallsEdge) {
	var functions []FunctionEntity

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function definition: def name(params):
		if strings.HasPrefix(trimmed, "def ") {
			// Extract function name and signature
			rest := strings.TrimPrefix(trimmed, "def ")
			colonIdx := strings.Index(rest, ":")
			if colonIdx == -1 {
				continue
			}

			sig := strings.TrimSpace(rest[:colonIdx])
			parenIdx := strings.Index(sig, "(")
			if parenIdx == -1 {
				continue
			}

			name := strings.TrimSpace(sig[:parenIdx])
			signature := sig

			// Find function end (next def/class at same or lower indentation, or EOF)
			endLine := p.findPythonFunctionEnd(lines, i)

			// Extract code text
			codeLines := lines[i:endLine]
			codeText := strings.Join(codeLines, "\n")
			codeText = p.truncateCodeText(codeText)

			fn := FunctionEntity{
				ID:        GenerateFunctionID(filePath, name, signature, lineNum, endLine, 1, len(line)),
				Name:      name,
				Signature: signature,
				FilePath:  filePath,
				CodeText:  codeText,
				StartLine: lineNum,
				EndLine:   endLine,
				StartCol:  1,
				EndCol:    len(line),
			}

			functions = append(functions, fn)
		}
	}

	// Extract calls (same-file function calls)
	calls := p.extractPythonCallsSimplified(functions)

	return functions, calls
}

// extractPythonCallsSimplified extracts same-file function calls using pattern matching.
func (p *Parser) extractPythonCallsSimplified(functions []FunctionEntity) []CallsEdge {
	var calls []CallsEdge

	// Build map of function names to IDs
	funcNameToID := make(map[string]string)
	for _, fn := range functions {
		funcNameToID[fn.Name] = fn.ID
	}

	// For each function, find calls to other functions in the file
	for _, caller := range functions {
		callerBody := caller.CodeText

		// Skip the function definition line
		if idx := strings.Index(callerBody, ":"); idx >= 0 && idx+1 < len(callerBody) {
			callerBody = callerBody[idx+1:]
		}

		// Find all potential function calls
		calledFuncs := p.findPythonCalls(callerBody)

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

// findPythonCalls extracts potential function call names from Python code.
func (p *Parser) findPythonCalls(code string) []string {
	var calls []string
	inString := false
	stringChar := byte(0)
	inComment := false

	i := 0
	for i < len(code) {
		// Handle comments
		if !inString && code[i] == '#' {
			// Skip to end of line
			for i < len(code) && code[i] != '\n' {
				i++
			}
			continue
		}

		// Handle triple-quoted strings
		if !inString && i+2 < len(code) {
			if (code[i] == '"' && code[i+1] == '"' && code[i+2] == '"') ||
				(code[i] == '\'' && code[i+1] == '\'' && code[i+2] == '\'') {
				stringChar = code[i]
				i += 3
				// Find closing triple quote
				for i+2 < len(code) {
					if code[i] == stringChar && code[i+1] == stringChar && code[i+2] == stringChar {
						i += 3
						break
					}
					i++
				}
				continue
			}
		}

		// Handle strings
		if !inString && (code[i] == '"' || code[i] == '\'') {
			stringChar = code[i]
			inString = true
			i++
			continue
		}
		if inString && code[i] == stringChar && (i == 0 || code[i-1] != '\\') {
			inString = false
			i++
			continue
		}
		if inString {
			i++
			continue
		}
		if inComment {
			i++
			continue
		}

		// Look for identifier followed by (
		if isPythonIdentStart(code[i]) {
			start := i
			for i < len(code) && isPythonIdentChar(code[i]) {
				i++
			}
			name := code[start:i]

			// Skip whitespace
			for i < len(code) && (code[i] == ' ' || code[i] == '\t') {
				i++
			}

			// Check for ( - this is a function call
			if i < len(code) && code[i] == '(' {
				// Skip keywords and builtins
				if !isPythonKeyword(name) {
					calls = append(calls, name)
				}
			}
			continue
		}

		i++
	}

	return calls
}

// isPythonIdentStart checks if c can start a Python identifier.
func isPythonIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// isPythonIdentChar checks if c can be part of a Python identifier.
func isPythonIdentChar(c byte) bool {
	return isPythonIdentStart(c) || (c >= '0' && c <= '9')
}

// isPythonKeyword checks if name is a Python keyword or common builtin.
func isPythonKeyword(name string) bool {
	keywords := map[string]bool{
		// Python keywords
		"False": true, "None": true, "True": true, "and": true,
		"as": true, "assert": true, "async": true, "await": true,
		"break": true, "class": true, "continue": true, "def": true,
		"del": true, "elif": true, "else": true, "except": true,
		"finally": true, "for": true, "from": true, "global": true,
		"if": true, "import": true, "in": true, "is": true,
		"lambda": true, "nonlocal": true, "not": true, "or": true,
		"pass": true, "raise": true, "return": true, "try": true,
		"while": true, "with": true, "yield": true,
		// Common builtins
		"print": true, "len": true, "range": true, "str": true,
		"int": true, "float": true, "list": true, "dict": true,
		"set": true, "tuple": true, "type": true, "isinstance": true,
		"hasattr": true, "getattr": true, "setattr": true, "open": true,
		"input": true, "super": true, "self": true,
	}
	return keywords[name]
}

// findPythonFunctionEnd finds the end line of a Python function.
func (p *Parser) findPythonFunctionEnd(lines []string, startIdx int) int {
	if startIdx >= len(lines) {
		return len(lines)
	}

	// Get indentation of function definition
	startLine := lines[startIdx]
	indent := len(startLine) - len(strings.TrimLeft(startLine, " \t"))

	// Find next line at same or lower indentation that's not blank
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))
		if lineIndent <= indent {
			return i
		}
	}

	return len(lines)
}
