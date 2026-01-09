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

package tools

import (
	"context"
	"fmt"
	"strings"
)

// TraceFuncInfo holds function metadata for call path tracing
type TraceFuncInfo struct {
	Name     string
	FilePath string
	Line     string
}

// TracePathArgs holds arguments for tracing call paths
type TracePathArgs struct {
	Target      string
	Source      string
	PathPattern string
	MaxPaths    int
	MaxDepth    int
}

// TracePath traces call paths from source function(s) to a target function
func TracePath(ctx context.Context, client Querier, args TracePathArgs) (*ToolResult, error) {
	if args.Target == "" {
		return NewError("Error: 'target' function name is required"), nil
	}

	// Safety limits to prevent hanging on large codebases
	const maxNodesExplored = 5000    // Maximum nodes to visit in BFS
	const maxQueriesPerSource = 1000 // Maximum getCallees queries per source

	// pathNode represents a node in the BFS traversal
	type pathNode struct {
		funcName string
		path     []TraceFuncInfo // path from source to this node
	}

	// Find source functions (entry points or specified source)
	var sources []TraceFuncInfo
	if args.Source == "" {
		// Auto-detect entry points based on language conventions
		sources = detectEntryPoints(ctx, client, args.PathPattern)
		if len(sources) == 0 {
			return NewResult("No entry points found. Try specifying a 'source' function explicitly."), nil
		}
	} else {
		// Find specified source function
		sources = findFunctionsByName(ctx, client, args.Source, args.PathPattern)
		if len(sources) == 0 {
			return NewResult(fmt.Sprintf("Source function '%s' not found.", args.Source)), nil
		}
	}

	// Find target functions
	targets := findFunctionsByName(ctx, client, args.Target, args.PathPattern)
	if len(targets) == 0 {
		return NewResult(fmt.Sprintf("Target function '%s' not found.", args.Target)), nil
	}

	// Build target set for quick lookup
	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[t.Name] = true
	}

	// BFS to find shortest paths from sources to target
	var foundPaths [][]TraceFuncInfo
	var searchLimitReached bool
	totalNodesExplored := 0

	// Cache for getCallees to avoid duplicate queries
	calleesCache := make(map[string][]TraceFuncInfo)

	for _, src := range sources {
		if len(foundPaths) >= args.MaxPaths {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return NewResult("Search cancelled (timeout or cancellation)."), nil
		default:
		}

		// BFS from this source
		visited := make(map[string]bool)
		queue := []pathNode{{
			funcName: src.Name,
			path:     []TraceFuncInfo{src},
		}}
		queriesThisSource := 0

		for len(queue) > 0 && len(foundPaths) < args.MaxPaths {
			// Check if we've hit safety limits
			if totalNodesExplored >= maxNodesExplored || queriesThisSource >= maxQueriesPerSource {
				searchLimitReached = true
				break
			}

			// Check context cancellation periodically
			if totalNodesExplored%100 == 0 {
				select {
				case <-ctx.Done():
					return NewResult("Search cancelled (timeout or cancellation)."), nil
				default:
				}
			}

			current := queue[0]
			queue = queue[1:]

			if len(current.path) > args.MaxDepth {
				continue
			}

			if visited[current.funcName] {
				continue
			}
			visited[current.funcName] = true
			totalNodesExplored++

			// Check if we reached target
			if targetSet[current.funcName] && len(current.path) > 1 {
				foundPaths = append(foundPaths, current.path)
				continue
			}

			// Get callees of current function (with caching)
			callees, cached := calleesCache[current.funcName]
			if !cached {
				callees = getCallees(ctx, client, current.funcName)
				calleesCache[current.funcName] = callees
				queriesThisSource++
			}

			for _, callee := range callees {
				if !visited[callee.Name] {
					newPath := make([]TraceFuncInfo, len(current.path))
					copy(newPath, current.path)
					newPath = append(newPath, callee)
					queue = append(queue, pathNode{
						funcName: callee.Name,
						path:     newPath,
					})
				}
			}
		}

		if searchLimitReached {
			break
		}
	}

	if len(foundPaths) == 0 {
		output := fmt.Sprintf("No path found from %s to '%s' within depth %d.\n\n",
			formatSources(sources, args.Source == ""), args.Target, args.MaxDepth)
		output += fmt.Sprintf("_Explored %d nodes before stopping._\n\n", totalNodesExplored)
		if searchLimitReached {
			output += "**Note:** Search limit reached (explored 5000 nodes). The path may exist but wasn't found in the explored portion of the call graph.\n\n"
		}
		output += "**Tips:**\n"
		output += "- Try increasing `max_depth` if the target is deeply nested\n"
		output += "- Use `path_pattern` to narrow the search scope (e.g., `path_pattern=\"apps/core\"`)\n"
		output += "- Check if the target function name is correct with `cie_find_function`\n"
		output += "- Specify a `source` function closer to the target to reduce search space\n"
		output += "- The call might be through an interface or dynamic dispatch (not statically traceable)\n"
		return NewResult(output), nil
	}

	// Format output
	output := fmt.Sprintf("## Call Paths to `%s`\n\n", args.Target)
	output += fmt.Sprintf("Found %d path(s) from %s\n", len(foundPaths), formatSources(sources, args.Source == ""))
	output += fmt.Sprintf("_Explored %d nodes._\n\n", totalNodesExplored)

	for i, path := range foundPaths {
		output += fmt.Sprintf("### Path %d (depth: %d)\n\n", i+1, len(path)-1)
		output += "```\n"
		for j, fn := range path {
			indent := strings.Repeat("  ", j)
			arrow := ""
			if j > 0 {
				arrow = "→ "
			}
			fileName := ExtractFileName(fn.FilePath)
			output += fmt.Sprintf("%s%s%s\n", indent, arrow, fn.Name)
			output += fmt.Sprintf("%s   %s:%s\n", indent, fileName, fn.Line)
		}
		output += "```\n\n"
	}

	if len(foundPaths) >= args.MaxPaths {
		output += fmt.Sprintf("*Showing first %d paths. Use `max_paths` to see more.*\n", args.MaxPaths)
	}
	if searchLimitReached {
		output += "\n**Note:** Search limit reached. There may be additional paths not shown.\n"
	}

	return NewResult(output), nil
}

