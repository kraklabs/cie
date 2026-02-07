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

	// Find source and target functions
	sources, err := getTraceSources(ctx, client, args)
	if err != nil {
		return NewResult(err.Error()), nil
	}
	targets := findFunctionsByName(ctx, client, args.Target, args.PathPattern)
	if len(targets) == 0 {
		return NewResult(fmt.Sprintf("Target function '%s' not found.", args.Target)), nil
	}

	// Build target set for quick lookup
	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[t.Name] = true
	}

	// Run BFS search
	searchResult := runTraceSearch(ctx, client, sources, targetSet, args)
	if searchResult.canceled {
		return NewResult("Search canceled (timeout or cancellation)."), nil
	}

	// Format and return output
	if len(searchResult.paths) == 0 {
		return NewResult(formatTraceNotFound(sources, args, searchResult)), nil
	}
	return NewResult(formatTraceOutput(sources, args, searchResult)), nil
}

// getTraceSources finds source functions for tracing.
func getTraceSources(ctx context.Context, client Querier, args TracePathArgs) ([]TraceFuncInfo, error) {
	if args.Source == "" {
		sources := detectEntryPoints(ctx, client, args.PathPattern)
		if len(sources) == 0 {
			return nil, fmt.Errorf("no entry points found: try specifying a 'source' function explicitly")
		}
		return sources, nil
	}
	sources := findFunctionsByName(ctx, client, args.Source, args.PathPattern)
	if len(sources) == 0 {
		return nil, fmt.Errorf("source function %q not found", args.Source)
	}
	return sources, nil
}

// traceSearchResult holds the result of a trace search.
type traceSearchResult struct {
	paths         [][]TraceFuncInfo
	nodesExplored int
	limitReached  bool
	canceled      bool
	deepestPath   []TraceFuncInfo // longest partial path explored (when no full path found)
}

// pathNode represents a node in the BFS traversal.
type pathNode struct {
	funcName string
	path     []TraceFuncInfo
}

// runTraceSearch performs BFS search from sources to targets.
func runTraceSearch(ctx context.Context, client Querier, sources []TraceFuncInfo, targetSet map[string]bool, args TracePathArgs) traceSearchResult {
	const maxNodesExplored = 5000
	const maxQueriesPerSource = 1000

	result := traceSearchResult{}
	calleesCache := make(map[string][]TraceFuncInfo)

	for _, src := range sources {
		if len(result.paths) >= args.MaxPaths {
			break
		}
		select {
		case <-ctx.Done():
			result.canceled = true
			return result
		default:
		}

		srcResult := searchFromSource(ctx, client, src, targetSet, args, calleesCache, &result.nodesExplored, maxNodesExplored, maxQueriesPerSource)
		result.paths = append(result.paths, srcResult.paths...)
		// Track the deepest partial path across all sources
		if len(srcResult.deepestPath) > len(result.deepestPath) {
			result.deepestPath = srcResult.deepestPath
		}
		if srcResult.limitReached {
			result.limitReached = true
			break
		}
		if srcResult.canceled {
			result.canceled = true
			return result
		}
	}
	return result
}

// searchFromSource performs BFS from a single source function.
func searchFromSource(ctx context.Context, client Querier, src TraceFuncInfo, targetSet map[string]bool, args TracePathArgs, calleesCache map[string][]TraceFuncInfo, totalNodes *int, maxNodes, maxQueries int) traceSearchResult {
	result := traceSearchResult{}
	visited := make(map[string]bool)
	queue := []pathNode{{funcName: src.Name, path: []TraceFuncInfo{src}}}
	queries := 0

	for len(queue) > 0 && len(result.paths) < args.MaxPaths {
		if *totalNodes >= maxNodes || queries >= maxQueries {
			result.limitReached = true
			return result
		}
		if *totalNodes%100 == 0 {
			select {
			case <-ctx.Done():
				result.canceled = true
				return result
			default:
			}
		}

		current := queue[0]
		queue = queue[1:]

		if len(current.path) > args.MaxDepth || visited[current.funcName] {
			continue
		}
		visited[current.funcName] = true
		*totalNodes++

		// Track deepest partial path for diagnostic output
		if len(current.path) > len(result.deepestPath) {
			result.deepestPath = current.path
		}

		if targetSet[current.funcName] && len(current.path) > 1 {
			result.paths = append(result.paths, current.path)
			continue
		}

		callees, cached := calleesCache[current.funcName]
		if !cached {
			callees = getCallees(ctx, client, current.funcName)
			calleesCache[current.funcName] = callees
			queries++
		}

		for _, callee := range callees {
			if !visited[callee.Name] {
				newPath := make([]TraceFuncInfo, len(current.path), len(current.path)+1)
				copy(newPath, current.path)
				queue = append(queue, pathNode{funcName: callee.Name, path: append(newPath, callee)})
			}
		}
	}
	return result
}

