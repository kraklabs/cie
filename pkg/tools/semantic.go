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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// SemanticSearchArgs holds arguments for semantic search.
type SemanticSearchArgs struct {
	Query            string
	Limit            int
	Role             string
	PathPattern      string
	ExcludePaths     string  // Optional regex to exclude additional paths (e.g., "metrics|dlq|telemetry")
	ExcludeAnonymous bool    // Exclude anonymous/arrow functions (default: true when not specified)
	MinSimilarity    float64 // Minimum similarity threshold (0.0-1.0, e.g., 0.5 = 50%)
	EmbeddingURL     string
	EmbeddingModel   string
}

// Compiled regex patterns for role-based file filtering (Go regexp syntax).
var (
	testFilePattern = regexp.MustCompile(
		`(?i)(_test\.go|test\.ts|test\.tsx|test\.js|\.test\.|_test\.py|tests/|__tests__/)`)
	generatedFilePattern = regexp.MustCompile(
		`(?i)(\.pb\.go|_generated\.go|\.gen\.go|_gen\.go|\.generated\.|/generated/)`)
	// defaultNoisePattern matches files that are noise in virtually ALL search cases.
	// Conservative list: tests, mocks, fixtures, examples, vendor dependencies.
	// Does NOT include metrics/dlq/telemetry as those may be relevant in many searches.
	defaultNoisePattern = regexp.MustCompile(
		`(?i)(/mock[s]?[./]|_mock\.go|mock_|/fixture[s]?[./]|/example[s]?[./]|` +
			`/vendor/|/node_modules/)`)
	// noiseTermsPattern matches query terms that indicate the user WANTS noise files
	noiseTermsPattern = regexp.MustCompile(
		`(?i)\b(mock[s]?|fixture[s]?|example[s]?|vendor)\b`)
	// anonymousFunctionPattern matches anonymous/generated function names that pollute search results
	// Matches: $anon_123, $arrow_456, $lambda_789, anonymous, <anonymous>
	anonymousFunctionPattern = regexp.MustCompile(`(?i)(^\$anon_\d+$|^\$arrow_\d+$|^\$lambda_\d+$|^anonymous$|^<anonymous>$)`)
)

// SemanticSearch performs semantic search using embeddings
func SemanticSearch(ctx context.Context, client Querier, args SemanticSearchArgs) (*ToolResult, error) {
	args = normalizeSemanticArgs(args)
	if args.Query == "" {
		return NewError("Error: 'query' is required"), nil
	}

	// Generate embedding
	embedding, err := generateEmbedding(ctx, args.EmbeddingURL, args.EmbeddingModel, args.Query)
	if err != nil {
		return semanticSearchFallback(ctx, client, args.Query, args.Limit, args.Role, args.PathPattern, args.ExcludePaths, fmt.Sprintf("embedding generation failed: %v", err))
	}

	// Execute HNSW query
	result, err := executeHNSWQuery(ctx, client, embedding, args)
	if err != nil {
		return semanticSearchFallback(ctx, client, args.Query, args.Limit, args.Role, args.PathPattern, args.ExcludePaths, fmt.Sprintf("HNSW query failed: %v", err))
	}
	if len(result.Rows) == 0 {
		return semanticSearchFallback(ctx, client, args.Query, args.Limit, args.Role, args.PathPattern, args.ExcludePaths, "no vectors found in HNSW index (embeddings may not be generated)")
	}

	// Post-filter results
	result.Rows = postFilterByPath(result.Rows, args.PathPattern, args.Role, args.Query, args.ExcludePaths, true)
	if len(result.Rows) == 0 {
		reason := "no results matching filters in semantic search results"
		if args.PathPattern != "" {
			reason = fmt.Sprintf("no results matching path '%s' in semantic search results", args.PathPattern)
		}
		return semanticSearchFallback(ctx, client, args.Query, args.Limit, args.Role, args.PathPattern, args.ExcludePaths, reason)
	}

	// Apply min_similarity filter
	result.Rows = filterByMinSimilarity(result.Rows, args.MinSimilarity)
	if len(result.Rows) == 0 {
		return NewResult(fmt.Sprintf("No results with similarity >= %.0f%% for '%s'", args.MinSimilarity*100, args.Query)), nil
	}

	// Limit and format results
	if len(result.Rows) > args.Limit {
		result.Rows = result.Rows[:args.Limit]
	}
	return NewResult(formatSemanticResults(result.Rows, args)), nil
}

