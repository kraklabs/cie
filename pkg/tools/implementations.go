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

package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// FindImplementationsArgs holds arguments for finding interface implementations.
type FindImplementationsArgs struct {
	InterfaceName string
	PathPattern   string // Optional file path filter
	Limit         int
}

// FindImplementations finds types that implement a given interface.
// For Go: searches for types with methods matching the interface's method signatures.
// For TypeScript: searches for classes with "implements InterfaceName".
func FindImplementations(ctx context.Context, client Querier, args FindImplementationsArgs) (*ToolResult, error) {
	if args.InterfaceName == "" {
		return NewError("Error: 'interface_name' is required"), nil
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}

	// Step 1: Find the interface definition to get its methods
	// Schema v3: Join with cie_type_code for code_text
	interfaceQuery := fmt.Sprintf(
		`?[name, kind, file_path, code_text, start_line] :=
		*cie_type { id, name, kind, file_path, start_line },
		*cie_type_code { type_id: id, code_text },
		name == %q, kind == "interface" :limit 1`,
		args.InterfaceName,
	)

	interfaceResult, err := client.Query(ctx, interfaceQuery)
	if err != nil {
		return NewError(fmt.Sprintf("Query error: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Implementations of `%s`\n\n", args.InterfaceName))

	if len(interfaceResult.Rows) == 0 {
		// Interface not found in cie_type - try text search fallback
		return findImplementationsByTextSearch(ctx, client, args, &sb)
	}

	// Extract interface info
	row := interfaceResult.Rows[0]
	interfaceFile := AnyToString(row[2])
	interfaceCode := AnyToString(row[3])
	interfaceLine := AnyToString(row[4])

	sb.WriteString(fmt.Sprintf("**Interface defined in**: %s:%s\n\n", interfaceFile, interfaceLine))

	// Step 2: Extract method names from interface code
	methods := extractMethodNames(interfaceCode)
	if len(methods) == 0 {
		sb.WriteString("Could not extract methods from interface definition.\n\n")
		return findImplementationsByTextSearch(ctx, client, args, &sb)
	}

	sb.WriteString(fmt.Sprintf("**Methods**: %s\n\n", strings.Join(methods, ", ")))

	// Step 3: Find types that have ALL these methods
	// We search for functions with receiver types that match these method names
	implementations := findTypesWithMethods(ctx, client, methods, args.PathPattern, args.Limit)

	if len(implementations) == 0 {
		sb.WriteString("No implementations found.\n\n")
		sb.WriteString("**Tips:**\n")
		sb.WriteString("- The interface may be implemented in external packages\n")
		sb.WriteString("- Try `cie_grep` to search for method signatures\n")
		return NewResult(sb.String()), nil
	}

	sb.WriteString(fmt.Sprintf("**Found %d implementation(s):**\n\n", len(implementations)))
	for i, impl := range implementations {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, impl.TypeName))
		sb.WriteString(fmt.Sprintf("   File: %s:%s\n", impl.FilePath, impl.Line))
		sb.WriteString(fmt.Sprintf("   Methods: %s\n\n", strings.Join(impl.Methods, ", ")))
	}

	return NewResult(sb.String()), nil
}

// implementationInfo holds information about a type that implements an interface.
type implementationInfo struct {
	TypeName string
	FilePath string
	Line     string
	Methods  []string
}

// extractMethodNames extracts method names from Go interface code.
// Example: "type Reader interface { Read(p []byte) (n int, err error) }" -> ["Read"]
func extractMethodNames(code string) []string {
	var methods []string

	// Pattern to match method declarations in interface
	// Matches: MethodName(params) returnType
	// Also matches: MethodName(params)
	methodPattern := regexp.MustCompile(`(?m)^\s*([A-Z][a-zA-Z0-9_]*)\s*\(`)

	matches := methodPattern.FindAllStringSubmatch(code, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			methods = append(methods, match[1])
			seen[match[1]] = true
		}
	}

	return methods
}