// detectEntryPoints finds entry point functions based on language conventions
func detectEntryPoints(ctx context.Context, client Querier, pathPattern string) []TraceFuncInfo {
	var results []TraceFuncInfo

	// Entry point patterns for different languages
	// Go/Rust: main functions
	// JS/TS: exports in index/app/server files
	// Python: __main__ blocks (represented as functions)
	// Note: Use [.] instead of \. for CozoDB regex compatibility
	patterns := []struct {
		namePattern string
		filePattern string
	}{
		// Go: main function
		{`^main$`, `[.]go$`},
		// Rust: main function
		{`^main$`, `[.]rs$`},
		// JS/TS: common entry point file patterns
		{`.*`, `(index|app|server|main)[.](js|ts|mjs|cjs)$`},
		// Python: module entry points
		{`^(__main__|main)$`, `[.]py$`},
	}

	for _, p := range patterns {
		var conditions []string
		conditions = append(conditions, fmt.Sprintf("regex_matches(name, %q)", p.namePattern))
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %q)", p.filePattern))
		if pathPattern != "" {
			conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %q)", pathPattern))
		}
		// Exclude test files (use [.] instead of \. for CozoDB compatibility)
		conditions = append(conditions, `!regex_matches(file_path, "_test[.]go|test_|[.]test[.](js|ts)")`)

		script := fmt.Sprintf(
			"?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, %s :limit 20",
			strings.Join(conditions, ", "),
		)

		result, err := client.Query(ctx, script)
		if err != nil {
			continue
		}

		for _, row := range result.Rows {
			results = append(results, TraceFuncInfo{
				Name:     AnyToString(row[0]),
				FilePath: AnyToString(row[1]),
				Line:     AnyToString(row[2]),
			})
		}
	}

	return results
}

// findFunctionsByName finds functions matching a name pattern
func findFunctionsByName(ctx context.Context, client Querier, name, pathPattern string) []TraceFuncInfo {
	var conditions []string
	// Try exact match first, then suffix match for method names (e.g., "Run" matches "Agent.Run")
	// Use OR condition: exact match OR ends with .name
	conditions = append(conditions, fmt.Sprintf("(name = %q or ends_with(name, %q))", name, "."+name))
	if pathPattern != "" {
		conditions = append(conditions, fmt.Sprintf("regex_matches(file_path, %q)", pathPattern))
	}

	script := fmt.Sprintf(
		"?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, %s :limit 50",
		strings.Join(conditions, ", "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil
	}

	var ret []TraceFuncInfo
	for _, row := range result.Rows {
		ret = append(ret, TraceFuncInfo{
			Name:     AnyToString(row[0]),
			FilePath: AnyToString(row[1]),
			Line:     AnyToString(row[2]),
		})
	}
	return ret
}

// getCallees returns functions called by the given function
func getCallees(ctx context.Context, client Querier, funcName string) []TraceFuncInfo {
	// Join cie_calls with cie_function to get callee details
	// Match caller by exact name or suffix (for methods like Struct.Method)
	script := fmt.Sprintf(
		`?[callee_name, callee_file, callee_line] :=
			*cie_calls { caller_id, callee_id },
			*cie_function { id: caller_id, name: caller_name },
			*cie_function { id: callee_id, file_path: callee_file, name: callee_name, start_line: callee_line },
			(caller_name = %q or ends_with(caller_name, %q))
		:limit 100`,
		funcName, "."+funcName,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil
	}

	var ret []TraceFuncInfo
	for _, row := range result.Rows {
		ret = append(ret, TraceFuncInfo{
			Name:     AnyToString(row[0]),
			FilePath: AnyToString(row[1]),
			Line:     AnyToString(row[2]),
		})
	}
	return ret
}

// formatSources formats the source list for display
func formatSources(sources []TraceFuncInfo, autoDetected bool) string {
	if len(sources) == 0 {
		return "unknown"
	}
	if len(sources) == 1 {
		return fmt.Sprintf("`%s`", sources[0].Name)
	}
	if autoDetected {
		return fmt.Sprintf("%d auto-detected entry points", len(sources))
	}
	return fmt.Sprintf("%d matching functions", len(sources))
}