func normalizeSemanticArgs(args SemanticSearchArgs) SemanticSearchArgs {
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Role == "" {
		args.Role = "source"
	}
	if args.Limit > 50 {
		args.Limit = 50
	}
	return args
}

func executeHNSWQuery(ctx context.Context, client Querier, embedding []float64, args SemanticSearchArgs) (*QueryResult, error) {
	vecLiteral := formatEmbeddingForCozoDB(embedding)
	queryK, ef := buildHNSWParams(args.Limit, args.Role, args.PathPattern)
	script := fmt.Sprintf(`?[name, file_path, signature, start_line, distance, code_text] :=
		~cie_function_embedding:embedding_idx { function_id | query: q, k: %d, ef: %d, bind_distance: distance },
		q = %s,
		*cie_function { id: function_id, name, file_path, signature, start_line },
		*cie_function_code { function_id: function_id, code_text }
		:order distance
		:limit %d`, queryK, ef, vecLiteral, queryK)
	return client.Query(ctx, script)
}

func filterByMinSimilarity(rows [][]any, minSimilarity float64) [][]any {
	if minSimilarity <= 0 {
		return rows
	}
	filtered := make([][]any, 0, len(rows))
	for _, row := range rows {
		if len(row) < 5 {
			continue
		}
		if d, ok := row[4].(float64); ok {
			if 1.0-d >= minSimilarity {
				filtered = append(filtered, row)
			}
		}
	}
	return filtered
}

func formatSemanticResults(rows [][]any, args SemanticSearchArgs) string {
	var sb strings.Builder
	if args.PathPattern != "" {
		fmt.Fprintf(&sb, "🔍 **Semantic search** for '%s' in '%s' (using embeddings):\n\n", args.Query, args.PathPattern)
	} else {
		fmt.Fprintf(&sb, "🔍 **Semantic search** for '%s' (using embeddings):\n\n", args.Query)
	}

	for i, row := range rows {
		formatSemanticResultRow(&sb, i+1, row)
	}
	return sb.String()
}

func formatSemanticResultRow(sb *strings.Builder, num int, row []any) {
	name := AnyToString(row[0])
	filePath := AnyToString(row[1])
	signature := AnyToString(row[2])
	startLine := AnyToString(row[3])

	similarity := 1.0
	if d, ok := row[4].(float64); ok {
		similarity = 1.0 - d
	}

	confidenceIcon := getConfidenceIcon(similarity)
	fmt.Fprintf(sb, "%d. %s **%s** (%.1f%% match)\n", num, confidenceIcon, name, similarity*100)
	fmt.Fprintf(sb, "   📁 %s:%s\n", filePath, startLine)
	if len(signature) < 100 && signature != "" {
		fmt.Fprintf(sb, "   📝 `%s`\n", signature)
	}

	if len(row) > 5 {
		codeText := AnyToString(row[5])
		snippet := extractCodeSnippet(codeText, 3)
		if snippet != "" {
			sb.WriteString("   ```\n")
			for _, line := range strings.Split(snippet, "\n") {
				sb.WriteString("   " + line + "\n")
			}
			sb.WriteString("   ```\n")
		}
	}
	sb.WriteString("\n")
}

func getConfidenceIcon(similarity float64) string {
	if similarity >= 0.75 {
		return "🟢"
	}
	if similarity >= 0.50 {
		return "🟡"
	}
	return "🔴"
}

