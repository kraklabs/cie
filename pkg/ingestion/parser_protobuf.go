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
	"strings"
)

// =============================================================================
// PROTOBUF PARSER (simplified, no tree-sitter)
// =============================================================================

// parseProtobufContent extracts services, RPCs, and messages from .proto files.
// Uses regex-based parsing since tree-sitter-proto is not bundled.
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
