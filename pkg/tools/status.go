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

// indexStatusState holds state for index status queries.
type indexStatusState struct {
	ctx    context.Context
	client *CIEClient
	errors []string
}

// runQuery executes a query with error tracking.
func (s *indexStatusState) runQuery(name, query string) *QueryResult {
	result, err := s.client.Query(s.ctx, query)
	if err != nil {
		s.errors = append(s.errors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	return result
}

// countEntities counts entities using aggregation with fallback.
func (s *indexStatusState) countEntities(name, countQuery, listQuery string) int {
	result := s.runQuery(name, countQuery)
	if result != nil && len(result.Rows) > 0 {
		if cnt, ok := result.Rows[0][0].(float64); ok {
			return int(cnt)
		}
	}
	result = s.runQuery(name+" (fallback)", listQuery)
	if result != nil {
		return len(result.Rows)
	}
	return 0
}

// IndexStatus shows the indexing status for the project or a specific path
func IndexStatus(ctx context.Context, client *CIEClient, pathPattern string) (*ToolResult, error) {
	state := &indexStatusState{ctx: ctx, client: client}

	output := fmt.Sprintf("# CIE Index Status\n\n**Project:** `%s`\n**Edge Cache:** `%s`\n\n", client.ProjectID, client.BaseURL)

	// Get total counts
	counts := state.getOverallCounts()
	output += state.formatOverallStats(counts)

	// Check for empty index
	if counts.files == 0 && counts.functions == 0 {
		output += formatEmptyIndexHelp()
		return NewResult(output), nil
	}

	// Path-specific or overall breakdown
	if pathPattern != "" {
		output += state.formatPathStats(pathPattern, counts)
	} else {
		output += state.formatOverallBreakdown()
	}

	// Add errors if any
	output += state.formatErrors()

	return NewResult(output), nil
}

type indexCounts struct {
	files, functions, embeddings int
	hasHNSW                      bool
}

func (s *indexStatusState) getOverallCounts() indexCounts {
	c := indexCounts{}
	c.files = s.countEntities("total files", `?[count(f)] := *cie_file { id: f }`, `?[id] := *cie_file { id } :limit 10000`)
	c.functions = s.countEntities("total functions", `?[count(f)] := *cie_function { id: f }`, `?[id] := *cie_function { id } :limit 10000`)
	c.embeddings = s.countEntities("embeddings", `?[count(f)] := *cie_function_embedding { function_id: f, embedding }, embedding != null`, `?[function_id] := *cie_function_embedding { function_id, embedding }, embedding != null :limit 10000`)
	hnswResult := s.runQuery("hnsw index", `::indices cie_function_embedding`)
	c.hasHNSW = hnswResult != nil && len(hnswResult.Rows) > 0
	return c
}

func (s *indexStatusState) formatOverallStats(c indexCounts) string {
	output := "## Overall Index\n"
	output += fmt.Sprintf("- **Files:** %d\n- **Functions:** %d\n- **Embeddings:** %d", c.files, c.functions, c.embeddings)
	if c.functions > 0 {
		output += fmt.Sprintf(" (%.0f%%)", float64(c.embeddings)/float64(c.functions)*100)
	}
	output += "\n"
	if c.hasHNSW {
		output += "- **HNSW Index:** ✅ ready\n"
	} else if c.embeddings > 0 {
		output += "- **HNSW Index:** ⚠️ not created (semantic search may be slow)\n"
	}
	if c.embeddings == 0 && c.functions > 0 {
		output += "\n⚠️ **No embeddings found!** Semantic search will use text fallback.\nTo enable semantic search: `ollama serve && cie index`\n"
	} else if c.embeddings > 0 && !c.hasHNSW {
		output += "\n⚠️ **HNSW index missing!** Remount project to create: restart Edge Cache pod\n"
	}
	return output
}

func formatEmptyIndexHelp() string {
	return "\n⚠️ **Index is empty!**\n\n### Possible causes:\n1. The project hasn't been indexed yet\n2. The Edge Cache is not connected to the Primary Hub\n3. The project_id doesn't match the indexed project\n\n### How to fix:\n```bash\n# Run indexing from the project root:\ncd /path/to/your/project\ncie index\n```\n"
}

func (s *indexStatusState) formatPathStats(pathPattern string, total indexCounts) string {
	output := fmt.Sprintf("\n## Path: `%s`\n", pathPattern)
	pathFiles := s.countEntities("path files", fmt.Sprintf(`?[count(f)] := *cie_file { id: f, path }, regex_matches(path, %q)`, pathPattern), fmt.Sprintf(`?[id] := *cie_file { id, path }, regex_matches(path, %q) :limit 10000`, pathPattern))
	pathFuncs := s.countEntities("path functions", fmt.Sprintf(`?[count(f)] := *cie_function { id: f, file_path }, regex_matches(file_path, %q)`, pathPattern), fmt.Sprintf(`?[id] := *cie_function { id, file_path }, regex_matches(file_path, %q) :limit 10000`, pathPattern))

	output += fmt.Sprintf("- **Files:** %d\n- **Functions:** %d\n", pathFiles, pathFuncs)

	if pathFiles == 0 && pathFuncs == 0 {
		output += fmt.Sprintf("\n⚠️ **No files indexed for this path!**\n\n### Possible causes:\n1. Path pattern `%s` doesn't match any files in the project\n2. Files in this path were excluded by `.cie/project.yaml` exclude patterns\n3. Files are in a format CIE doesn't support (binary files, images, etc.)\n\n### How to check:\n- Use `cie_list_files` to see what paths are actually indexed\n- Check your `.cie/project.yaml` for exclude patterns\n- Try a broader path pattern (e.g., 'apps' instead of 'apps/gateway')\n", pathPattern)
	} else {
		filePct, funcPct := 0.0, 0.0
		if total.files > 0 {
			filePct = float64(pathFiles) / float64(total.files) * 100
		}
		if total.functions > 0 {
			funcPct = float64(pathFuncs) / float64(total.functions) * 100
		}
		output += fmt.Sprintf("\n_This path represents %.1f%% of files and %.1f%% of functions_\n", filePct, funcPct)
		output += s.formatSampleFiles(pathPattern)
	}
	return output
}

func (s *indexStatusState) formatSampleFiles(pathPattern string) string {
	sampleFiles := s.runQuery("sample files", fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :limit 10`, pathPattern))
	if sampleFiles == nil || len(sampleFiles.Rows) == 0 {
		return ""
	}
	output := "\n### Sample indexed files:\n"
	for i, row := range sampleFiles.Rows {
		if i >= 5 {
			output += fmt.Sprintf("_... and %d more_\n", len(sampleFiles.Rows)-5)
			break
		}
		output += fmt.Sprintf("- `%s`\n", row[0])
	}
	return output
}

func (s *indexStatusState) formatOverallBreakdown() string {
	output := ""
	langResult := s.runQuery("languages", `?[lang, count(f)] := *cie_file { id: f, language: lang } :order -count(f) :limit 10`)
	if langResult != nil && len(langResult.Rows) > 0 {
		output += "\n### By Language:\n"
		for _, row := range langResult.Rows {
			output += fmt.Sprintf("- %s: %v files\n", row[0], row[1])
		}
	}
	filesResult := s.runQuery("files for dirs", `?[path] := *cie_file { path } :limit 500`)
	if filesResult != nil && len(filesResult.Rows) > 0 {
		dirs := make(map[string]int)
		for _, row := range filesResult.Rows {
			if fp, ok := row[0].(string); ok {
				dirs[ExtractTopDir(fp)]++
			}
		}
		output += "\n### Top Directories:\n"
		for dir, count := range dirs {
			output += fmt.Sprintf("- `%s/`: %d files\n", dir, count)
		}
	}
	return output
}

func (s *indexStatusState) formatErrors() string {
	if len(s.errors) == 0 {
		return ""
	}
	output := "\n---\n### Query Errors\n"
	for _, e := range s.errors {
		output += fmt.Sprintf("- %s\n", e)
	}
	output += "\n_Some queries failed. The Edge Cache may be unavailable or the project may not be fully indexed._\n"
	output += fmt.Sprintf("\n### Troubleshooting:\n1. Check Edge Cache is running: `curl %s/health`\n2. Re-run indexing: `cie index --force-full-reindex`\n", s.client.BaseURL)
	return output
}
