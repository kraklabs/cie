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
	"sort"
	"strings"

	"github.com/kraklabs/cie/pkg/llm"
)

// AnalyzeArgs holds arguments for the analyze tool.
type AnalyzeArgs struct {
	Question    string
	PathPattern string
	Role        string // "source" (default, excludes tests), "test", "any"
}

// relevantFunction holds a function found via semantic search with its code
type relevantFunction struct {
	Name       string
	FilePath   string
	StartLine  string
	Signature  string
	Code       string
	Similarity float64
	StubInfo   *StubDetection // nil if not a stub
}

// StubDetection holds information about whether a function appears to be a stub or unimplemented.
//
// Stub detection helps filter out placeholder functions from analysis results,
// improving the quality of semantic search and architectural analysis.
//
// The detection heuristics check for common stub patterns across multiple languages:
//   - Go: "not implemented" errors, panics, ErrNotImplemented returns
//   - Python: NotImplementedError exceptions
//   - Rust: todo!() and unimplemented!() macros
//   - Java: UnsupportedOperationException
//   - Generic: Empty functions, TODO comments, minimal code lines
type StubDetection struct {
	// IsStub indicates whether the function is detected as a stub or placeholder.
	IsStub bool

	// Reason provides a human-readable explanation of why the function was classified as a stub.
	// Example: "raises NotImplementedError" or "function has only 2 code lines and 1 comment lines"
	Reason string

	// Patterns lists the specific stub patterns that matched in the function code.
	// Example: ["returns 'not implemented' error", "minimal code (< 5 lines)"]
	Patterns []string
}

// stubPattern defines a pattern for detecting stub functions.
type stubPattern struct {
	pattern *regexp.Regexp
	name    string
	langs   []string // empty means all languages
}

// appliesToLang checks if the pattern applies to the given language.
func (sp *stubPattern) appliesToLang(lang string) bool {
	if len(sp.langs) == 0 {
		return true
	}
	for _, l := range sp.langs {
		if l == lang {
			return true
		}
	}
	return false
}

var strongStubPatterns = []stubPattern{
	// Go
	{regexp.MustCompile(`(?i)return\s+(fmt\.Errorf|errors\.New)\s*\(\s*["'].*not\s+implemented`), "returns 'not implemented' error", []string{"go"}},
	{regexp.MustCompile(`(?i)panic\s*\(\s*["'].*not\s+implemented`), "panics with 'not implemented'", []string{"go"}},
	{regexp.MustCompile(`(?i)return\s+ErrNotImplemented`), "returns ErrNotImplemented", []string{"go"}},
	// Python
	{regexp.MustCompile(`(?i)raise\s+NotImplementedError`), "raises NotImplementedError", []string{"python"}},
	// Rust
	{regexp.MustCompile(`(?i)\btodo!\s*\(`), "uses todo!()", []string{"rust"}},
	{regexp.MustCompile(`(?i)\bunimplemented!\s*\(`), "uses unimplemented!()", []string{"rust"}},
	// Java
	{regexp.MustCompile(`(?i)throw\s+new\s+UnsupportedOperationException`), "throws UnsupportedOperationException", []string{"java"}},
	// Generic
	{regexp.MustCompile(`(?i)throw\s+new\s+Error\s*\(\s*["'].*not\s+implemented`), "throws 'not implemented' error", nil},
	{regexp.MustCompile(`(?i)["']not\s+implemented["']`), "contains 'not implemented' string", nil},
}

var weakStubPatterns = []stubPattern{
	// Go
	{regexp.MustCompile(`^\s*return\s+nil\s*$`), "only returns nil", []string{"go"}},
	{regexp.MustCompile(`^\s*return\s*$`), "empty return", []string{"go"}},
	// Python
	{regexp.MustCompile(`^\s*pass\s*$`), "only contains 'pass'", []string{"python"}},
	{regexp.MustCompile(`^\s*\.\.\.\s*$`), "only contains '...' (ellipsis)", []string{"python"}},
	{regexp.MustCompile(`^\s*return\s+None\s*$`), "only returns None", []string{"python"}},
	// JS/TS
	{regexp.MustCompile(`^\s*return\s*;\s*$`), "empty return", []string{"typescript", "javascript"}},
	{regexp.MustCompile(`^\s*return\s+undefined\s*;?\s*$`), "returns undefined", []string{"typescript", "javascript"}},
	{regexp.MustCompile(`^\s*return\s+null\s*;?\s*$`), "returns null", []string{"typescript", "javascript"}},
}