// semanticSearchFallback uses text search when semantic search is unavailable
func semanticSearchFallback(ctx context.Context, client Querier, query string, limit int, role, pathPattern, excludePaths, reason string) (*ToolResult, error) {
	// Extract key terms and use regex search
	terms := ExtractKeyTerms(query)
	if len(terms) == 0 {
		return NewError("No searchable terms found in query"), nil
	}

	pattern := "(?i)(" + terms[0]
	for i := 1; i < len(terms); i++ {
		pattern += "|" + terms[i]
	}
	pattern += ")"

	// Build file pattern based on role
	// Note: CozoDB regex doesn't support lookahead (?!...), so we use negate() in query instead
	filePattern := pathPattern
	excludePattern := ""
	switch role {
	case "source":
		// Exclude test and generated files - pattern to negate
		excludePattern = "(_test[.]go|test[.]ts|test[.]tsx|__tests__|tests/|[.]pb[.]go|_generated[.]go)"
		// Also exclude default noise files (mocks, fixtures, vendor) unless query mentions them
		if !noiseTermsPattern.MatchString(query) {
			excludePattern += "|(/mock[s]?[./]|_mock[.]go|mock_|/fixture[s]?[./]|/example[s]?[./]|/vendor/|/node_modules/)"
		}
		// Add agent-specified exclude pattern if provided
		if excludePaths != "" {
			excludePattern += "|(" + excludePaths + ")"
		}
	case "test":
		if filePattern == "" {
			filePattern = "(_test[.]go|test[.]ts|test[.]tsx|__tests__|tests/)"
		}
	case "generated":
		if filePattern == "" {
			filePattern = "([.]pb[.]go|_generated[.]go|[.]gen[.]go|/generated/)"
		}
	}

	// Search in both name AND code for better recall
	// First try to find matches in function names
	result, err := SearchText(ctx, client, SearchTextArgs{
		Pattern:        pattern,
		SearchIn:       "all", // Search name, signature, AND code
		FilePattern:    filePattern,
		ExcludePattern: excludePattern,
		Limit:          limit,
	})
	if err != nil {
		return NewError(fmt.Sprintf("Search error: %v", err)), nil
	}

	// Add note about fallback with specific reason
	output := fmt.Sprintf("⚠️ **Text search fallback** (reason: %s)\n\n", reason)
	output += fmt.Sprintf("Searching for keywords from: '%s'\n", query)
	output += fmt.Sprintf("Pattern: `%s`\n\n", pattern)
	output += result.Text
	if result.Text == "" || ContainsStr(result.Text, "Found 0") {
		output += "\n\n**Tips to improve results:**\n"
		output += "- Use `cie_grep` for exact text patterns (fastest)\n"
		output += "- Use `cie_search_text` with `literal: true` for exact patterns\n"
		output += "- Use `cie_find_function` for specific function names\n"
		output += "- Use `cie_list_files` to explore the codebase structure\n"
	}
	output += "\n\n---\n"
	output += "💡 **To enable true semantic search:**\n"
	output += "1. Ensure Ollama is running: `ollama serve`\n"
	output += "2. Pull the embedding model: `ollama pull nomic-embed-text`\n"
	output += "3. Re-index with embeddings: `cie index`\n"

	return NewResult(output), nil
}

// preprocessQueryForCode applies model-specific preprocessing to queries.
// Different models use different formats:
//   - Qodo-Embed-1: Uses gte-Qwen2-instruct format (Instruct + Query)
//   - Nomic: "search_query: <query>" prefix for asymmetric search
//
// The embeddingModel parameter is used to detect the model type.
func preprocessQueryForCode(query, embeddingModel string) string {
	// Qodo-Embed models: use gte-Qwen2-instruct format
	// Base model uses "Instruct: <task>\nQuery: <query>" format
	if embeddingModel == "" || isQodoModel(embeddingModel) {
		return "Instruct: Given a code search query, retrieve relevant code that matches the query\nQuery: " + query
	}
	// Nomic and other models: asymmetric search prefix
	return "search_query: " + query
}

// isQodoModel checks if the model name indicates a Qodo embedding model.
func isQodoModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "qodo")
}