// formatTraceNotFound formats the output when no paths are found.
func formatTraceNotFound(sources []TraceFuncInfo, args TracePathArgs, result traceSearchResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "No path found from %s to '%s' within depth %d.\n\n",
		formatSources(sources, args.Source == ""), args.Target, args.MaxDepth)
	fmt.Fprintf(&sb, "_Explored %d nodes before stopping._\n\n", result.nodesExplored)

	// Show partial path to help diagnose where the trace got stuck
	if len(result.deepestPath) > 1 {
		sb.WriteString("**Deepest partial path explored:**\n```\n")
		for i, fn := range result.deepestPath {
			indent := strings.Repeat("  ", i)
			arrow := ""
			if i > 0 {
				arrow = "→ "
			}
			fmt.Fprintf(&sb, "%s%s%s\n", indent, arrow, fn.Name)
			fmt.Fprintf(&sb, "%s   %s:%s\n", indent, ExtractFileName(fn.FilePath), fn.Line)
		}
		lastFn := result.deepestPath[len(result.deepestPath)-1]
		fmt.Fprintf(&sb, "```\n_Chain stopped at `%s` — no outgoing calls reached the target._\n\n", lastFn.Name)
	}

	if result.limitReached {
		sb.WriteString("**Note:** Search limit reached (explored 5000 nodes). The path may exist but wasn't found in the explored portion of the call graph.\n\n")
	}
	sb.WriteString("**Tips:**\n")
	sb.WriteString("- Try increasing `max_depth` if the target is deeply nested\n")
	sb.WriteString("- Use `path_pattern` to narrow the search scope (e.g., `path_pattern=\"apps/core\"`)\n")
	sb.WriteString("- Check if the target function name is correct with `cie_find_function`\n")
	sb.WriteString("- Specify a `source` function closer to the target to reduce search space\n")
	sb.WriteString("- Ensure the codebase was re-indexed with the latest CIE version (`cie index --full`)\n")
	return sb.String()
}

// formatTraceOutput formats the output when paths are found.
func formatTraceOutput(sources []TraceFuncInfo, args TracePathArgs, result traceSearchResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Call Paths to `%s`\n\n", args.Target)
	fmt.Fprintf(&sb, "Found %d path(s) from %s\n", len(result.paths), formatSources(sources, args.Source == ""))
	fmt.Fprintf(&sb, "_Explored %d nodes._\n\n", result.nodesExplored)

	for i, path := range result.paths {
		fmt.Fprintf(&sb, "### Path %d (depth: %d)\n\n```\n", i+1, len(path)-1)
		for j, fn := range path {
			indent := strings.Repeat("  ", j)
			arrow := ""
			if j > 0 {
				arrow = "→ "
			}
			fmt.Fprintf(&sb, "%s%s%s\n", indent, arrow, fn.Name)
			fmt.Fprintf(&sb, "%s   %s:%s\n", indent, ExtractFileName(fn.FilePath), fn.Line)
		}
		sb.WriteString("```\n\n")
	}

	if len(result.paths) >= args.MaxPaths {
		fmt.Fprintf(&sb, "*Showing first %d paths. Use `max_paths` to see more.*\n", args.MaxPaths)
	}
	if result.limitReached {
		sb.WriteString("\n**Note:** Search limit reached. There may be additional paths not shown.\n")
	}
	return sb.String()
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

// getCallees returns functions called by the given function.
// Includes both direct call edges (cie_calls) and interface dispatch
// (cie_field + cie_implements → concrete method implementations).
func getCallees(ctx context.Context, client Querier, funcName string) []TraceFuncInfo {
	// 1. Direct callees via cie_calls
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

	seen := make(map[string]bool)
	var ret []TraceFuncInfo
	for _, row := range result.Rows {
		name := AnyToString(row[0])
		seen[name] = true
		ret = append(ret, TraceFuncInfo{
			Name:     name,
			FilePath: AnyToString(row[1]),
			Line:     AnyToString(row[2]),
		})
	}

	// 2. Interface dispatch callees
	// If the caller is a method (e.g., "Builder.Build"), find its struct's
	// interface-typed fields and resolve to concrete implementations
	structName := extractStructName(funcName)
	if structName != "" {
		dispatchScript := fmt.Sprintf(
			`?[callee_name, callee_file, callee_line] :=
				*cie_field { struct_name: %q, field_type },
				*cie_implements { interface_name },
				(field_type = interface_name or ends_with(field_type, concat(".", interface_name))),
				*cie_implements { interface_name, type_name: impl_type },
				impl_prefix = concat(impl_type, "."),
				*cie_function { name: callee_name, file_path: callee_file, start_line: callee_line },
				starts_with(callee_name, impl_prefix)
			:limit 50`,
			structName,
		)

		dispatchResult, err := client.Query(ctx, dispatchScript)
		if err == nil {
			for _, row := range dispatchResult.Rows {
				name := AnyToString(row[0])
				if !seen[name] {
					seen[name] = true
					ret = append(ret, TraceFuncInfo{
						Name:     name,
						FilePath: AnyToString(row[1]),
						Line:     AnyToString(row[2]),
					})
				}
			}
		}
	}

	return ret
}

// extractStructName extracts the struct name from a qualified method name.
// e.g., "Builder.Build" → "Builder", "main" → ""
func extractStructName(funcName string) string {
	if idx := strings.Index(funcName, "."); idx > 0 {
		return funcName[:idx]
	}
	return ""
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