// detectStub analyzes function code to determine if it's likely a stub or not implemented.
func detectStub(code, filePath string) *StubDetection {
	if code == "" {
		return nil
	}
	lang := detectLanguage(filePath)

	// Check strong indicators first
	if matches := matchStubPatterns(code, lang, strongStubPatterns, false); len(matches) > 0 {
		return &StubDetection{
			IsStub:   true,
			Reason:   fmt.Sprintf("Function %s", strings.Join(matches, ", ")),
			Patterns: matches,
		}
	}

	// Check weak indicators only for short functions
	codeLines := countCodeLines(code)
	if codeLines <= 3 {
		if matches := matchStubPatterns(code, lang, weakStubPatterns, true); len(matches) > 0 {
			return &StubDetection{
				IsStub:   true,
				Reason:   fmt.Sprintf("Very short function (%d lines) that %s", codeLines, strings.Join(matches, ", ")),
				Patterns: matches,
			}
		}
	}

	return nil
}

// matchStubPatterns checks code against patterns, returning matched pattern names.
func matchStubPatterns(code, lang string, patterns []stubPattern, perLine bool) []string {
	var matches []string
	for i := range patterns {
		sp := &patterns[i]
		if !sp.appliesToLang(lang) {
			continue
		}
		if perLine {
			if matchPatternPerLine(code, sp.pattern) {
				matches = append(matches, sp.name)
			}
		} else if sp.pattern.MatchString(code) {
			matches = append(matches, sp.name)
		}
	}
	return matches
}