// generateEmbedding generates an embedding using the configured provider.
// Supports Ollama API (/api/embeddings), llama.cpp server (/embedding), and OpenAI-compatible (/v1/embeddings).
//
//nolint:gocyclo // Embedding provider detection has inherent complexity
func generateEmbedding(ctx context.Context, embeddingURL, embeddingModel, text string) ([]float64, error) {
	// Preprocess the query for better code matching
	processedText := preprocessQueryForCode(text, embeddingModel)

	// Detect API type based on URL patterns
	isLlamaCpp := strings.Contains(embeddingURL, ":8090") || embeddingModel == ""
	isOpenAI := strings.Contains(embeddingURL, "/v1") || strings.Contains(embeddingURL, ":30090")

	var endpoint string
	var body []byte

	if isOpenAI {
		// OpenAI-compatible format (TEI, vLLM, etc.)
		// If URL already ends with /v1, append /embeddings; otherwise append /v1/embeddings
		if strings.HasSuffix(embeddingURL, "/v1") {
			endpoint = embeddingURL + "/embeddings"
		} else if strings.Contains(embeddingURL, "/v1/") {
			endpoint = embeddingURL // Already complete
			if !strings.HasSuffix(endpoint, "/embeddings") {
				endpoint = strings.TrimSuffix(endpoint, "/") + "/embeddings"
			}
		} else {
			endpoint = strings.TrimSuffix(embeddingURL, "/") + "/v1/embeddings"
		}
		payload := map[string]any{
			"input": processedText,
			"model": embeddingModel,
		}
		body, _ = json.Marshal(payload)
	} else if isLlamaCpp {
		// llama.cpp server format
		endpoint = embeddingURL + "/embedding"
		payload := map[string]any{
			"content": processedText,
		}
		body, _ = json.Marshal(payload)
	} else {
		// Ollama format
		endpoint = embeddingURL + "/api/embeddings"
		payload := map[string]any{
			"model":  embeddingModel,
			"prompt": processedText,
		}
		body, _ = json.Marshal(payload)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second} // Longer timeout for local models
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Try to parse the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	// OpenAI returns: {"data": [{"embedding": [...]}]}
	// llama.cpp returns: [{"index": 0, "embedding": [[...vectors...]]}]
	// Ollama returns: {"embedding": [...vectors...]}
	if isOpenAI {
		var result struct {
			Data []struct {
				Embedding []float64 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("parse OpenAI response: %w", err)
		}
		if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding returned from OpenAI-compatible API")
		}
		return result.Data[0].Embedding, nil
	}

	if isLlamaCpp {
		var results []struct {
			Index     int         `json:"index"`
			Embedding [][]float64 `json:"embedding"`
		}
		if err := json.Unmarshal(respBody, &results); err != nil {
			return nil, fmt.Errorf("parse llama.cpp response: %w", err)
		}
		if len(results) == 0 || len(results[0].Embedding) == 0 || len(results[0].Embedding[0]) == 0 {
			return nil, fmt.Errorf("empty embedding returned")
		}
		return results[0].Embedding[0], nil
	}

	// Ollama format
	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse ollama embedding response: %w", err)
	}

	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return result.Embedding, nil
}

// formatEmbeddingForCozoDB formats a float64 slice as a CozoDB vec() function call
// CozoDB HNSW queries expect: query: q }, q = vec([0.1, 0.2, ...])
func formatEmbeddingForCozoDB(embedding []float64) string {
	var buf bytes.Buffer
	buf.WriteString("vec([")
	for i, v := range embedding {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(fmt.Sprintf("%.6f", v))
	}
	buf.WriteString("])")
	return buf.String()
}

// buildHNSWParams determines the HNSW query parameters based on filtering requirements.
// We always retrieve extra candidates and post-filter in Go for reliability.
// HNSW in-query filters have parsing issues with complex regex patterns.
// Returns: queryK (number of candidates), ef (exploration factor)
func buildHNSWParams(limit int, role, pathPattern string) (queryK, ef int) {
	// Constants
	const semanticSearchPathFilterK = 2000
	const semanticSearchMinEf = 50

	// Determine if we need post-filtering (which requires more candidates)
	// All roles except "any" need filtering since they exclude test/generated files
	needsFiltering := pathPattern != "" || role != "any"

	if needsFiltering {
		// Get many candidates for post-filtering
		queryK = semanticSearchPathFilterK
		ef = semanticSearchPathFilterK
	} else {
		// role="any" - no filtering needed, use normal limit
		queryK = limit
		ef = semanticSearchMinEf
		if queryK > ef {
			ef = queryK
		}
	}
	return
}

