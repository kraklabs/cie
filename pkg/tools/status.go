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

// IndexStatus shows the indexing status for the project or a specific path
func IndexStatus(ctx context.Context, client *CIEClient, pathPattern string) (*ToolResult, error) {
	var output string
	var errors []string

	// Helper to run query with error tracking
	runQuery := func(name, query string) *QueryResult {
		result, err := client.Query(ctx, query)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", name, err))
			return nil
		}
		return result
	}

	// Robust count helper: tries aggregation first, falls back to counting rows
	countEntities := func(name, countQuery, listQuery string) int {
		// Try the count aggregation query first
		result := runQuery(name, countQuery)
		if result != nil && len(result.Rows) > 0 {
			if cnt, ok := result.Rows[0][0].(float64); ok {
				return int(cnt)
			}
		}
		// Fallback: list entities and count rows
		result = runQuery(name+" (fallback)", listQuery)
		if result != nil {
			return len(result.Rows)
		}
		return 0
	}

	output = "# CIE Index Status\n\n"
	output += fmt.Sprintf("**Project:** `%s`\n", client.ProjectID)
	output += fmt.Sprintf("**Edge Cache:** `%s`\n\n", client.BaseURL)

	// Get total counts with fallback
	totalFileCount := countEntities("total files",
		`?[count(f)] := *cie_file { id: f }`,
		`?[id] := *cie_file { id } :limit 10000`)
	totalFuncCount := countEntities("total functions",
		`?[count(f)] := *cie_function { id: f }`,
		`?[id] := *cie_function { id } :limit 10000`)

	// Count functions with embeddings - Schema v3: embeddings are in cie_function_embedding table
	embeddingCount := countEntities("embeddings",
		`?[count(f)] := *cie_function_embedding { function_id: f, embedding }, embedding != null`,
		`?[function_id] := *cie_function_embedding { function_id, embedding }, embedding != null :limit 10000`)

	output += "## Overall Index\n"
	output += fmt.Sprintf("- **Files:** %d\n", totalFileCount)
	output += fmt.Sprintf("- **Functions:** %d\n", totalFuncCount)
	output += fmt.Sprintf("- **Embeddings:** %d", embeddingCount)
	if totalFuncCount > 0 {
		pct := float64(embeddingCount) / float64(totalFuncCount) * 100
		output += fmt.Sprintf(" (%.0f%%)", pct)
	}
	output += "\n"

	// Check HNSW index status - Schema v3: HNSW is on cie_function_embedding
	hnswResult := runQuery("hnsw index", `::indices cie_function_embedding`)
	hasHNSW := hnswResult != nil && len(hnswResult.Rows) > 0
	if hasHNSW {
		output += "- **HNSW Index:** ✅ ready\n"
	} else if embeddingCount > 0 {
		output += "- **HNSW Index:** ⚠️ not created (semantic search may be slow)\n"
	}

	// Show warning if no embeddings
	if embeddingCount == 0 && totalFuncCount > 0 {
		output += "\n⚠️ **No embeddings found!** Semantic search will use text fallback.\n"
		output += "To enable semantic search: `ollama serve && cie index`\n"
	} else if embeddingCount > 0 && !hasHNSW {
		output += "\n⚠️ **HNSW index missing!** Remount project to create: restart Edge Cache pod\n"
	}

	// Check if index is empty
	if totalFileCount == 0 && totalFuncCount == 0 {
		output += "\n⚠️ **Index is empty!**\n\n"
		output += "### Possible causes:\n"
		output += "1. The project hasn't been indexed yet\n"
		output += "2. The Edge Cache is not connected to the Primary Hub\n"
		output += "3. The project_id doesn't match the indexed project\n\n"
		output += "### How to fix:\n"
		output += "```bash\n"
		output += "# Run indexing from the project root:\n"
		output += "cd /path/to/your/project\n"
		output += "cie index\n"
		output += "```\n"
		return NewResult(output), nil
	}

	// If path pattern provided, show stats for that path
	if pathPattern != "" {
		output += fmt.Sprintf("\n## Path: `%s`\n", pathPattern)

		// Use robust counting with fallback for path-specific counts
		pathFileCount := countEntities("path files",
			fmt.Sprintf(`?[count(f)] := *cie_file { id: f, path }, regex_matches(path, %q)`, pathPattern),
			fmt.Sprintf(`?[id] := *cie_file { id, path }, regex_matches(path, %q) :limit 10000`, pathPattern))
		pathFuncCount := countEntities("path functions",
			fmt.Sprintf(`?[count(f)] := *cie_function { id: f, file_path }, regex_matches(file_path, %q)`, pathPattern),
			fmt.Sprintf(`?[id] := *cie_function { id, file_path }, regex_matches(file_path, %q) :limit 10000`, pathPattern))

		output += fmt.Sprintf("- **Files:** %d\n", pathFileCount)
		output += fmt.Sprintf("- **Functions:** %d\n", pathFuncCount)

		if pathFileCount == 0 && pathFuncCount == 0 {
			output += "\n⚠️ **No files indexed for this path!**\n\n"
			output += "### Possible causes:\n"
			output += fmt.Sprintf("1. Path pattern `%s` doesn't match any files in the project\n", pathPattern)
			output += "2. Files in this path were excluded by `.cie/project.yaml` exclude patterns\n"
			output += "3. Files are in a format CIE doesn't support (binary files, images, etc.)\n\n"
			output += "### How to check:\n"
			output += "- Use `cie_list_files` to see what paths are actually indexed\n"
			output += "- Check your `.cie/project.yaml` for exclude patterns\n"
			output += "- Try a broader path pattern (e.g., 'apps' instead of 'apps/gateway')\n"
		} else {
			// Show percentage of total
			filePercent := 0.0
			funcPercent := 0.0
			if totalFileCount > 0 {
				filePercent = float64(pathFileCount) / float64(totalFileCount) * 100
			}
			if totalFuncCount > 0 {
				funcPercent = float64(pathFuncCount) / float64(totalFuncCount) * 100
			}
			output += fmt.Sprintf("\n_This path represents %.1f%% of files and %.1f%% of functions_\n", filePercent, funcPercent)

			// Show sample files
			sampleFiles := runQuery("sample files", fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :limit 10`, pathPattern))
			if sampleFiles != nil && len(sampleFiles.Rows) > 0 {
				output += "\n### Sample indexed files:\n"
				for i, row := range sampleFiles.Rows {
					if i >= 5 {
						output += fmt.Sprintf("_... and %d more_\n", len(sampleFiles.Rows)-5)
						break
					}
					output += fmt.Sprintf("- `%s`\n", row[0])
				}
			}
		}
	} else {
		// Show language breakdown
		langQuery := `?[lang, count(f)] := *cie_file { id: f, language: lang } :order -count(f) :limit 10`
		if langResult := runQuery("languages", langQuery); langResult != nil && len(langResult.Rows) > 0 {
			output += "\n### By Language:\n"
			for _, row := range langResult.Rows {
				output += fmt.Sprintf("- %s: %v files\n", row[0], row[1])
			}
		}

		// Show top directories
		filesResult := runQuery("files for dirs", `?[path] := *cie_file { path } :limit 500`)
		if filesResult != nil && len(filesResult.Rows) > 0 {
			dirs := make(map[string]int)
			for _, row := range filesResult.Rows {
				if fp, ok := row[0].(string); ok {
					dir := ExtractTopDir(fp)
					dirs[dir]++
				}
			}
			output += "\n### Top Directories:\n"
			// Sort by count (simple approach)
			for dir, count := range dirs {
				output += fmt.Sprintf("- `%s/`: %d files\n", dir, count)
			}
		}
	}

	if len(errors) > 0 {
		output += "\n---\n### Query Errors\n"
		for _, e := range errors {
			output += fmt.Sprintf("- %s\n", e)
		}
		output += "\n_Some queries failed. The Edge Cache may be unavailable or the project may not be fully indexed._\n"
		output += "\n### Troubleshooting:\n"
		output += fmt.Sprintf("1. Check Edge Cache is running: `curl %s/health`\n", client.BaseURL)
		output += "2. Re-run indexing: `cie index --force-full-reindex`\n"
	}

	return NewResult(output), nil
}