// matchPatternPerLine checks if pattern matches any line in code.
func matchPatternPerLine(code string, pattern *regexp.Regexp) bool {
	for _, line := range strings.Split(code, "\n") {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// countCodeLines counts non-empty, non-comment lines in code
func countCodeLines(code string) int {
	lines := strings.Split(code, "\n")
	count := 0
	inBlockComment := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Handle block comments
		if strings.Contains(trimmed, "/*") {
			inBlockComment = true
		}
		if strings.Contains(trimmed, "*/") {
			inBlockComment = false
			continue
		}
		if inBlockComment {
			continue
		}

		// Skip single-line comments
		if strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Skip function signature lines (rough heuristic)
		if strings.HasPrefix(trimmed, "func ") ||
			strings.HasPrefix(trimmed, "def ") ||
			strings.HasPrefix(trimmed, "function ") ||
			strings.HasPrefix(trimmed, "async ") ||
			trimmed == "{" || trimmed == "}" ||
			trimmed == "(" || trimmed == ")" {
			continue
		}

		count++
	}

	return count
}

// analyzeState holds intermediate state during analysis.
type analyzeState struct {
	sections       []string
	errors         []string
	localizedFuncs []relevantFunction
	globalFuncs    []relevantFunction
	searchFailed   bool
	args           AnalyzeArgs
}

// Analyze uses semantic search + LLM to answer architectural questions about the codebase
func Analyze(ctx context.Context, client *CIEClient, args AnalyzeArgs) (*ToolResult, error) {
	if args.Question == "" {
		return NewError("Error: 'question' is required"), nil
	}

	// Default to source (exclude tests) for architectural analysis
	if args.Role == "" {
		args.Role = "source"
	}

	state := &analyzeState{args: args}

	// Step 1: Get index stats
	state.addIndexStats(ctx, client)

	// Step 2: Perform semantic searches
	state.performSemanticSearch(ctx, client)

	// Step 3: Format semantic results
	state.formatSemanticResults()

	// Step 4: Fallback to keyword search if semantic failed
	state.performKeywordFallback(ctx, client)

	// Step 5: Run contextual keyword queries
	state.runContextualQueries(ctx, client)

	// Step 6: Build final output
	return state.buildOutput(ctx, client)
}

// addIndexStats adds basic index statistics to sections.
func (s *analyzeState) addIndexStats(ctx context.Context, client *CIEClient) {
	fileCount := countWithFallback(ctx, client, "file count",
		`?[count(f)] := *cie_file { id: f }`,
		`?[id] := *cie_file { id } :limit 10000`)
	funcCount := countWithFallback(ctx, client, "function count",
		`?[count(f)] := *cie_function { id: f }`,
		`?[id] := *cie_function { id } :limit 10000`)

	stats := "## Index Status\n"
	stats += fmt.Sprintf("- Files indexed: %d\n", fileCount)
	stats += fmt.Sprintf("- Functions indexed: %d\n", funcCount)
	s.sections = append(s.sections, stats)
}

// performSemanticSearch executes both localized and global semantic searches.
func (s *analyzeState) performSemanticSearch(ctx context.Context, client *CIEClient) {
	if client.EmbeddingURL == "" || client.EmbeddingModel == "" {
		s.errors = append(s.errors, fmt.Sprintf("embedding not configured (url=%q, model=%q) - using keyword fallback",
			client.EmbeddingURL, client.EmbeddingModel))
		s.searchFailed = true
		return
	}

	// Localized search (if path specified)
	if s.args.PathPattern != "" {
		funcs, err := findRelevantFunctionsLocalized(ctx, client, s.args.Question, s.args.PathPattern, s.args.Role, 10)
		if err != nil {
			s.errors = append(s.errors, fmt.Sprintf("localized semantic search: %v", err))
		} else if len(funcs) > 0 {
			s.localizedFuncs = funcs
		}
	}

	// Global search (reduced limit if localized succeeded)
	globalLimit := 10
	if len(s.localizedFuncs) > 0 {
		globalLimit = 5
	}
	funcs, err := findRelevantFunctions(ctx, client, s.args.Question, "", s.args.Role, globalLimit)
	if err != nil {
		s.errors = append(s.errors, fmt.Sprintf("global semantic search: %v", err))
	} else {
		s.globalFuncs = funcs
	}

	// Mark as failed if both searches returned nothing
	if len(s.localizedFuncs) == 0 && len(s.globalFuncs) == 0 {
		s.searchFailed = true
	}
}

// formatSemanticResults formats the semantic search results into sections.
func (s *analyzeState) formatSemanticResults() {
	if len(s.localizedFuncs) > 0 {
		section := fmt.Sprintf("## Semantically Relevant (in %s)\n\n", s.args.PathPattern)
		section += formatFunctionList(s.localizedFuncs)
		s.sections = append(s.sections, section)
	}

	if len(s.globalFuncs) > 0 {
		section := "## Semantically Relevant (global)\n\n"
		section += formatFunctionList(s.globalFuncs)
		s.sections = append(s.sections, section)
	}
}

// formatFunctionList formats a list of relevant functions.
func formatFunctionList(funcs []relevantFunction) string {
	var result string
	for i, f := range funcs {
		stubMarker := ""
		if f.StubInfo != nil && f.StubInfo.IsStub {
			stubMarker = " [⚠️ STUB]"
		}
		result += fmt.Sprintf("%d. **%s**%s (%.0f%% similar)\n", i+1, f.Name, stubMarker, f.Similarity*100)
		result += fmt.Sprintf("   - File: `%s:%s`\n", f.FilePath, f.StartLine)
		if f.Signature != "" && len(f.Signature) < 120 {
			result += fmt.Sprintf("   - Signature: `%s`\n", f.Signature)
		}
		if f.StubInfo != nil && f.StubInfo.IsStub {
			result += fmt.Sprintf("   - ⚠️ **Not implemented:** %s\n", f.StubInfo.Reason)
		}
		result += "\n"
	}
	return result
}

// performKeywordFallback runs keyword search when semantic search fails.
func (s *analyzeState) performKeywordFallback(ctx context.Context, client *CIEClient) {
	if !s.searchFailed {
		return
	}

	terms := ExtractKeyTerms(s.args.Question)
	if len(terms) == 0 {
		return
	}

	// Build pattern from extracted terms
	pattern := buildKeywordPattern(terms)

	// Search in function names
	s.runKeywordNameSearch(ctx, client, pattern)

	// Search in function code
	s.runKeywordCodeSearch(ctx, client, pattern)
}

// buildKeywordPattern creates a regex pattern from key terms.
func buildKeywordPattern(terms []string) string {
	pattern := "(?i)(" + terms[0]
	for i := 1; i < len(terms) && i < 5; i++ {
		pattern += "|" + terms[i]
	}
	pattern += ")"
	return pattern
}

// runKeywordNameSearch searches function names for keywords.
func (s *analyzeState) runKeywordNameSearch(ctx context.Context, client *CIEClient, pattern string) {
	query := fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, regex_matches(name, %q) :limit 30`, pattern)
	if s.args.PathPattern != "" {
		query = fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, regex_matches(name, %q), regex_matches(file_path, %q) :limit 30`, pattern, s.args.PathPattern)
	}
	if result := s.runQuery(ctx, client, "keyword name search", query); result != nil && len(result.Rows) > 0 {
		s.sections = append(s.sections, "## Functions Matching Keywords (name)\n"+FormatRows(result.Rows))
	}
}