// postFilterByPath filters HNSW results by path pattern, role, and excludes noise.
// This is used when path_pattern is specified since HNSW path filters are too restrictive.
// Parameters:
//   - query: used to determine if default noise filtering should be applied
//   - excludePaths: additional regex pattern to exclude (agent-specified, case-by-case)
//   - excludeAnonymous: if true, filters out anonymous/arrow functions
func postFilterByPath(rows [][]any, pathPattern, role, query, excludePaths string, excludeAnonymous bool) [][]any {
	var pathRegex *regexp.Regexp
	if pathPattern != "" {
		// Build case-insensitive regex for path matching
		pathRegex = regexp.MustCompile("(?i)" + pathPattern)
	}

	// Compile additional exclude pattern if provided
	var excludeRegex *regexp.Regexp
	if excludePaths != "" {
		excludeRegex = regexp.MustCompile("(?i)" + excludePaths)
	}

	// Determine if we should filter default noise files (mocks, fixtures, vendor, etc.)
	// Only filter if the query doesn't explicitly mention those terms
	filterDefaultNoise := !noiseTermsPattern.MatchString(query)

	filtered := make([][]any, 0, len(rows))
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		name := AnyToString(row[0])
		filePath := AnyToString(row[1])

		// Filter out anonymous functions (noise in search results) if enabled
		if excludeAnonymous && anonymousFunctionPattern.MatchString(name) {
			continue
		}

		// Apply path filter
		if pathRegex != nil && !pathRegex.MatchString(filePath) {
			continue
		}

		// Apply role filter
		if !MatchesRoleFilter(filePath, role) {
			continue
		}

		// Apply default noise filter for source role when query doesn't mention noise terms
		if filterDefaultNoise && (role == "source" || role == "") && defaultNoisePattern.MatchString(filePath) {
			continue
		}

		// Apply agent-specified exclude pattern (always applied if provided)
		if excludeRegex != nil && excludeRegex.MatchString(filePath) {
			continue
		}

		filtered = append(filtered, row)
	}
	return filtered
}

// MatchesRoleFilter checks if a file path matches the given role filter.
// Returns true if the file should be included in results.
func MatchesRoleFilter(filePath, role string) bool {
	switch role {
	case "source", "", "router", "handler", "entry_point":
		// Exclude test and generated files for implementation-focused roles
		return !testFilePattern.MatchString(filePath) && !generatedFilePattern.MatchString(filePath)
	case "test":
		return testFilePattern.MatchString(filePath)
	case "generated":
		return generatedFilePattern.MatchString(filePath)
	case "any":
		return true
	default:
		// Unknown roles default to excluding tests (safer default for implementation search)
		return !testFilePattern.MatchString(filePath) && !generatedFilePattern.MatchString(filePath)
	}
}

// roleFiltersForHNSW returns filter expression for HNSW queries
// HNSW filter parameter uses different syntax: ! for negation, && for and, || for or
func RoleFiltersForHNSW(role string) string {
	testPattern := `"(?i)(_test[.]go|test[.]ts|test[.]tsx|test[.]js|[.]test[.]|_test[.]py|tests/|__tests__/)"`
	generatedPattern := `"(?i)([.]pb[.]go|_generated[.]go|[.]gen[.]go|_gen[.]go|[.]generated[.]|/generated/)"`
	entryPointPattern := `"(?i)^main$"`
	routerNamePattern := `"(?i)(RegisterRoutes|SetupRoutes|InitRoutes|NewRouter|Routes|SetupRouter|SetupHandlers|RegisterAPI)"`
	handlerNamePattern := `"(?i)(Handler|Controller|handle[A-Z])"`

	switch role {
	case "source":
		return fmt.Sprintf(`!regex_matches(file_path, %s) && !regex_matches(file_path, %s)`, testPattern, generatedPattern)
	case "test":
		return fmt.Sprintf(`regex_matches(file_path, %s)`, testPattern)
	case "generated":
		return fmt.Sprintf(`regex_matches(file_path, %s)`, generatedPattern)
	case "entry_point":
		return fmt.Sprintf(`regex_matches(name, %s) && !regex_matches(file_path, %s)`, entryPointPattern, testPattern)
	case "router":
		return fmt.Sprintf(`regex_matches(name, %s) && !regex_matches(file_path, %s)`, routerNamePattern, testPattern)
	case "handler":
		return fmt.Sprintf(`regex_matches(name, %s) && !regex_matches(file_path, %s)`, handlerNamePattern, testPattern)
	default: // "any"
		return ""
	}
}

