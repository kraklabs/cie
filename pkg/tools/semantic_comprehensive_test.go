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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGetConfidenceIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		similarity float64
		want       string
	}{
		{"high confidence 100%", 1.0, "🟢"},
		{"high confidence 75%", 0.75, "🟢"},
		{"high confidence threshold", 0.75, "🟢"},
		{"medium confidence 70%", 0.70, "🟡"},
		{"medium confidence 50%", 0.50, "🟡"},
		{"medium confidence threshold", 0.50, "🟡"},
		{"low confidence 49%", 0.49, "🔴"},
		{"low confidence 25%", 0.25, "🔴"},
		{"low confidence 0%", 0.0, "🔴"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getConfidenceIcon(tt.similarity)
			if got != tt.want {
				t.Errorf("getConfidenceIcon(%f) = %q, want %q", tt.similarity, got, tt.want)
			}
		})
	}
}

func TestExtractCodeSnippet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		code     string
		maxLines int
		want     string
	}{
		{
			name:     "empty code",
			code:     "",
			maxLines: 3,
			want:     "",
		},
		{
			name:     "single line",
			code:     "func main() {",
			maxLines: 3,
			want:     "func main() {",
		},
		{
			name:     "multiple lines with limit",
			code:     "func main() {\n\tfmt.Println(\"hello\")\n\treturn\n}\n",
			maxLines: 2,
			want:     "func main() {\n\tfmt.Println(\"hello\")",
		},
		{
			name:     "skip empty lines",
			code:     "func main() {\n\n\n\tfmt.Println(\"hello\")\n\n\treturn\n}",
			maxLines: 3,
			want:     "func main() {\n\tfmt.Println(\"hello\")\n\treturn",
		},
		{
			name:     "truncate long lines",
			code:     "func main() { " + strings.Repeat("x", 100) + " }",
			maxLines: 1,
			want:     "func main() { " + strings.Repeat("x", 63) + "...",
		},
		{
			name:     "only empty lines",
			code:     "\n\n\n",
			maxLines: 3,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractCodeSnippet(tt.code, tt.maxLines)
			if got != tt.want {
				t.Errorf("extractCodeSnippet() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeSemanticArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		args      SemanticSearchArgs
		wantLimit int
		wantRole  string
	}{
		{
			name:      "zero limit defaults to 10",
			args:      SemanticSearchArgs{Query: "test", Limit: 0},
			wantLimit: 10,
			wantRole:  "source",
		},
		{
			name:      "negative limit defaults to 10",
			args:      SemanticSearchArgs{Query: "test", Limit: -5},
			wantLimit: 10,
			wantRole:  "source",
		},
		{
			name:      "limit > 50 capped at 50",
			args:      SemanticSearchArgs{Query: "test", Limit: 100},
			wantLimit: 50,
			wantRole:  "source",
		},
		{
			name:      "empty role defaults to source",
			args:      SemanticSearchArgs{Query: "test", Limit: 10, Role: ""},
			wantLimit: 10,
			wantRole:  "source",
		},
		{
			name:      "custom role preserved",
			args:      SemanticSearchArgs{Query: "test", Limit: 10, Role: "test"},
			wantLimit: 10,
			wantRole:  "test",
		},
		{
			name:      "valid limit preserved",
			args:      SemanticSearchArgs{Query: "test", Limit: 25, Role: "any"},
			wantLimit: 25,
			wantRole:  "any",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeSemanticArgs(tt.args)
			if got.Limit != tt.wantLimit {
				t.Errorf("normalizeSemanticArgs().Limit = %d, want %d", got.Limit, tt.wantLimit)
			}
			if got.Role != tt.wantRole {
				t.Errorf("normalizeSemanticArgs().Role = %q, want %q", got.Role, tt.wantRole)
			}
		})
	}
}

func TestPreprocessQueryForCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		query          string
		embeddingModel string
		wantPrefix     string
	}{
		{
			name:           "qodo model uses instruct format",
			query:          "authentication handler",
			embeddingModel: "qodo-embed-1",
			wantPrefix:     "Instruct: Given a code search query",
		},
		{
			name:           "empty model defaults to qodo format",
			query:          "database connection",
			embeddingModel: "",
			wantPrefix:     "Instruct: Given a code search query",
		},
		{
			name:           "nomic model uses search_query prefix",
			query:          "HTTP handler",
			embeddingModel: "nomic-embed-text",
			wantPrefix:     "search_query: HTTP handler",
		},
		{
			name:           "other model uses search_query prefix",
			query:          "user service",
			embeddingModel: "text-embedding-3-small",
			wantPrefix:     "search_query: user service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := preprocessQueryForCode(tt.query, tt.embeddingModel)
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("preprocessQueryForCode() = %q, want prefix %q", got, tt.wantPrefix)
			}
		})
	}
}

func TestIsQodoModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"qodo-embed-1", true},
		{"Qodo-Embed-1", true},
		{"QODO-EMBED-1", true},
		{"nomic-embed-text", false},
		{"text-embedding-3-small", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			got := isQodoModel(tt.model)
			if got != tt.want {
				t.Errorf("isQodoModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestFilterByMinSimilarity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		rows          [][]any
		minSimilarity float64
		wantLen       int
	}{
		{
			name: "no filter with 0.0 threshold",
			rows: [][]any{
				{"func1", "file1.go", "sig1", 1, 0.2}, // 80% similarity
				{"func2", "file2.go", "sig2", 2, 0.5}, // 50% similarity
				{"func3", "file3.go", "sig3", 3, 0.8}, // 20% similarity
			},
			minSimilarity: 0.0,
			wantLen:       3,
		},
		{
			name: "filter >= 70% similarity",
			rows: [][]any{
				{"func1", "file1.go", "sig1", 1, 0.2}, // 80% similarity
				{"func2", "file2.go", "sig2", 2, 0.5}, // 50% similarity
				{"func3", "file3.go", "sig3", 3, 0.8}, // 20% similarity
			},
			minSimilarity: 0.7,
			wantLen:       1, // only func1
		},
		{
			name: "filter >= 50% similarity",
			rows: [][]any{
				{"func1", "file1.go", "sig1", 1, 0.2}, // 80% similarity
				{"func2", "file2.go", "sig2", 2, 0.5}, // 50% similarity
				{"func3", "file3.go", "sig3", 3, 0.8}, // 20% similarity
			},
			minSimilarity: 0.5,
			wantLen:       2, // func1 and func2
		},
		{
			name: "all results below threshold",
			rows: [][]any{
				{"func1", "file1.go", "sig1", 1, 0.6}, // 40% similarity
				{"func2", "file2.go", "sig2", 2, 0.8}, // 20% similarity
			},
			minSimilarity: 0.5,
			wantLen:       0,
		},
		{
			name:          "empty rows",
			rows:          [][]any{},
			minSimilarity: 0.5,
			wantLen:       0,
		},
		{
			name: "row with missing distance field",
			rows: [][]any{
				{"func1", "file1.go", "sig1", 1}, // no distance
			},
			minSimilarity: 0.5,
			wantLen:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterByMinSimilarity(tt.rows, tt.minSimilarity)
			if len(got) != tt.wantLen {
				t.Errorf("filterByMinSimilarity() returned %d rows, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// =============================================================================
// Formatting Tests
// =============================================================================

func TestFormatSemanticResults(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		rows         [][]any
		args         SemanticSearchArgs
		wantContains []string
	}{
		{
			name: "basic formatting",
			rows: [][]any{
				{"HandleAuth", "internal/auth.go", "func HandleAuth()", 10, 0.1, "func HandleAuth() { /* code */ }"},
			},
			args: SemanticSearchArgs{Query: "authentication"},
			wantContains: []string{
				"🔍 **Semantic search** for 'authentication'",
				"🟢 **HandleAuth** (90.0% match)",
				"📁 internal/auth.go:10",
			},
		},
		{
			name: "with path pattern",
			rows: [][]any{
				{"RegisterRoutes", "internal/routes.go", "func RegisterRoutes()", 5, 0.2},
			},
			args: SemanticSearchArgs{Query: "routes", PathPattern: "internal/"},
			wantContains: []string{
				"🔍 **Semantic search** for 'routes' in 'internal/'",
				"🟢 **RegisterRoutes**",
			},
		},
		{
			name: "multiple results with different confidence",
			rows: [][]any{
				{"HighMatch", "file1.go", "func HighMatch()", 1, 0.1},    // 90% - high
				{"MediumMatch", "file2.go", "func MediumMatch()", 2, 0.4}, // 60% - medium
				{"LowMatch", "file3.go", "func LowMatch()", 3, 0.6},      // 40% - low
			},
			args: SemanticSearchArgs{Query: "test"},
			wantContains: []string{
				"🟢 **HighMatch** (90.0% match)",
				"🟡 **MediumMatch** (60.0% match)",
				"🔴 **LowMatch** (40.0% match)",
			},
		},
		{
			name:         "empty results",
			rows:         [][]any{},
			args:         SemanticSearchArgs{Query: "nothing"},
			wantContains: []string{"🔍 **Semantic search** for 'nothing'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatSemanticResults(tt.rows, tt.args)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatSemanticResults() missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatSemanticResultRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		row          []any
		wantContains []string
	}{
		{
			name: "basic row without code",
			row:  []any{"MyFunc", "pkg/file.go", "func MyFunc()", 42, 0.15},
			wantContains: []string{
				"🟢 **MyFunc** (85.0% match)",
				"📁 pkg/file.go:42",
				"📝 `func MyFunc()`",
			},
		},
		{
			name: "row with code snippet",
			row:  []any{"MyFunc", "pkg/file.go", "func MyFunc()", 42, 0.15, "func MyFunc() {\n\tfmt.Println(\"test\")\n}"},
			wantContains: []string{
				"🟢 **MyFunc**",
				"```",
			},
		},
		{
			name: "row with long signature",
			row: []any{
				"LongFunc",
				"pkg/file.go",
				"func LongFunc(ctx context.Context, req *Request, opts ...Option) (*Response, error)",
				10,
				0.3,
			},
			wantContains: []string{
				"🟡 **LongFunc** (70.0% match)",
				"📁 pkg/file.go:10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			formatSemanticResultRow(&sb, 1, tt.row)
			got := sb.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatSemanticResultRow() missing %q\nGot:\n%s", want, got)
				}
			}
		})
	}
}

// =============================================================================
// HTTP Mocking for Embedding Tests
// =============================================================================

func TestGenerateEmbedding_Ollama(t *testing.T) {
	t.Parallel()

	// Mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("expected /api/embeddings, got %s", r.URL.Path)
		}

		// Verify request body
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req["model"] != "nomic-embed-text" {
			t.Errorf("expected model nomic-embed-text, got %v", req["model"])
		}

		// Return mock embedding
		resp := map[string]any{
			"embedding": []float64{0.1, 0.2, 0.3, 0.4, 0.5},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	embedding, err := generateEmbedding(ctx, server.URL, "nomic-embed-text", "test query")

	if err != nil {
		t.Fatalf("generateEmbedding() error = %v", err)
	}

	if len(embedding) != 5 {
		t.Errorf("expected 5 dimensions, got %d", len(embedding))
	}

	expectedFirst := 0.1
	if embedding[0] != expectedFirst {
		t.Errorf("embedding[0] = %f, want %f", embedding[0], expectedFirst)
	}
}

func TestGenerateEmbedding_OpenAI(t *testing.T) {
	t.Parallel()

	// Mock OpenAI-compatible server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			t.Errorf("expected /embeddings suffix, got %s", r.URL.Path)
		}

		// Return OpenAI format
		resp := map[string]any{
			"data": []map[string]any{
				{"embedding": []float64{0.5, 0.6, 0.7}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	embedding, err := generateEmbedding(ctx, server.URL+"/v1", "text-embedding-3-small", "test query")

	if err != nil {
		t.Fatalf("generateEmbedding() error = %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(embedding))
	}
}

func TestGenerateEmbedding_LlamaCpp(t *testing.T) {
	t.Parallel()

	// Mock llama.cpp server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embedding" {
			t.Errorf("expected /embedding, got %s", r.URL.Path)
		}

		// Return llama.cpp format
		resp := []map[string]any{
			{
				"index":     0,
				"embedding": [][]float64{{0.2, 0.3, 0.4}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	embedding, err := generateEmbedding(ctx, server.URL, "", "test query")

	if err != nil {
		t.Fatalf("generateEmbedding() error = %v", err)
	}

	if len(embedding) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(embedding))
	}
}

func TestGenerateEmbedding_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    string
	}{
		{
			name:       "500 server error",
			statusCode: 500,
			response:   "Internal server error",
			wantErr:    "embedding API error",
		},
		{
			name:       "404 not found",
			statusCode: 404,
			response:   "Not found",
			wantErr:    "embedding API error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			ctx := context.Background()
			_, err := generateEmbedding(ctx, server.URL, "test-model", "test query")

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateEmbedding_EmptyResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response map[string]any
		wantErr  string
	}{
		{
			name:     "empty embedding array",
			response: map[string]any{"embedding": []float64{}},
			wantErr:  "empty embedding",
		},
		{
			name: "openai empty data",
			response: map[string]any{
				"data": []map[string]any{},
			},
			wantErr: "empty embedding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			ctx := context.Background()
			_, err := generateEmbedding(ctx, server.URL, "nomic-embed-text", "test")

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// SemanticSearch Integration Tests
// =============================================================================

func TestSemanticSearch_EmptyQuery(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)
	client := NewMockClientEmpty()

	args := SemanticSearchArgs{
		Query:        "",
		EmbeddingURL: "http://localhost:11434",
	}

	result, err := SemanticSearch(ctx, client, args)

	assertNoError(t, err)
	assertContains(t, result.Text, "Error: 'query' is required")
}

func TestSemanticSearch_WithMinSimilarity(t *testing.T) {
	t.Parallel()

	// This test verifies the min_similarity filtering works correctly
	ctx := setupTest(t)

	// Create mock client that returns HNSW results
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			// Return multiple results with varying distances
			return NewMockQueryResult(
				[]string{"name", "file_path", "signature", "start_line", "distance", "code_text"},
				[][]any{
					{"HighSimilarityFunc", "file1.go", "func High()", 1, 0.1, "code1"}, // 90% similarity
					{"MedSimilarityFunc", "file2.go", "func Med()", 2, 0.4, "code2"},   // 60% similarity
					{"LowSimilarityFunc", "file3.go", "func Low()", 3, 0.7, "code3"},   // 30% similarity
				},
			), nil
		},
		nil,
	)

	// Mock embedding server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"embedding": []float64{0.1, 0.2, 0.3}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	args := SemanticSearchArgs{
		Query:         "test",
		MinSimilarity: 0.7, // 70% threshold
		EmbeddingURL:  server.URL,
		EmbeddingModel: "nomic-embed-text",
		Limit:         10,
	}

	result, err := SemanticSearch(ctx, client, args)

	assertNoError(t, err)
	// Should only include HighSimilarityFunc (90%)
	assertContains(t, result.Text, "HighSimilarityFunc")
	if strings.Contains(result.Text, "MedSimilarityFunc") {
		t.Error("Should not include MedSimilarityFunc (60% < 70% threshold)")
	}
	if strings.Contains(result.Text, "LowSimilarityFunc") {
		t.Error("Should not include LowSimilarityFunc (30% < 70% threshold)")
	}
}

func TestSemanticSearch_NoResults(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)

	// Mock client that returns empty results
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			// First call: HNSW query returns empty
			if strings.Contains(script, "~cie_function_embedding") {
				return NewMockQueryResult([]string{}, [][]any{}), nil
			}
			// Fallback to text search also returns empty
			return NewMockQueryResult([]string{}, [][]any{}), nil
		},
		nil,
	)

	// Mock embedding server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"embedding": []float64{0.1, 0.2, 0.3}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	args := SemanticSearchArgs{
		Query:          "nonexistent",
		EmbeddingURL:   server.URL,
		EmbeddingModel: "nomic-embed-text",
	}

	result, err := SemanticSearch(ctx, client, args)

	assertNoError(t, err)
	// Should fall back to text search
	assertContains(t, result.Text, "⚠️ **Text search fallback**")
	assertContains(t, result.Text, "no vectors found in HNSW index")
}

func TestSemanticSearch_EmbeddingError(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)
	client := NewMockClientEmpty()

	// Mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	args := SemanticSearchArgs{
		Query:          "test",
		EmbeddingURL:   server.URL,
		EmbeddingModel: "nomic-embed-text",
	}

	result, err := SemanticSearch(ctx, client, args)

	assertNoError(t, err)
	// Should fall back to text search with error message
	assertContains(t, result.Text, "⚠️ **Text search fallback**")
	assertContains(t, result.Text, "embedding generation failed")
}

