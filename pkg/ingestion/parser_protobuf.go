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
	"strings"
)

// =============================================================================
// PROTOBUF PARSER (simplified, no tree-sitter)
// =============================================================================

// parseProtobufContent extracts services, RPC methods, and messages from .proto files.
//
// Extracts:
//   - Services (service definitions)
//   - RPC methods (rpc declarations with request/response types)
//   - Message types (as TypeEntity)
//
// Uses regex-based parsing since tree-sitter-proto is not bundled.
// RPC methods are represented as FunctionEntity with signatures like:
//
//	"rpc MethodName(RequestType) returns (ResponseType)"
//
// This is a shared implementation used by both Parser and TreeSitterParser.
// protoParseState holds state during protobuf parsing.
type protoParseState struct {
	filePath     string
	truncateFunc func(string) string
	lines        []string
	functions    []FunctionEntity
	// Service tracking
	currentService   string
	serviceStartLine int
	serviceLines     []string
	braceCount       int
}

func parseProtobufContent(content string, filePath string, truncateFunc func(string) string) ([]FunctionEntity, []CallsEdge) {
	state := &protoParseState{
		filePath:     filePath,
		truncateFunc: truncateFunc,
		lines:        strings.Split(content, "\n"),
	}

	for i, line := range state.lines {
		state.processLine(i, line)
	}

	return state.functions, nil
}

func (s *protoParseState) processLine(idx int, line string) {
	lineNum := idx + 1
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
		return
	}

	if s.currentService != "" {
		s.processServiceLine(lineNum, line, trimmed)
		return
	}

	s.processTopLevel(idx, lineNum, line, trimmed)
}

func (s *protoParseState) processServiceLine(lineNum int, line, trimmed string) {
	s.serviceLines = append(s.serviceLines, line)
	s.braceCount += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

	if strings.HasPrefix(trimmed, "rpc ") {
		s.handleRPC(lineNum, trimmed)
	}

	if s.braceCount == 0 {
		s.endService(lineNum)
	}
}

func (s *protoParseState) handleRPC(lineNum int, trimmed string) {
	rpcName, rpcSignature := extractRPCSignature(trimmed)
	if rpcName != "" {
		fn := createProtobufEntity(s.filePath, s.currentService+"."+rpcName, rpcSignature, lineNum, lineNum, trimmed, s.truncateFunc)
		s.functions = append(s.functions, fn)
	}
}

func (s *protoParseState) endService(lineNum int) {
	fn := createProtobufEntity(s.filePath, s.currentService, "service "+s.currentService, s.serviceStartLine, lineNum, strings.Join(s.serviceLines, "\n"), s.truncateFunc)
	s.functions = append(s.functions, fn)
	s.currentService = ""
	s.serviceLines = nil
}

func (s *protoParseState) processTopLevel(idx, lineNum int, line, trimmed string) {
	if strings.HasPrefix(trimmed, "service ") && strings.Contains(trimmed, "{") {
		s.startService(lineNum, line, trimmed)
	} else if strings.HasPrefix(trimmed, "message ") && strings.Contains(trimmed, "{") {
		s.handleBlock(idx, lineNum, trimmed, "message")
	} else if strings.HasPrefix(trimmed, "enum ") && strings.Contains(trimmed, "{") {
		s.handleBlock(idx, lineNum, trimmed, "enum")
	}
}

func (s *protoParseState) startService(lineNum int, line, trimmed string) {
	parts := strings.Fields(trimmed)
	if len(parts) < 2 {
		return
	}
	s.currentService = strings.TrimSuffix(parts[1], "{")
	s.serviceStartLine = lineNum
	s.serviceLines = []string{line}
	s.braceCount = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")
	if s.braceCount == 0 {
		s.endService(lineNum)
	}
}

func (s *protoParseState) handleBlock(idx, lineNum int, trimmed, prefix string) {
	parts := strings.Fields(trimmed)
	if len(parts) < 2 {
		return
	}
	name := strings.TrimSuffix(parts[1], "{")
	endLine := findProtobufBlockEnd(s.lines, idx)
	codeText := strings.Join(s.lines[idx:endLine], "\n")
	fn := createProtobufEntity(s.filePath, name, prefix+" "+name, lineNum, endLine, codeText, s.truncateFunc)
	s.functions = append(s.functions, fn)
}

// parseProtobufSimplified extracts services, RPCs, and messages from .proto files.
// Uses regex-based parsing since tree-sitter-proto is not bundled.
func parseProtobufSimplified(content []byte, filePath string, p *TreeSitterParser) ([]FunctionEntity, []CallsEdge) {
	return parseProtobufContent(string(content), filePath, p.truncateCodeText)
}

// extractRPCSignature extracts the RPC name and full signature from a proto rpc line.
func extractRPCSignature(line string) (name, signature string) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(line), "rpc ")
	parenIdx := strings.Index(trimmed, "(")
	if parenIdx == -1 {
		return "", ""
	}

	name = strings.TrimSpace(trimmed[:parenIdx])

	semiIdx := strings.Index(trimmed, ";")
	braceIdx := strings.Index(trimmed, "{")

	endIdx := len(trimmed)
	if semiIdx >= 0 && (braceIdx < 0 || semiIdx < braceIdx) {
		endIdx = semiIdx
	} else if braceIdx >= 0 {
		endIdx = braceIdx
	}

	signature = "rpc " + strings.TrimSpace(trimmed[:endIdx])
	return name, signature
}

// createProtobufEntity creates a FunctionEntity for a protobuf definition.
func createProtobufEntity(filePath, name, signature string, startLine, endLine int, codeText string, truncateFunc func(string) string) FunctionEntity {
	codeText = truncateFunc(codeText)
	return FunctionEntity{
		ID:        GenerateFunctionID(filePath, name, signature, startLine, endLine, 1, 1),
		Name:      name,
		Signature: signature,
		FilePath:  filePath,
		CodeText:  codeText,
		StartLine: startLine,
		EndLine:   endLine,
		StartCol:  1,
		EndCol:    1,
	}
}

// findProtobufBlockEnd finds the end line of a protobuf block (message, enum, etc.).
func findProtobufBlockEnd(lines []string, startIdx int) int {
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
