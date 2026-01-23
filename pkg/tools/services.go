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
)

// ListServices lists gRPC services and RPC methods from .proto files.
//
// It scans indexed .proto files and extracts service definitions and their RPC methods.
// Services are identified by parsing function-like entities in proto files.
//
// The pathPattern parameter filters proto files by path (regex), leave empty for all files.
// The serviceName parameter filters by service name (case-insensitive regex), leave empty for all services.
//
// Returns a ToolResult containing a formatted list of proto files and their service definitions,
// grouped by file. Returns an error if the query execution fails.
//
// Note: Proto files must be indexed with proto support enabled. If no services are found,
// check if .proto files are excluded in .cie/project.yaml configuration.
func ListServices(ctx context.Context, client Querier, pathPattern, serviceName string) (*ToolResult, error) {
	// First, find .proto files
	protoQuery := `?[path] := *cie_file { path }, regex_matches(path, "[.]proto$") :limit 100`
	if pathPattern != "" {
		protoQuery = fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, "[.]proto$"), regex_matches(path, %q) :limit 100`, pathPattern)
	}

	protoFiles, err := client.Query(ctx, protoQuery)
	if err != nil {
		return nil, fmt.Errorf("query proto files: %w", err)
	}

	if len(protoFiles.Rows) == 0 {
		return NewResult("No .proto files found in the index.\n\n**Note:** Proto files need to be indexed. Check if they're excluded in `.cie/project.yaml`."), nil
	}

	// Search for service/rpc definitions in function code
	// Services in proto are parsed as "functions" with names like "ServiceName" or "ServiceName.MethodName"
	var conditions []string
	conditions = append(conditions, `regex_matches(file_path, "[.]proto$")`)

	if serviceName != "" {
		conditions = append(conditions, fmt.Sprintf(`regex_matches(name, %q)`, "(?i)"+EscapeRegex(serviceName)))
	}

	// Query functions from proto files - these are the service/rpc definitions
	script := fmt.Sprintf(`?[file_path, name, signature, start_line] := *cie_function { file_path, name, signature, start_line }, %s :limit 100`,
		conditions[0])
	if len(conditions) > 1 {
		script = fmt.Sprintf(`?[file_path, name, signature, start_line] := *cie_function { file_path, name, signature, start_line }, %s, %s :limit 100`,
			conditions[0], conditions[1])
	}

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query service definitions: %w", err)
	}

	output := "# gRPC Services\n\n"
	output += fmt.Sprintf("Found %d .proto files\n\n", len(protoFiles.Rows))

	// List proto files
	output += "## Proto Files\n"
	for _, row := range protoFiles.Rows {
		output += fmt.Sprintf("- `%s`\n", row[0])
	}

	if len(result.Rows) > 0 {
		output += "\n## Service Definitions\n"

		// Group by file
		fileServices := make(map[string][]string)
		for _, row := range result.Rows {
			filePath := AnyToString(row[0])
			name := AnyToString(row[1])
			signature := AnyToString(row[2])
			startLine := AnyToString(row[3])

			entry := fmt.Sprintf("- **%s** (line %s)\n  `%s`", name, startLine, signature)
			fileServices[filePath] = append(fileServices[filePath], entry)
		}

		for file, services := range fileServices {
			output += fmt.Sprintf("\n### %s\n", file)
			for _, svc := range services {
				output += svc + "\n"
			}
		}
	} else {
		output += "\n_No service/rpc definitions found in proto files._\n"
		output += "\n**Note:** Proto file parsing may require re-indexing with proto support enabled.\n"
	}

	return NewResult(output), nil
}

// RoleFiltersWithCustom returns CozoScript filter conditions for a given role, supporting custom roles.
//
// It first checks if the role exists in the customRoles map. If found, it builds filter conditions
// based on the custom role's FilePattern, NamePattern, and CodePattern fields.
// If not found, it falls back to built-in roles from RoleFilters().
//
// The role parameter specifies which role to filter for (e.g., "source", "test", "entry_point", or custom).
// The customRoles parameter provides a map of custom role definitions with their matching patterns.
//
// Returns a slice of CozoScript condition strings that can be added to a query's WHERE clause.
// An empty slice is returned if the role is "any" or unrecognized.
func RoleFiltersWithCustom(role string, customRoles map[string]RolePattern) []string {
	// Check for custom role first
	if customRole, ok := customRoles[role]; ok {
		var conditions []string
		if customRole.FilePattern != "" {
			conditions = append(conditions, fmt.Sprintf(`regex_matches(file_path, %q)`, customRole.FilePattern))
		}
		if customRole.NamePattern != "" {
			conditions = append(conditions, fmt.Sprintf(`regex_matches(name, %q)`, customRole.NamePattern))
		}
		if customRole.CodePattern != "" {
			conditions = append(conditions, fmt.Sprintf(`regex_matches(code_text, %q)`, customRole.CodePattern))
		}
		return conditions
	}

	// Fall back to built-in roles
	return RoleFilters(role)
}
