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
// JAVASCRIPT PARSER
// =============================================================================

// parseJavaScriptAST extracts functions, classes, and call relationships from JavaScript source using Tree-sitter.
//
// Extracts:
//   - Function declarations (function foo() {})
//   - Arrow functions (const foo = () => {})
//   - Function expressions (const foo = function() {})
//   - Classes (class Foo {})
//   - Methods (within classes)
//   - Async functions
//   - Function calls within the file
//
// Handles ES6+ syntax including arrow functions and class methods.
func (p *TreeSitterParser) parseJavaScriptAST(content []byte, filePath string) ([]FunctionEntity, []TypeEntity, []CallsEdge, error) {
	tree, err := p.jsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("tree-sitter parse: %w", err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode.HasError() {
		if errorCount := countErrors(rootNode); errorCount > 0 {
			p.logger.Warn("parser.treesitter.javascript.syntax_errors",
				"path", filePath,
				"error_count", errorCount,
			)
		}
	}

	var functions []FunctionEntity
	funcNameToID := make(map[string]string)
	anonCounter := 0

	p.walkJSFunctions(rootNode, content, filePath, &functions, funcNameToID, &anonCounter)

	// Extract types (classes in JavaScript)
	types := p.extractJSTypes(rootNode, content, filePath)

	// Extract calls
	var calls []CallsEdge
	for _, fn := range functions {
		fnCalls := p.extractJSCalls(rootNode, content, fn, funcNameToID)
		calls = append(calls, fnCalls...)
	}

	return functions, types, calls, nil
}

// walkJSFunctions recursively walks the AST to find JavaScript function declarations.
func (p *TreeSitterParser) walkJSFunctions(node *sitter.Node, content []byte, filePath string, functions *[]FunctionEntity, funcNameToID map[string]string, anonCounter *int) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Function declarations: function foo() {}
	if nodeType == "function_declaration" {
		fn := p.extractJSFunction(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	// Arrow functions: const foo = () => {}
	// Variable declarations with function expressions
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

	// Method definitions in classes/objects
	if nodeType == "method_definition" {
		fn := p.extractJSMethod(node, content, filePath)
		if fn != nil {
			*functions = append(*functions, *fn)
			funcNameToID[fn.Name] = fn.ID
		}
	}

	// Anonymous arrow functions (not assigned to variable)
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

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkJSFunctions(child, content, filePath, functions, funcNameToID, anonCounter)
	}
}

// extractJSFunction extracts a JavaScript function declaration.
func (p *TreeSitterParser) extractJSFunction(node *sitter.Node, content []byte, filePath string) *FunctionEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	signature := fmt.Sprintf("function %s%s", name, params)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
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

// extractJSArrowOrExpressionFunction extracts an arrow function or function expression.
func (p *TreeSitterParser) extractJSArrowOrExpressionFunction(nameNode, valueNode *sitter.Node, content []byte, filePath string) *FunctionEntity {
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	paramsNode := valueNode.ChildByFieldName("parameters")
	if paramsNode == nil {
		paramsNode = valueNode.ChildByFieldName("parameter")
	}
	var params string
	if paramsNode != nil {
		params = string(content[paramsNode.StartByte():paramsNode.EndByte()])
		if !strings.HasPrefix(params, "(") {
			params = "(" + params + ")"
		}
	} else {
		params = "()"
	}

	isArrow := valueNode.Type() == "arrow_function"
	var signature string
	if isArrow {
		signature = fmt.Sprintf("const %s = %s =>", name, params)
	} else {
		signature = fmt.Sprintf("const %s = function%s", name, params)
	}

	startLine := int(nameNode.StartPoint().Row) + 1
	endLine := int(valueNode.EndPoint().Row) + 1
	startCol := int(nameNode.StartPoint().Column) + 1
	endCol := int(valueNode.EndPoint().Column) + 1

	// Get full declaration including const/let/var
	parent := nameNode.Parent()
	if parent != nil {
		grandparent := parent.Parent()
		if grandparent != nil && (grandparent.Type() == "lexical_declaration" || grandparent.Type() == "variable_declaration") {
			startLine = int(grandparent.StartPoint().Row) + 1
			startCol = int(grandparent.StartPoint().Column) + 1
		}
	}

	codeText := string(content[nameNode.StartByte():valueNode.EndByte()])
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

// extractJSMethod extracts a JavaScript method definition.
func (p *TreeSitterParser) extractJSMethod(node *sitter.Node, content []byte, filePath string) *FunctionEntity {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}
	name := string(content[nameNode.StartByte():nameNode.EndByte()])

	paramsNode := node.ChildByFieldName("parameters")
	var params string
	if paramsNode != nil {
		params = string(content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	signature := fmt.Sprintf("%s%s", name, params)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
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

// extractJSAnonymousArrow extracts an anonymous arrow function.
func (p *TreeSitterParser) extractJSAnonymousArrow(node *sitter.Node, content []byte, filePath string, index int) *FunctionEntity {
	name := fmt.Sprintf("$arrow_%d", index)

	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		paramsNode = node.ChildByFieldName("parameter")
	}
	var params string
	if paramsNode != nil {
		params = string(content[paramsNode.StartByte():paramsNode.EndByte()])
	}

	signature := fmt.Sprintf("%s =>", params)

	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1
	startCol := int(node.StartPoint().Column) + 1
	endCol := int(node.EndPoint().Column) + 1

	codeText := string(content[node.StartByte():node.EndByte()])
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

// extractJSCalls extracts function calls within a JavaScript function.
func (p *TreeSitterParser) extractJSCalls(root *sitter.Node, content []byte, caller FunctionEntity, funcNameToID map[string]string) []CallsEdge {
	var calls []CallsEdge

	fnNode := findNodeAtPosition(root, uint32(caller.StartLine-1), uint32(caller.StartCol-1)) //nolint:gosec // G115: line/col from parsed source are bounded
	if fnNode == nil {
		return calls
	}

	p.walkJSCallExpressions(fnNode, content, caller.ID, funcNameToID, &calls)
	return calls
}

// walkJSCallExpressions finds call expressions in JavaScript.
func (p *TreeSitterParser) walkJSCallExpressions(node *sitter.Node, content []byte, callerID string, funcNameToID map[string]string, calls *[]CallsEdge) {
	if node == nil {
		return
	}

	if node.Type() == "call_expression" {
		funcNode := node.ChildByFieldName("function")
		if funcNode != nil {
			calleeName := p.extractJSCalleeName(funcNode, content)
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
		p.walkJSCallExpressions(child, content, callerID, funcNameToID, calls)
	}
}

// extractJSCalleeName extracts the function name from a JavaScript call.
func (p *TreeSitterParser) extractJSCalleeName(node *sitter.Node, content []byte) string {
	nodeType := node.Type()

	if nodeType == "identifier" {
		return string(content[node.StartByte():node.EndByte()])
	}

	if nodeType == "member_expression" {
		propNode := node.ChildByFieldName("property")
		if propNode != nil {
			return string(content[propNode.StartByte():propNode.EndByte()])
		}
	}

	return ""
}

// =============================================================================
// JAVASCRIPT TYPE EXTRACTION
// =============================================================================

// extractJSTypes extracts all type declarations from JavaScript source.
// JavaScript only supports class declarations (no interfaces or type aliases).
func (p *TreeSitterParser) extractJSTypes(rootNode *sitter.Node, content []byte, filePath string) []TypeEntity {
	var types []TypeEntity

	if rootNode == nil {
		return types
	}

	p.walkJSTypesAST(rootNode, content, filePath, &types)

	return types
}

// walkJSTypesAST recursively walks the JavaScript AST to find class declarations.
func (p *TreeSitterParser) walkJSTypesAST(node *sitter.Node, content []byte, filePath string, types *[]TypeEntity) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	if nodeType == "class_declaration" {
		te := p.extractJSClass(node, content, filePath)
		if te != nil {
			*types = append(*types, *te)
		}
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		p.walkJSTypesAST(child, content, filePath, types)
	}
}

// extractJSClass extracts a JavaScript class declaration.
func (p *TreeSitterParser) extractJSClass(node *sitter.Node, content []byte, filePath string) *TypeEntity {
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

// parseJSFile extracts functions from JavaScript/TypeScript source code.
// Uses simplified pattern matching for function declarations and arrow functions.
// Limitations: May not handle all arrow functions, methods, or complex cases correctly.
// For more accurate parsing, use Tree-sitter parser (ParserModeTreeSitter).
func (p *Parser) parseJSFile(content, filePath string) ([]FunctionEntity, []CallsEdge) {
	var functions []FunctionEntity

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect function: function name(...) {
		if strings.Contains(trimmed, "function ") {
			// Extract function name
			parts := strings.Fields(trimmed)
			for j, part := range parts {
				if part == "function" && j+1 < len(parts) {
					name := strings.TrimSuffix(parts[j+1], "(")
					name = strings.TrimSuffix(name, "{")
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					signature := trimmed

					// Find function end (closing brace)
					endLine := p.findJSFunctionEnd(lines, i)

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
					break
				}
			}
		}

		// Detect arrow functions: const/let/var name = (...) => or name = (...) =>
		if strings.Contains(trimmed, "=>") {
			name, signature := p.extractJSArrowFunction(trimmed, line)
			if name != "" {
				endLine := p.findJSFunctionEnd(lines, i)

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
	}

	// Extract calls (same-file function calls)
	calls := p.extractJSCallsSimplified(functions)

	return functions, calls
}

// extractJSArrowFunction extracts name and signature from an arrow function declaration.
func (p *Parser) extractJSArrowFunction(trimmed, line string) (name, signature string) {
	// Pattern: const/let/var name = (...) => or name = (...) =>
	trimmed = strings.TrimPrefix(trimmed, "const ")
	trimmed = strings.TrimPrefix(trimmed, "let ")
	trimmed = strings.TrimPrefix(trimmed, "var ")
	trimmed = strings.TrimPrefix(trimmed, "export ")
	trimmed = strings.TrimPrefix(trimmed, "export default ")

	// Find =
	eqIdx := strings.Index(trimmed, "=")
	if eqIdx == -1 {
		return "", ""
	}

	name = strings.TrimSpace(trimmed[:eqIdx])
	// Validate name (must be identifier)
	if !isValidJSIdentifier(name) {
		return "", ""
	}

	signature = line
	return name, signature
}

// isValidJSIdentifier checks if name is a valid JavaScript identifier.
func isValidJSIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	if !isJSIdentStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isJSIdentChar(name[i]) {
			return false
		}
	}
	return !isJSKeyword(name)
}

// extractJSCallsSimplified extracts same-file function calls using pattern matching.
func (p *Parser) extractJSCallsSimplified(functions []FunctionEntity) []CallsEdge {
	var calls []CallsEdge

	// Build map of function names to IDs
	funcNameToID := make(map[string]string)
	for _, fn := range functions {
		funcNameToID[fn.Name] = fn.ID
	}

	// For each function, find calls to other functions in the file
	for _, caller := range functions {
		callerBody := caller.CodeText

		// Find all potential function calls
		calledFuncs := p.findJSCalls(callerBody)

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

// jsParseState tracks state during JavaScript code parsing.
type jsParseState struct {
	code          string
	pos           int
	inString      bool
	stringChar    byte
	inTemplate    bool
	inComment     bool
	inLineComment bool
}

// findJSCalls extracts potential function call names from JavaScript code.
func (p *Parser) findJSCalls(code string) []string {
	var calls []string
	state := &jsParseState{code: code}

	for state.pos < len(code) {
		if state.handleJSComment() {
			continue
		}
		if state.inComment || state.inLineComment {
			state.pos++
			continue
		}
		if state.handleJSTemplate() {
			continue
		}
		if state.inTemplate {
			state.pos++
			continue
		}
		if state.handleJSString() {
			continue
		}
		if state.inString {
			state.pos++
			continue
		}
		if call := state.extractJSCall(); call != "" {
			calls = append(calls, call)
			continue
		}
		state.pos++
	}
	return calls
}

// handleJSComment handles JavaScript comments.
func (s *jsParseState) handleJSComment() bool {
	if s.inString || s.inTemplate {
		return false
	}
	// Check for comment start
	if s.pos+1 < len(s.code) {
		if s.code[s.pos] == '/' && s.code[s.pos+1] == '/' {
			s.inLineComment = true
			s.pos += 2
			return true
		}
		if s.code[s.pos] == '/' && s.code[s.pos+1] == '*' {
			s.inComment = true
			s.pos += 2
			return true
		}
	}
	// Check for comment end
	if s.inLineComment && s.pos < len(s.code) && s.code[s.pos] == '\n' {
		s.inLineComment = false
		s.pos++
		return true
	}
	if s.inComment && s.pos+1 < len(s.code) && s.code[s.pos] == '*' && s.code[s.pos+1] == '/' {
		s.inComment = false
		s.pos += 2
		return true
	}
	return false
}

// handleJSTemplate handles template literal boundaries.
func (s *jsParseState) handleJSTemplate() bool {
	if s.inString || s.pos >= len(s.code) || s.code[s.pos] != '`' {
		return false
	}
	s.inTemplate = !s.inTemplate
	s.pos++
	return true
}

// handleJSString handles string start/end.
func (s *jsParseState) handleJSString() bool {
	if s.pos >= len(s.code) {
		return false
	}
	c := s.code[s.pos]
	if !s.inString && (c == '"' || c == '\'') {
		s.stringChar = c
		s.inString = true
		s.pos++
		return true
	}
	if s.inString && c == s.stringChar && (s.pos == 0 || s.code[s.pos-1] != '\\') {
		s.inString = false
		s.pos++
		return true
	}
	return false
}

// extractJSCall extracts a function call if present.
func (s *jsParseState) extractJSCall() string {
	if s.pos >= len(s.code) || !isJSIdentStart(s.code[s.pos]) {
		return ""
	}
	start := s.pos
	for s.pos < len(s.code) && isJSIdentChar(s.code[s.pos]) {
		s.pos++
	}
	name := s.code[start:s.pos]

	// Skip whitespace
	for s.pos < len(s.code) && (s.code[s.pos] == ' ' || s.code[s.pos] == '\t' || s.code[s.pos] == '\n') {
		s.pos++
	}

	if s.pos < len(s.code) && s.code[s.pos] == '(' && !isJSKeyword(name) {
		return name
	}
	return ""
}

// isJSIdentStart checks if c can start a JavaScript identifier.
func isJSIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$'
}

// isJSIdentChar checks if c can be part of a JavaScript identifier.
func isJSIdentChar(c byte) bool {
	return isJSIdentStart(c) || (c >= '0' && c <= '9')
}

// isJSKeyword checks if name is a JavaScript keyword.
func isJSKeyword(name string) bool {
	keywords := map[string]bool{
		// JavaScript keywords
		"break": true, "case": true, "catch": true, "continue": true,
		"debugger": true, "default": true, "delete": true, "do": true,
		"else": true, "finally": true, "for": true, "function": true,
		"if": true, "in": true, "instanceof": true, "new": true,
		"return": true, "switch": true, "this": true, "throw": true,
		"try": true, "typeof": true, "var": true, "void": true,
		"while": true, "with": true, "class": true, "const": true,
		"enum": true, "export": true, "extends": true, "import": true,
		"super": true, "implements": true, "interface": true, "let": true,
		"package": true, "private": true, "protected": true, "public": true,
		"static": true, "yield": true, "async": true, "await": true,
		// Common globals
		"console": true, "window": true, "document": true, "process": true,
		"require": true, "module": true, "exports": true, "global": true,
		"undefined": true, "null": true, "true": true, "false": true,
		"NaN": true, "Infinity": true, "Array": true, "Object": true,
		"String": true, "Number": true, "Boolean": true, "Symbol": true,
		"Promise": true, "Map": true, "Set": true, "WeakMap": true,
		"WeakSet": true, "Error": true, "JSON": true, "Math": true,
		"Date": true, "RegExp": true, "parseInt": true, "parseFloat": true,
		"isNaN": true, "isFinite": true, "setTimeout": true, "setInterval": true,
	}
	return keywords[name]
}

// findJSFunctionEnd finds the end line of a JavaScript function.
func (p *Parser) findJSFunctionEnd(lines []string, startIdx int) int {
	braceCount := 0
	started := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		braceCount += strings.Count(line, "{") - strings.Count(line, "}")
		if !started && strings.Contains(line, "{") {
			started = true
		}
		if started && braceCount == 0 {
			return i + 1
		}
	}

	return len(lines)
}