// runKeywordCodeSearch searches function code for keywords.
func (s *analyzeState) runKeywordCodeSearch(ctx context.Context, client *CIEClient, pattern string) {
	query := fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { id, name, file_path, start_line }, *cie_function_code { function_id: id, code_text }, regex_matches(code_text, %q) :limit 30`, pattern)
	if s.args.PathPattern != "" {
		query = fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { id, name, file_path, start_line }, *cie_function_code { function_id: id, code_text }, regex_matches(code_text, %q), regex_matches(file_path, %q) :limit 30`, pattern, s.args.PathPattern)
	}
	if result := s.runQuery(ctx, client, "keyword code search", query); result != nil && len(result.Rows) > 0 {
		s.sections = append(s.sections, "## Functions Matching Keywords (code)\n"+FormatRows(result.Rows))
	}
}

// runContextualQueries runs keyword queries based on question context.
func (s *analyzeState) runContextualQueries(ctx context.Context, client *CIEClient) {
	questionLower := Query2Lower(s.args.Question)
	testFilter := s.getTestExcludeFilter()

	// Entry points
	if ContainsAny(questionLower, []string{"entry", "main", "start", "begin", "bootstrap", "init"}) {
		s.runEntryPointQuery(ctx, client, testFilter)
	}

	// Routes/endpoints
	if ContainsAny(questionLower, []string{"route", "endpoint", "http", "api", "rest", "url", "path"}) {
		s.runRouteQuery(ctx, client, testFilter)
	}

	// Architecture/structure
	if ContainsAny(questionLower, []string{"architect", "structure", "organiz", "layout", "folder", "director"}) {
		s.runArchitectureQuery(ctx, client)
	}
}

// getTestExcludeFilter returns the test file exclusion filter.
func (s *analyzeState) getTestExcludeFilter() string {
	if s.args.Role == "source" {
		return `, negate(regex_matches(file_path, "(?i)(_test[.]go|test[.]ts|test[.]js|_test[.]py|/tests/|/__tests__/)"))`
	}
	return ""
}

// runEntryPointQuery searches for main/entry point functions.
func (s *analyzeState) runEntryPointQuery(ctx context.Context, client *CIEClient, testFilter string) {
	query := fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, name == "main"%s :limit 20`, testFilter)
	if s.args.PathPattern != "" {
		query = fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { name, file_path, start_line }, name == "main", regex_matches(file_path, %q)%s :limit 20`, s.args.PathPattern, testFilter)
	}
	if result := s.runQuery(ctx, client, "main functions", query); result != nil && len(result.Rows) > 0 {
		s.sections = append(s.sections, "## Main Functions (Entry Points)\n"+FormatRows(result.Rows))
	}
}

