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
func parseProtobufContent(content string, filePath string, truncateFunc func(string) string) ([]FunctionEntity, []CallsEdge) {
	var functions []FunctionEntity

	lines := strings.Split(content, "\n")
	var currentService string
	var serviceStartLine int
	var serviceLines []string
	braceCount := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}

		// Detect service: service ServiceName {
		if strings.HasPrefix(trimmed, "service ") && strings.Contains(trimmed, "{") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				currentService = strings.TrimSuffix(parts[1], "{")
				serviceStartLine = lineNum
				serviceLines = []string{line}
				braceCount = strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

				if braceCount == 0 {
					fn := createProtobufEntity(filePath, currentService, "service "+currentService, serviceStartLine, lineNum, strings.Join(serviceLines, "\n"), truncateFunc)
					functions = append(functions, fn)
					currentService = ""
				}
			}
			continue
		}

		// Track braces in service block
		if currentService != "" {
			serviceLines = append(serviceLines, line)
			braceCount += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

			// Detect RPC
			if strings.HasPrefix(trimmed, "rpc ") {
				rpcName, rpcSignature := extractRPCSignature(trimmed)
				if rpcName != "" {
					fullName := currentService + "." + rpcName
					fn := createProtobufEntity(filePath, fullName, rpcSignature, lineNum, lineNum, trimmed, truncateFunc)
					functions = append(functions, fn)
				}
			}

			// End of service
			if braceCount == 0 {
				fn := createProtobufEntity(filePath, currentService, "service "+currentService, serviceStartLine, lineNum, strings.Join(serviceLines, "\n"), truncateFunc)
				functions = append(functions, fn)
				currentService = ""
				serviceLines = nil
			}
			continue
		}

		// Detect message
		if strings.HasPrefix(trimmed, "message ") && strings.Contains(trimmed, "{") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				msgName := strings.TrimSuffix(parts[1], "{")
				endLine := findProtobufBlockEnd(lines, i)

				codeLines := lines[i:endLine]
				codeText := strings.Join(codeLines, "\n")

				fn := createProtobufEntity(filePath, msgName, "message "+msgName, lineNum, endLine, codeText, truncateFunc)
				functions = append(functions, fn)
			}
		}

		// Detect enum
		if strings.HasPrefix(trimmed, "enum ") && strings.Contains(trimmed, "{") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				enumName := strings.TrimSuffix(parts[1], "{")
				endLine := findProtobufBlockEnd(lines, i)

				codeLines := lines[i:endLine]
				codeText := strings.Join(codeLines, "\n")

				fn := createProtobufEntity(filePath, enumName, "enum "+enumName, lineNum, endLine, codeText, truncateFunc)
				functions = append(functions, fn)
			}
		}
	}

	return functions, nil
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