// findTypesWithMethods finds types that have methods matching the given names.
func findTypesWithMethods(ctx context.Context, client Querier, methods []string, pathPattern string, limit int) []implementationInfo {
	if len(methods) == 0 {
		return nil
	}

	// Build query to find functions that are methods (have receiver) with matching names
	// We look for method names that match the interface methods
	var conditions []string
	for _, method := range methods {
		// Match functions ending with .MethodName (receiver methods)
		conditions = append(conditions, fmt.Sprintf("ends_with(name, %q)", "."+method))
	}

	// Query for each method and find common receiver types
	receiverMethods := make(map[string][]string) // receiver -> [methods]
	receiverFiles := make(map[string]string)     // receiver -> file
	receiverLines := make(map[string]string)     // receiver -> line

	for _, method := range methods {
		query := fmt.Sprintf(
			`?[name, file_path, start_line] :=
			*cie_function { name, file_path, start_line },
			ends_with(name, %q)`,
			"."+method,
		)
		if pathPattern != "" {
			query = fmt.Sprintf(
				`?[name, file_path, start_line] :=
				*cie_function { name, file_path, start_line },
				ends_with(name, %q), regex_matches(file_path, %q)`,
				"."+method, pathPattern,
			)
		}
		query += " :limit 100"

		result, err := client.Query(ctx, query)
		if err != nil {
			continue
		}

		for _, row := range result.Rows {
			fullName := AnyToString(row[0])
			filePath := AnyToString(row[1])
			line := AnyToString(row[2])

			// Extract receiver type from "ReceiverType.MethodName"
			parts := strings.Split(fullName, ".")
			if len(parts) >= 2 {
				receiver := strings.Join(parts[:len(parts)-1], ".")
				receiverMethods[receiver] = append(receiverMethods[receiver], method)
				if _, ok := receiverFiles[receiver]; !ok {
					receiverFiles[receiver] = filePath
					receiverLines[receiver] = line
				}
			}
		}
	}

	// Find receivers that have ALL methods
	var implementations []implementationInfo
	for receiver, methodList := range receiverMethods {
		if len(methodList) >= len(methods) {
			// Check if it has all required methods
			hasAll := true
			for _, m := range methods {
				found := false
				for _, rm := range methodList {
					if rm == m {
						found = true
						break
					}
				}
				if !found {
					hasAll = false
					break
				}
			}
			if hasAll {
				implementations = append(implementations, implementationInfo{
					TypeName: receiver,
					FilePath: receiverFiles[receiver],
					Line:     receiverLines[receiver],
					Methods:  methodList,
				})
			}
		}
	}

	// Limit results
	if len(implementations) > limit {
		implementations = implementations[:limit]
	}

	return implementations
}

// findImplementationsByTextSearch uses text search as fallback.
func findImplementationsByTextSearch(ctx context.Context, client Querier, args FindImplementationsArgs, sb *strings.Builder) (*ToolResult, error) {
	sb.WriteString("**Using text search fallback**\n\n")

	// For TypeScript/JavaScript: search for "implements InterfaceName"
	// Schema v3: Join with cie_type_code for code_text
	tsQuery := fmt.Sprintf(
		`?[name, file_path, start_line, code_text] :=
		*cie_type { id, name, file_path, start_line },
		*cie_type_code { type_id: id, code_text },
		regex_matches(code_text, "implements.*%s")`,
		EscapeRegex(args.InterfaceName),
	)
	if args.PathPattern != "" {
		tsQuery = fmt.Sprintf(
			`?[name, file_path, start_line, code_text] :=
			*cie_type { id, name, file_path, start_line },
			*cie_type_code { type_id: id, code_text },
			regex_matches(code_text, "implements.*%s"), regex_matches(file_path, %q)`,
			EscapeRegex(args.InterfaceName), args.PathPattern,
		)
	}
	tsQuery += fmt.Sprintf(" :limit %d", args.Limit)

	tsResult, err := client.Query(ctx, tsQuery)
	if err == nil && len(tsResult.Rows) > 0 {
		sb.WriteString(fmt.Sprintf("**Found %d class(es) implementing `%s`:**\n\n", len(tsResult.Rows), args.InterfaceName))
		for i, row := range tsResult.Rows {
			name := AnyToString(row[0])
			filePath := AnyToString(row[1])
			line := AnyToString(row[2])
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, name))
			sb.WriteString(fmt.Sprintf("   File: %s:%s\n\n", filePath, line))
		}
		return NewResult(sb.String()), nil
	}

	// For Go: search for methods that might implement the interface
	// Search for the interface name in function signatures (common pattern)
	goQuery := fmt.Sprintf(
		`?[name, file_path, start_line] :=
		*cie_function { name, file_path, start_line, signature },
		regex_matches(signature, %q)`,
		EscapeRegex(args.InterfaceName),
	)
	goQuery += fmt.Sprintf(" :limit %d", args.Limit)

	goResult, err := client.Query(ctx, goQuery)
	if err == nil && len(goResult.Rows) > 0 {
		sb.WriteString(fmt.Sprintf("**Found %d function(s) referencing `%s` in signature:**\n\n", len(goResult.Rows), args.InterfaceName))
		for i, row := range goResult.Rows {
			name := AnyToString(row[0])
			filePath := AnyToString(row[1])
			line := AnyToString(row[2])
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, name))
			sb.WriteString(fmt.Sprintf("   File: %s:%s\n\n", filePath, line))
		}
		return NewResult(sb.String()), nil
	}

	sb.WriteString("No implementations found.\n\n")
	sb.WriteString("**Tips:**\n")
	sb.WriteString(fmt.Sprintf("- Use `cie_find_type` to find the interface: `%s`\n", args.InterfaceName))
	sb.WriteString("- Use `cie_grep` to search for method signatures\n")
	sb.WriteString("- The interface may be implemented in external packages\n")

	return NewResult(sb.String()), nil
}