// runRouteQuery searches for HTTP route definitions.
func (s *analyzeState) runRouteQuery(ctx context.Context, client *CIEClient, testFilter string) {
	query := fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { id, name, file_path, start_line }, *cie_function_code { function_id: id, code_text }, regex_matches(code_text, "[.](GET|POST|PUT|DELETE|PATCH|Handle)[(]")%s :limit 20`, testFilter)
	if s.args.PathPattern != "" {
		query = fmt.Sprintf(`?[name, file_path, start_line] := *cie_function { id, name, file_path, start_line }, *cie_function_code { function_id: id, code_text }, regex_matches(code_text, "[.](GET|POST|PUT|DELETE|PATCH|Handle)[(]"), regex_matches(file_path, %q)%s :limit 20`, s.args.PathPattern, testFilter)
	}
	if result := s.runQuery(ctx, client, "route functions", query); result != nil && len(result.Rows) > 0 {
		s.sections = append(s.sections, "## Functions with Route Definitions\n"+FormatRows(result.Rows))
	}
}

// runArchitectureQuery extracts directory structure.
func (s *analyzeState) runArchitectureQuery(ctx context.Context, client *CIEClient) {
	query := `?[path] := *cie_file { path } :limit 100`
	if s.args.PathPattern != "" {
		query = fmt.Sprintf(`?[path] := *cie_file { path }, regex_matches(path, %q) :limit 100`, s.args.PathPattern)
	}
	result := s.runQuery(ctx, client, "files", query)
	if result == nil || len(result.Rows) == 0 {
		return
	}

	dirs := make(map[string]int)
	for _, row := range result.Rows {
		if fp, ok := row[0].(string); ok {
			dir := ExtractDir(fp)
			dirs[dir]++
		}
	}

	dirList := "## Directory Structure\n"
	for dir, count := range dirs {
		dirList += fmt.Sprintf("- `%s/` (%d files)\n", dir, count)
	}
	s.sections = append(s.sections, dirList)
}

// runQuery executes a query with error tracking.
func (s *analyzeState) runQuery(ctx context.Context, client *CIEClient, name, query string) *QueryResult {
	result, err := client.Query(ctx, query)
	if err != nil {
		s.errors = append(s.errors, fmt.Sprintf("%s: %v", name, err))
		return nil
	}
	return result
}

// buildOutput constructs the final analysis output.
func (s *analyzeState) buildOutput(ctx context.Context, client *CIEClient) (*ToolResult, error) {
	output := fmt.Sprintf("# Analysis: %s\n\n", s.args.Question)
	if s.args.PathPattern != "" {
		output += fmt.Sprintf("_Scope: `%s`_\n\n", s.args.PathPattern)
	}
	if s.args.Role == "source" {
		output += "_Filtering: excluding test files_\n\n"
	}

	// Check if we have meaningful results
	if len(s.sections) <= 1 && len(s.globalFuncs) == 0 {
		output += "**No relevant results found.**\n\n"
		output += "### Suggestions:\n"
		output += "- Try rephrasing your question\n"
		output += "- Add a `path_pattern` to focus the search\n"
		output += "- Use `cie_semantic_search` directly for more control\n\n"
	}

	for _, section := range s.sections {
		output += section + "\n"
	}

	if len(s.errors) > 0 {
		output += "\n---\n### Query Issues\n"
		for _, e := range s.errors {
			output += fmt.Sprintf("- %s\n", e)
		}
	}

	// Merge localized + global for code context (localized first, they're more relevant)
	allRelevantFuncs := append(s.localizedFuncs, s.globalFuncs...)

	// Generate LLM narrative with code context
	if client.LLMClient != nil && (len(s.sections) > 1 || len(allRelevantFuncs) > 0) {
		// Build enriched context with actual code
		codeContext := buildCodeContext(allRelevantFuncs)
		narrative, err := generateNarrativeWithCode(ctx, client.LLMClient, s.args.Question, output, codeContext, client.LLMMaxTokens)
		if err != nil {
			output += fmt.Sprintf("\n---\n_LLM narrative generation failed: %v_\n", err)
		} else if narrative != "" {
			// Prepend narrative summary
			output = fmt.Sprintf("# Analysis: %s\n\n", s.args.Question) +
				narrative +
				"\n\n---\n\n## Raw Analysis Data\n\n" +
				strings.TrimPrefix(output, fmt.Sprintf("# Analysis: %s\n\n", s.args.Question))
		}
	} else if client.LLMClient == nil {
		output += "\n---\n_Note: LLM not configured. Run `cie init` to enable narrative generation._\n"
	}

	return NewResult(output), nil
}

// findRelevantFunctions uses semantic search to find the most relevant functions for a question
func findRelevantFunctions(ctx context.Context, client *CIEClient, question, pathPattern, role string, limit int) ([]relevantFunction, error) {
	// Generate embedding for the question
	embedding, err := generateEmbedding(ctx, client.EmbeddingURL, client.EmbeddingModel, question)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	// Build HNSW query - retrieve extra candidates for post-filtering
	vecLiteral := formatEmbeddingForCozoDB(embedding)
	queryK := 500 // Get many candidates for filtering
	ef := 500

	script := fmt.Sprintf(`?[name, file_path, signature, start_line, distance] :=
		~cie_function_embedding:embedding_idx { function_id | query: q, k: %d, ef: %d, bind_distance: distance },
		q = %s,
		*cie_function { id: function_id, name, file_path, signature, start_line }
		:order distance
		:limit %d`, queryK, ef, vecLiteral, queryK)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("HNSW query: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	// Post-filter by path and role
	result.Rows = postFilterByPath(result.Rows, pathPattern, role, question, "", true)

	// Limit results
	if len(result.Rows) > limit {
		result.Rows = result.Rows[:limit]
	}

	// Convert to relevantFunction structs and fetch code
	var funcs []relevantFunction
	for _, row := range result.Rows {
		name := AnyToString(row[0])
		filePath := AnyToString(row[1])
		signature := AnyToString(row[2])
		startLine := AnyToString(row[3])
		distance := 0.0
		if d, ok := row[4].(float64); ok {
			distance = d
		}

		f := relevantFunction{
			Name:       name,
			FilePath:   filePath,
			StartLine:  startLine,
			Signature:  signature,
			Similarity: 1.0 - distance,
		}

		// Fetch function code (truncated for context window)
		code, err := getFunctionCodeByName(ctx, client, name, filePath)
		if err == nil && code != "" {
			// Truncate very long functions
			if len(code) > 2000 {
				code = code[:2000] + "\n// ... (truncated)"
			}
			f.Code = code

			// Detect if this function is a stub
			f.StubInfo = detectStub(code, filePath)
		}

		funcs = append(funcs, f)
	}

	return funcs, nil
}

// findRelevantFunctionsLocalized does semantic search restricted to a specific path pattern.
// Uses a very high k value to ensure we capture functions from the specific path.
// Applies keyword boosting to re-rank results based on question terms in function names.
func findRelevantFunctionsLocalized(ctx context.Context, client *CIEClient, question, pathPattern, role string, limit int) ([]relevantFunction, error) {
	if pathPattern == "" {
		return nil, nil // No path pattern, nothing to localize
	}

	// Generate embedding for the question
	embedding, err := generateEmbedding(ctx, client.EmbeddingURL, client.EmbeddingModel, question)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	// Use VERY high k to ensure we get candidates from the specific path
	vecLiteral := formatEmbeddingForCozoDB(embedding)
	queryK := 5000
	ef := 5000

	script := fmt.Sprintf(`?[name, file_path, signature, start_line, distance] :=
		~cie_function_embedding:embedding_idx { function_id | query: q, k: %d, ef: %d, bind_distance: distance },
		q = %s,
		*cie_function { id: function_id, name, file_path, signature, start_line }
		:order distance
		:limit %d`, queryK, ef, vecLiteral, queryK)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("HNSW query: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	// STRICT filter by path pattern
	result.Rows = postFilterByPath(result.Rows, pathPattern, role, question, "", true)

	// Get MORE candidates than requested for re-ranking (2x limit)
	candidateLimit := limit * 2
	if len(result.Rows) > candidateLimit {
		result.Rows = result.Rows[:candidateLimit]
	}

	// Extract key terms for boosting
	keyTerms := ExtractKeyTerms(question)

	// Convert to relevantFunction structs
	var funcs []relevantFunction
	for _, row := range result.Rows {
		name := AnyToString(row[0])
		filePath := AnyToString(row[1])
		signature := AnyToString(row[2])
		startLine := AnyToString(row[3])
		distance := 0.0
		if d, ok := row[4].(float64); ok {
			distance = d
		}

		similarity := 1.0 - distance

		// KEYWORD BOOST: If function name contains question terms, boost similarity
		nameLower := strings.ToLower(name)
		for _, term := range keyTerms {
			if strings.Contains(nameLower, strings.ToLower(term)) {
				similarity += 0.15 // Boost by 15% per matching term
			}
		}
		// Cap at 1.0
		if similarity > 1.0 {
			similarity = 1.0
		}

		f := relevantFunction{
			Name:       name,
			FilePath:   filePath,
			StartLine:  startLine,
			Signature:  signature,
			Similarity: similarity,
		}

		funcs = append(funcs, f)
	}

	// RE-RANK by boosted similarity
	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].Similarity > funcs[j].Similarity
	})

	// Take top 'limit' after re-ranking
	if len(funcs) > limit {
		funcs = funcs[:limit]
	}

	// Fetch code for final results only (expensive operation)
	for i := range funcs {
		code, err := getFunctionCodeByName(ctx, client, funcs[i].Name, funcs[i].FilePath)
		if err == nil && code != "" {
			if len(code) > 2000 {
				code = code[:2000] + "\n// ... (truncated)"
			}
			funcs[i].Code = code

			// Detect if this function is a stub
			funcs[i].StubInfo = detectStub(code, funcs[i].FilePath)
		}
	}

	return funcs, nil
}

// getFunctionCodeByName retrieves the code for a specific function
func getFunctionCodeByName(ctx context.Context, client *CIEClient, name, filePath string) (string, error) {
	// Query for function code using name and file_path to be specific
	script := fmt.Sprintf(`?[code_text] :=
		*cie_function { id, name, file_path },
		*cie_function_code { function_id: id, code_text },
		name == %q, file_path == %q
		:limit 1`, name, filePath)

	result, err := client.Query(ctx, script)
	if err != nil {
		return "", err
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return "", nil
	}

	return AnyToString(result.Rows[0][0]), nil
}

// buildCodeContext creates a formatted code context string for the LLM
func buildCodeContext(funcs []relevantFunction) string {
	if len(funcs) == 0 {
		return ""
	}

	var sb strings.Builder
	var stubCount int

	// Count stubs first
	for _, f := range funcs {
		if f.StubInfo != nil && f.StubInfo.IsStub {
			stubCount++
		}
	}

	sb.WriteString("\n\n## Relevant Code\n\n")

	// Add stub warning if any detected
	if stubCount > 0 {
		sb.WriteString(fmt.Sprintf("**⚠️ WARNING: %d function(s) detected as stubs/not implemented. See [STUB] markers below.**\n\n", stubCount))
	}

	for i, f := range funcs {
		if f.Code == "" {
			continue
		}

		// Mark stubs prominently
		if f.StubInfo != nil && f.StubInfo.IsStub {
			sb.WriteString(fmt.Sprintf("### %d. %s [⚠️ STUB] (%s:%s)\n", i+1, f.Name, f.FilePath, f.StartLine))
			sb.WriteString(fmt.Sprintf("**Stub reason:** %s\n\n", f.StubInfo.Reason))
		} else {
			sb.WriteString(fmt.Sprintf("### %d. %s (%s:%s)\n", i+1, f.Name, f.FilePath, f.StartLine))
		}

		// Detect language for syntax highlighting
		lang := detectLanguage(f.FilePath)
		if lang == "unknown" {
			lang = "go" // default
		}
		sb.WriteString(fmt.Sprintf("```%s\n", lang))
		sb.WriteString(f.Code)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}