func TestExecuteHNSWQuery(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)

	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			// Verify the query structure
			if !strings.Contains(script, "~cie_function_embedding:embedding_idx") {
				t.Error("Query should use HNSW index")
			}
			if !strings.Contains(script, "vec([") {
				t.Error("Query should include embedding vector")
			}

			return NewMockQueryResult(
				[]string{"name", "file_path", "signature", "start_line", "distance", "code_text"},
				[][]any{
					{"TestFunc", "test.go", "func TestFunc()", 1, 0.2, "code"},
				},
			), nil
		},
		nil,
	)

	embedding := []float64{0.1, 0.2, 0.3}
	args := SemanticSearchArgs{
		Query: "test",
		Limit: 10,
		Role:  "source",
	}

	result, err := executeHNSWQuery(ctx, client, embedding, args)

	assertNoError(t, err)
	if len(result.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestSemanticSearchFallback(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)

	// Mock client that returns text search results
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			// Text search query
			return NewMockQueryResult(
				[]string{"name", "file_path", "signature", "start_line", "code_text"},
				[][]any{
					{"HandleAuth", "internal/auth.go", "func HandleAuth()", 10, "func HandleAuth() {}"},
				},
			), nil
		},
		nil,
	)

	result, err := semanticSearchFallback(ctx, client, "authentication handler", 10, "source", "", "", "test reason")

	assertNoError(t, err)
	assertContains(t, result.Text, "⚠️ **Text search fallback**")
	assertContains(t, result.Text, "test reason")
	assertContains(t, result.Text, "authentication handler")
	assertContains(t, result.Text, "💡 **To enable true semantic search:**")
}

func TestSemanticSearchFallback_NoResults(t *testing.T) {
	t.Parallel()

	ctx := setupTest(t)

	// Mock client that returns empty results
	client := NewMockClientCustom(
		func(ctx context.Context, script string) (*QueryResult, error) {
			return NewMockQueryResult([]string{}, [][]any{}), nil
		},
		nil,
	)

	result, err := semanticSearchFallback(ctx, client, "nonexistent query", 10, "source", "", "", "no matches")

	assertNoError(t, err)
	assertContains(t, result.Text, "⚠️ **Text search fallback**")
	assertContains(t, result.Text, "**Tips to improve results:**")
	assertContains(t, result.Text, "cie_grep")
}