// roleFilters returns CozoScript filter conditions for a given role (for normal queries)
func RoleFilters(role string) []string {
	// Note: Use [.] for literal dot in regex - CozoDB interprets \. differently
	testPattern := `"(?i)(_test[.]go|test[.]ts|test[.]tsx|test[.]js|[.]test[.]|_test[.]py|tests/|__tests__/)"`
	generatedPattern := `"(?i)([.]pb[.]go|_generated[.]go|[.]gen[.]go|_gen[.]go|[.]generated[.]|/generated/)"`
	entryPointPattern := `"(?i)^main$"`

	// Router detection: function names OR code patterns for Go frameworks (Gin, Echo, Fiber, Chi, Mux)
	routerNamePattern := `"(?i)(RegisterRoutes|SetupRoutes|InitRoutes|NewRouter|Routes|SetupRouter|SetupHandlers|RegisterAPI)"`
	// Handler detection: function names OR signature patterns for Go frameworks
	handlerNamePattern := `"(?i)(Handler|Controller|handle[A-Z])"`

	switch role {
	case "source":
		return []string{
			fmt.Sprintf(`negate(regex_matches(file_path, %s))`, testPattern),
			fmt.Sprintf(`negate(regex_matches(file_path, %s))`, generatedPattern),
		}
	case "test":
		return []string{fmt.Sprintf(`regex_matches(file_path, %s)`, testPattern)}
	case "generated":
		return []string{fmt.Sprintf(`regex_matches(file_path, %s)`, generatedPattern)}
	case "entry_point":
		return []string{
			fmt.Sprintf(`regex_matches(name, %s)`, entryPointPattern),
			fmt.Sprintf(`negate(regex_matches(file_path, %s))`, testPattern),
		}
	case "router":
		// Match by name OR by code content (Gin/Echo/Fiber/Chi route patterns)
		// Note: Use [.] for literal dots, avoid \\s which may not work in CozoDB
		return []string{
			fmt.Sprintf(`(regex_matches(name, %s) or regex_matches(code_text, "(?i)([.](GET|POST|PUT|DELETE|PATCH|Group|Handle|Use)[(]|RouterGroup|gin[.]Engine|echo[.]Echo|fiber[.]App|chi[.]Router|mux[.]Router)"))`, routerNamePattern),
			fmt.Sprintf(`negate(regex_matches(file_path, %s))`, testPattern),
		}
	case "handler":
		// Match by name OR by signature (receives HTTP context types)
		return []string{
			fmt.Sprintf(`(regex_matches(name, %s) or regex_matches(signature, "(?i)(gin[.]Context|echo[.]Context|fiber[.]Ctx|http[.]ResponseWriter|[*]http[.]Request)"))`, handlerNamePattern),
			fmt.Sprintf(`negate(regex_matches(file_path, %s))`, testPattern),
		}
	default: // "any"
		return nil
	}
}

// extractCodeSnippet extracts the first N meaningful lines from code text.
// Skips empty lines and trims whitespace for a clean preview.
func extractCodeSnippet(code string, maxLines int) string {
	if code == "" {
		return ""
	}

	lines := strings.Split(code, "\n")
	var result []string
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comment-only lines for better preview
		if trimmed == "" {
			continue
		}
		// Include the line (preserve original indentation but limit length)
		if len(line) > 80 {
			line = line[:77] + "..."
		}
		result = append(result, line)
		count++
		if count >= maxLines {
			break
		}
	}

	if len(result) == 0 {
		return ""
	}

	return strings.Join(result, "\n")
}