// generateNarrativeWithCode generates a narrative that incorporates code context
func generateNarrativeWithCode(ctx context.Context, provider llm.Provider, question, analysisData, codeContext string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	prompt := fmt.Sprintf(`Analyze this codebase to answer the user's question.

**User Question:** %s

**Analysis Data:**
%s
%s

**Instructions:**
- Answer the user's question directly and thoroughly (3-5 paragraphs)
- Reference specific function names and file paths from the results
- When explaining code, use ONLY the actual snippets provided in the "Relevant Code" section above
- NEVER invent, generate, or create placeholder code snippets - only quote from what is provided
- If you need to show code, copy it EXACTLY from the provided snippets
- Identify patterns, relationships, and architectural decisions
- Be specific - mention actual names, not generic descriptions
- If the question asks about a specific topic, focus your explanation on that topic

**CRITICAL - Stub Detection:**
- Functions marked with [⚠️ STUB] are NOT actually implemented - they return errors like "not implemented" or have empty bodies
- DO NOT claim these functions provide real functionality - they are placeholders
- When comparing implementations, clearly distinguish between real implementations and stubs
- If a feature appears to exist structurally but is marked as STUB, report it as "not implemented" or "placeholder only"`, question, analysisData, codeContext)

	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: "You are a senior software architect analyzing code. Provide clear, specific, and thorough explanations. Always reference actual function and file names from the provided data."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxTokens,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	return "## Summary\n\n" + resp.Message.Content, nil
}

// countWithFallback tries count aggregation first, falls back to row counting
func countWithFallback(ctx context.Context, client Querier, name, countQuery, listQuery string) int {
	result, err := client.Query(ctx, countQuery)
	if err == nil && len(result.Rows) > 0 {
		if cnt, ok := result.Rows[0][0].(float64); ok {
			return int(cnt)
		}
	}
	// Fallback
	result, err = client.Query(ctx, listQuery)
	if err == nil {
		return len(result.Rows)
	}
	return 0
}
