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

package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"log/slog"
)

// EmbeddingProvider generates embeddings for code text.
type EmbeddingProvider interface {
	// Embed generates an embedding vector for the given text.
	// Returns a normalized vector (L2 norm = 1.0) or error.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// MockEmbeddingProvider generates deterministic mock embeddings for testing.
type MockEmbeddingProvider struct {
	dimension int
	logger    *slog.Logger
}

// NewMockEmbeddingProvider creates a mock embedding provider.
func NewMockEmbeddingProvider(dimension int, logger *slog.Logger) *MockEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &MockEmbeddingProvider{
		dimension: dimension,
		logger:    logger,
	}
}

// Embed generates a deterministic mock embedding based on text hash.
func (m *MockEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Generate deterministic embedding from text hash
	// This is just for testing - not semantically meaningful
	hash := hashString(text)

	embedding := make([]float32, m.dimension)
	for i := 0; i < m.dimension; i++ {
		// Use hash to generate pseudo-random values
		val := float32((hash+uint64(i)*7919)%10000) / 10000.0
		embedding[i] = val*2.0 - 1.0 // Map to [-1, 1]
	}

	// Normalize to unit vector
	norm := float32(0.0)
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding, nil
}

func hashString(s string) uint64 {
	var hash uint64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + uint64(c)
	}
	return hash
}

// EmbeddingGenerator manages embedding generation with concurrency and retries.
type EmbeddingGenerator struct {
	provider EmbeddingProvider
	workers  int
	logger   *slog.Logger
	retry    RetryConfig
}

// NewEmbeddingGenerator creates a new embedding generator.
func NewEmbeddingGenerator(provider EmbeddingProvider, workers int, logger *slog.Logger) *EmbeddingGenerator {
	if logger == nil {
		logger = slog.Default()
	}
	return &EmbeddingGenerator{
		provider: provider,
		workers:  workers,
		logger:   logger,
		retry:    RetryConfig{MaxRetries: 3, InitialBackoff: 200 * time.Millisecond, MaxBackoff: 2 * time.Second, Multiplier: 2.0},
	}
}

// SetRetryConfig sets the retry configuration for embedding operations.
func (eg *EmbeddingGenerator) SetRetryConfig(cfg RetryConfig) {
	// Basic sanity defaults to avoid zero values causing busy loops
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 200 * time.Millisecond
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 2 * time.Second
	}
	if cfg.Multiplier <= 1.0 {
		cfg.Multiplier = 2.0
	}
	eg.retry = cfg
}

// EmbedFunctionsResult contains the results of embedding generation with error counts.
type EmbedFunctionsResult struct {
	Functions      []FunctionEntity
	ErrorCount     int
	TruncatedCount int
	ErrorDetails   []string // Optional: detailed error messages (limited to avoid memory issues)
}

// EmbedFunctions generates embeddings for a batch of functions.
// Uses worker pool for concurrency.
// Returns functions with embeddings (or empty embeddings on error) and error count.
// Never returns a fatal error - continues processing even if some embeddings fail.
func (eg *EmbeddingGenerator) EmbedFunctions(ctx context.Context, functions []FunctionEntity) (*EmbedFunctionsResult, error) {
	if len(functions) == 0 {
		return &EmbedFunctionsResult{
			Functions:      functions,
			ErrorCount:     0,
			TruncatedCount: 0,
		}, nil
	}

	// Use worker pool if configured, otherwise process sequentially
	if eg.workers <= 1 {
		return eg.embedFunctionsSequential(ctx, functions)
	}

	return eg.embedFunctionsParallel(ctx, functions)
}

// embedFunctionsSequential processes embeddings sequentially (fallback for workers <= 1).
func (eg *EmbeddingGenerator) embedFunctionsSequential(ctx context.Context, functions []FunctionEntity) (*EmbedFunctionsResult, error) {
	results := make([]FunctionEntity, len(functions))
	errorCount := 0
	truncatedCount := 0

	for i, fn := range functions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		embedding, wasTruncated, err := eg.embedFunction(ctx, fn)
		if err != nil {
			errorCount++
		}
		if wasTruncated {
			truncatedCount++
		}

		fn.Embedding = embedding
		results[i] = fn
	}

	// Log summary if there were errors or truncations (instead of individual warnings)
	if errorCount > 0 || truncatedCount > 0 {
		eg.logger.Info("embedding.summary",
			"total_functions", len(functions),
			"errors", errorCount,
			"truncated", truncatedCount,
		)
	}

	return &EmbedFunctionsResult{
		Functions:      results,
		ErrorCount:     errorCount,
		TruncatedCount: truncatedCount,
	}, nil
}

// embedFunctionsParallel processes embeddings in parallel using worker pool.
func (eg *EmbeddingGenerator) embedFunctionsParallel(ctx context.Context, functions []FunctionEntity) (*EmbedFunctionsResult, error) {
	results := make([]FunctionEntity, len(functions))
	errorCount := int32(0) // Use atomic for thread safety
	truncatedCount := int32(0)

	// Create channels for work distribution
	jobs := make(chan int, len(functions))
	resultsChan := make(chan struct {
		index     int
		function  FunctionEntity
		err       bool
		truncated bool
	}, len(functions))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < eg.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				fn := functions[i]
				embedding, wasTruncated, err := eg.embedFunction(ctx, fn)
				if err != nil {
					// Atomic increment
					for {
						old := errorCount
						if atomic.CompareAndSwapInt32(&errorCount, old, old+1) {
							break
						}
					}
				}
				if wasTruncated {
					for {
						old := truncatedCount
						if atomic.CompareAndSwapInt32(&truncatedCount, old, old+1) {
							break
						}
					}
				}

				fn.Embedding = embedding
				resultsChan <- struct {
					index     int
					function  FunctionEntity
					err       bool
					truncated bool
				}{i, fn, err != nil, wasTruncated}
			}
		}()
	}

	// Send jobs
	for i := range functions {
		jobs <- i
	}
	close(jobs)

	// Wait for workers and collect results
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results[result.index] = result.function
	}

	// Log summary if there were errors or truncations (aggregated, not individual)
	errCount := int(errorCount)
	truncCount := int(truncatedCount)
	if errCount > 0 || truncCount > 0 {
		eg.logger.Info("embedding.summary",
			"total_functions", len(functions),
			"errors", errCount,
			"truncated", truncCount,
			"workers", eg.workers,
			"error_rate_pct", float64(errCount)/float64(len(functions))*100.0,
		)
	}

	return &EmbedFunctionsResult{
		Functions:      results,
		ErrorCount:     errCount,
		TruncatedCount: truncCount,
	}, nil
}

// EmbedTypesResult contains the results of embedding generation for types.
type EmbedTypesResult struct {
	Types          []TypeEntity
	ErrorCount     int
	TruncatedCount int
}

// EmbedTypes generates embeddings for a batch of types.
// Uses the same logic as EmbedFunctions but for TypeEntity.
// Returns types with embeddings (or empty embeddings on error) and error count.
func (eg *EmbeddingGenerator) EmbedTypes(ctx context.Context, types []TypeEntity) (*EmbedTypesResult, error) {
	if len(types) == 0 {
		return &EmbedTypesResult{
			Types:          types,
			ErrorCount:     0,
			TruncatedCount: 0,
		}, nil
	}

	// Use worker pool if configured, otherwise process sequentially
	if eg.workers <= 1 {
		return eg.embedTypesSequential(ctx, types)
	}

	return eg.embedTypesParallel(ctx, types)
}

// embedTypesSequential processes type embeddings sequentially (fallback for workers <= 1).
func (eg *EmbeddingGenerator) embedTypesSequential(ctx context.Context, types []TypeEntity) (*EmbedTypesResult, error) {
	results := make([]TypeEntity, len(types))
	errorCount := 0
	truncatedCount := 0

	for i, t := range types {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		embedding, wasTruncated, err := eg.embedType(ctx, t)
		if err != nil {
			errorCount++
		}
		if wasTruncated {
			truncatedCount++
		}

		t.Embedding = embedding
		results[i] = t
	}

	if errorCount > 0 || truncatedCount > 0 {
		eg.logger.Info("embedding.types.summary",
			"total_types", len(types),
			"errors", errorCount,
			"truncated", truncatedCount,
		)
	}

	return &EmbedTypesResult{
		Types:          results,
		ErrorCount:     errorCount,
		TruncatedCount: truncatedCount,
	}, nil
}

// embedTypesParallel processes type embeddings in parallel using worker pool.
func (eg *EmbeddingGenerator) embedTypesParallel(ctx context.Context, types []TypeEntity) (*EmbedTypesResult, error) {
	results := make([]TypeEntity, len(types))
	errorCount := int32(0)
	truncatedCount := int32(0)

	// Create channels for work distribution
	jobs := make(chan int, len(types))
	resultsChan := make(chan struct {
		index     int
		typeEnt   TypeEntity
		err       bool
		truncated bool
	}, len(types))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < eg.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				t := types[i]
				embedding, wasTruncated, err := eg.embedType(ctx, t)
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
				if wasTruncated {
					atomic.AddInt32(&truncatedCount, 1)
				}

				t.Embedding = embedding
				resultsChan <- struct {
					index     int
					typeEnt   TypeEntity
					err       bool
					truncated bool
				}{i, t, err != nil, wasTruncated}
			}
		}()
	}

	// Send jobs
	for i := range types {
		jobs <- i
	}
	close(jobs)

	// Wait for workers and collect results
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results[result.index] = result.typeEnt
	}

	// Log summary if there were errors or truncations
	errCount := int(errorCount)
	truncCount := int(truncatedCount)
	if errCount > 0 || truncCount > 0 {
		eg.logger.Info("embedding.types.summary",
			"total_types", len(types),
			"errors", errCount,
			"truncated", truncCount,
			"workers", eg.workers,
			"error_rate_pct", float64(errCount)/float64(len(types))*100.0,
		)
	}

	return &EmbedTypesResult{
		Types:          results,
		ErrorCount:     errCount,
		TruncatedCount: truncCount,
	}, nil
}

// embedType embeds a single type with retry logic.
func (eg *EmbeddingGenerator) embedType(ctx context.Context, t TypeEntity) ([]float32, bool, error) {
	text := t.CodeText
	maxChars := 2000
	wasTruncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		wasTruncated = true
	}

	var embedding []float32
	var err error
	maxRetries := eg.retry.MaxRetries
	base := eg.retry.InitialBackoff
	maxBackoff := eg.retry.MaxBackoff
	mult := eg.retry.Multiplier
	for attempt := 0; attempt < maxRetries; attempt++ {
		embedding, err = eg.provider.Embed(ctx, text)
		if err == nil {
			break
		}
		retryable := isRetryableEmbeddingError(err)
		if !retryable || attempt == maxRetries-1 {
			break
		}
		sleep := computeBackoffWithJitter(base, attempt, mult, maxBackoff)
		recordEmbedRetry()
		eg.logger.Warn("embedding.type.retry", "type_id", t.ID, "attempt", attempt+1, "sleep_ms", sleep.Milliseconds(), "err", err)
		select {
		case <-ctx.Done():
			return nil, wasTruncated, ctx.Err()
		case <-time.After(sleep):
		}
	}

	if err != nil {
		eg.logger.Error("embedding.type.failed",
			"type_id", t.ID,
			"type_name", t.Name,
			"code_text_len", len(t.CodeText),
			"error", err,
		)
		embedding = []float32{}
	}

	return embedding, wasTruncated, err
}

// embedFunction embeds a single function with retry logic.
// Returns embedding, wasTruncated flag, and error.
func (eg *EmbeddingGenerator) embedFunction(ctx context.Context, fn FunctionEntity) ([]float32, bool, error) {
	// Truncate code text if too long (embedding models have token limits)
	// nomic-embed-text has ~8192 token limit, but code tokenizes poorly
	// (special chars, operators = multiple tokens). Using 2000 chars as safe limit.
	text := fn.CodeText
	maxChars := 2000 // Conservative limit for code (may be ~3000-4000 tokens)
	wasTruncated := false
	if len(text) > maxChars {
		text = text[:maxChars]
		wasTruncated = true
	}

	// Generate embedding with classified retry + jittered backoff
	var embedding []float32
	var err error
	maxRetries := eg.retry.MaxRetries
	base := eg.retry.InitialBackoff
	maxBackoff := eg.retry.MaxBackoff
	mult := eg.retry.Multiplier
	for attempt := 0; attempt < maxRetries; attempt++ {
		embedding, err = eg.provider.Embed(ctx, text)
		if err == nil {
			break
		}
		retryable := isRetryableEmbeddingError(err)
		if !retryable || attempt == maxRetries-1 {
			break
		}
		// Exponential backoff with full jitter
		sleep := computeBackoffWithJitter(base, attempt, mult, maxBackoff)
		recordEmbedRetry()
		eg.logger.Warn("embedding.retry", "function_id", fn.ID, "attempt", attempt+1, "sleep_ms", sleep.Milliseconds(), "err", err)
		select {
		case <-ctx.Done():
			return nil, wasTruncated, ctx.Err()
		case <-time.After(sleep):
		}
	}

	if err != nil {
		// Log the specific function that failed for debugging
		eg.logger.Error("embedding.function.failed",
			"function_id", fn.ID,
			"function_name", fn.Name,
			"code_text_len", len(fn.CodeText),
			"error", err,
		)
		// Graceful failure: use empty embedding and continue
		embedding = []float32{} // Empty embedding as placeholder
	}

	return embedding, wasTruncated, err
}

// isRetryableEmbeddingError classifies provider errors: network/timeout and HTTP 5xx/429 are retryable.
func isRetryableEmbeddingError(err error) bool {
	if err == nil {
		return false
	}
	// Best-effort classification based on error text to avoid importing provider internals
	msg := err.Error()
	// Common retryable substrings
	retrySubstr := []string{"timeout", "temporarily unavailable", "connection refused", "connection reset", "deadline exceeded", "EOF"}
	for _, s := range retrySubstr {
		if containsFold(msg, s) {
			return true
		}
	}
	// HTTP status codes if present in message
	// treat 429 and 5xx as retryable
	httpRetry := []string{" 429 ", " 500 ", " 502 ", " 503 ", " 504 "}
	for _, s := range httpRetry {
		if containsFold(msg, s) {
			return true
		}
	}
	return false
}

// computeBackoffWithJitter returns exponential backoff with full jitter
func computeBackoffWithJitter(base time.Duration, attempt int, mult float64, capDur time.Duration) time.Duration {
	// exp = base * mult^attempt
	exp := float64(base)
	for i := 0; i < attempt; i++ {
		exp *= mult
	}
	d := time.Duration(exp)
	if d > capDur {
		d = capDur
	}
	// full jitter [0, d]
	if d <= 0 {
		return base
	}
	n := time.Duration(randInt63n(int64(d) + 1))
	return n
}

// containsFold is a lightweight strings.ContainsFold
func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// randInt63n returns [0,n). Separate to avoid importing math/rand globally here.
var randMu sync.Mutex
var randSeed int64

func randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	randMu.Lock()
	defer randMu.Unlock()
	// simple LCG for deterministic-ish jitter without extra deps
	// X_{k+1} = (a X_k + c) mod m
	const a = 6364136223846793005
	const c = 1
	const m = 1<<63 - 1
	if randSeed == 0 {
		randSeed = time.Now().UnixNano() & m
	}
	randSeed = (a*randSeed + c) & m
	if randSeed < 0 {
		randSeed = -randSeed
	}
	return randSeed % n
}

// CreateEmbeddingProvider creates an embedding provider based on config.
// Supported providers:
//   - "mock": Deterministic mock embeddings for testing (384 dimensions)
//   - "nomic": Nomic Atlas API (requires NOMIC_API_KEY env var)
//   - "ollama": Local Ollama server (default: http://localhost:11434)
//   - "openai": OpenAI-compatible API (requires OPENAI_API_KEY and optionally OPENAI_API_BASE)
func CreateEmbeddingProvider(providerType string, logger *slog.Logger) (EmbeddingProvider, error) {
	switch providerType {
	case "mock":
		return NewMockEmbeddingProvider(384, logger), nil // 384 is a common embedding dimension

	case "nomic":
		apiKey := os.Getenv("NOMIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("NOMIC_API_KEY environment variable is required for nomic provider")
		}
		baseURL := os.Getenv("NOMIC_API_BASE")
		if baseURL == "" {
			baseURL = "https://api-atlas.nomic.ai/v1"
		}
		model := os.Getenv("NOMIC_MODEL")
		if model == "" {
			model = "nomic-embed-text-v1.5" // Default model
		}
		return NewNomicEmbeddingProvider(apiKey, baseURL, model, logger), nil

	case "ollama", "local_model":
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := os.Getenv("OLLAMA_EMBED_MODEL")
		if model == "" {
			model = "nomic-embed-text" // Default embedding model for Ollama
		}
		return NewOllamaEmbeddingProvider(baseURL, model, logger), nil

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required for openai provider")
		}
		baseURL := os.Getenv("OPENAI_API_BASE")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := os.Getenv("OPENAI_EMBED_MODEL")
		if model == "" {
			model = "text-embedding-3-small" // Default OpenAI embedding model
		}
		return NewOpenAIEmbeddingProvider(apiKey, baseURL, model, logger), nil

	case "llamacpp", "qodo":
		// LlamaCpp server for Qodo-Embed-1-1.5B (1536 dimensions)
		// Runs locally via: llama-server --embedding -m Qodo-Embed-1-1.5B-Q8_0.gguf --port 8090
		baseURL := os.Getenv("LLAMACPP_EMBED_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8090"
		}
		return NewLlamaCppEmbeddingProvider(baseURL, logger), nil

	default:
		return nil, fmt.Errorf("unknown embedding provider: %s (supported: mock, nomic, ollama, openai, llamacpp, qodo)", providerType)
	}
}

// =============================================================================
// NOMIC EMBEDDING PROVIDER
// =============================================================================

// NomicEmbeddingProvider generates embeddings using the Nomic Atlas API.
// Nomic provides high-quality code and text embeddings with a generous free tier.
// API Docs: https://docs.nomic.ai/reference/endpoints/nomic-embed-text
type NomicEmbeddingProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NomicEmbedRequest represents the request body for Nomic embeddings API.
type NomicEmbedRequest struct {
	Texts    []string `json:"texts"`
	Model    string   `json:"model"`
	TaskType string   `json:"task_type,omitempty"` // "search_document", "search_query", "clustering", "classification"
}

// NomicEmbedResponse represents the response from Nomic embeddings API.
type NomicEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model"`
	Usage      struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// NomicErrorResponse represents an error response from Nomic API.
type NomicErrorResponse struct {
	Detail string `json:"detail"`
}

// NewNomicEmbeddingProvider creates a new Nomic embedding provider.
func NewNomicEmbeddingProvider(apiKey, baseURL, model string, logger *slog.Logger) *NomicEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &NomicEmbeddingProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// Embed generates an embedding for the given text using Nomic API.
func (n *NomicEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Build request
	reqBody := NomicEmbedRequest{
		Texts:    []string{text},
		Model:    n.model,
		TaskType: "search_document", // Optimized for retrieval
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	url := n.baseURL + "/embedding/text"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.apiKey)

	// Execute request
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		var errResp NomicErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Detail != "" {
			return nil, fmt.Errorf("nomic API error (status %d): %s", resp.StatusCode, errResp.Detail)
		}
		return nil, fmt.Errorf("nomic API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embedResp NomicEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(embedResp.Embeddings) == 0 {
		return nil, fmt.Errorf("nomic returned empty embeddings")
	}

	// Convert float64 to float32 and normalize
	embedding := make([]float32, len(embedResp.Embeddings[0]))
	for i, v := range embedResp.Embeddings[0] {
		embedding[i] = float32(v)
	}

	// Normalize to unit vector (Nomic embeddings should already be normalized, but verify)
	embedding = normalizeEmbedding(embedding)

	return embedding, nil
}

// =============================================================================
// OLLAMA EMBEDDING PROVIDER
// =============================================================================

// OllamaEmbeddingProvider generates embeddings using a local Ollama server.
// Ollama runs models locally and provides an OpenAI-compatible API.
// Supports models like nomic-embed-text, mxbai-embed-large, all-minilm, etc.
type OllamaEmbeddingProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

// OllamaEmbedRequest represents the request body for Ollama embeddings API.
type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbedResponse represents the response from Ollama embeddings API.
type OllamaEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

// OllamaErrorResponse represents an error response from Ollama.
type OllamaErrorResponse struct {
	Error string `json:"error"`
}

// isNomicModel checks if the model is a Nomic embedding model that supports
// asymmetric search prefixes (search_document/search_query).
func isNomicModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "nomic")
}

// isQodoModel checks if the model is a Qodo embedding model.
// Qodo-Embed models are trained on natural language <-> code pairs directly,
// requiring no special prefixes for documents or queries.
// See: https://huggingface.co/Qodo/Qodo-Embed-1-1.5B
func isQodoModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "qodo")
}

// NewOllamaEmbeddingProvider creates a new Ollama embedding provider.
func NewOllamaEmbeddingProvider(baseURL, model string, logger *slog.Logger) *OllamaEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &OllamaEmbeddingProvider{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Local models may be slower
		},
		logger: logger,
	}
}

// Embed generates an embedding for the given text using local Ollama.
func (o *OllamaEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// For nomic-embed-text and similar models, add "search_document:" prefix
	// to enable asymmetric embeddings. This significantly improves retrieval
	// quality when queries use "search_query:" prefix.
	// See: https://huggingface.co/nomic-ai/nomic-embed-text-v1.5
	prompt := text
	if isNomicModel(o.model) {
		prompt = "search_document: " + text
	}

	// Build request
	reqBody := OllamaEmbedRequest{
		Model:  o.model,
		Prompt: prompt,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	url := o.baseURL + "/api/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request (is Ollama running at %s?): %w", o.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		var errResp OllamaErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embedResp OllamaEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(embedResp.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}

	// Convert float64 to float32
	embedding := make([]float32, len(embedResp.Embedding))
	for i, v := range embedResp.Embedding {
		embedding[i] = float32(v)
	}

	// Normalize to unit vector
	embedding = normalizeEmbedding(embedding)

	return embedding, nil
}

// =============================================================================
// OPENAI-COMPATIBLE EMBEDDING PROVIDER
// =============================================================================

// OpenAIEmbeddingProvider generates embeddings using OpenAI or compatible APIs.
// Works with OpenAI, Azure OpenAI, Anyscale, Together AI, etc.
type OpenAIEmbeddingProvider struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *slog.Logger
}

// OpenAIEmbedRequest represents the request body for OpenAI embeddings API.
type OpenAIEmbedRequest struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format,omitempty"` // "float" or "base64"
}

// OpenAIEmbedResponse represents the response from OpenAI embeddings API.
type OpenAIEmbedResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// OpenAIErrorResponse represents an error response from OpenAI API.
type OpenAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider.
func NewOpenAIEmbeddingProvider(apiKey, baseURL, model string, logger *slog.Logger) *OpenAIEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &OpenAIEmbeddingProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// Embed generates an embedding for the given text using OpenAI API.
// For Qodo-Embed models (based on gte-Qwen2), documents are embedded as-is without prefix.
// Asymmetric search is handled by adding "Instruct:\nQuery:" format to queries during search.
func (o *OpenAIEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Documents (code) are embedded as-is without prefix for Qodo-Embed models
	// The asymmetric search instruction is added only to queries during search time

	// Build request
	reqBody := OpenAIEmbedRequest{
		Input:          text,
		Model:          o.model,
		EncodingFormat: "float",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	url := o.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	// Execute request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		var errResp OpenAIErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var embedResp OpenAIEmbedResponse
	if err := json.Unmarshal(body, &embedResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(embedResp.Data) == 0 || len(embedResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai returned empty embedding")
	}

	// Convert float64 to float32
	embedding := make([]float32, len(embedResp.Data[0].Embedding))
	for i, v := range embedResp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	// Normalize to unit vector (OpenAI embeddings are already normalized, but verify)
	embedding = normalizeEmbedding(embedding)

	return embedding, nil
}

// =============================================================================
// LLAMACPP EMBEDDING PROVIDER (Qodo-Embed-1)
// =============================================================================

// LlamaCppEmbeddingProvider generates embeddings using a llama.cpp server.
// Designed for Qodo-Embed-1-1.5B which produces 1536-dimensional embeddings.
// The server should be running with: llama-server --embedding -m model.gguf --port 8090
type LlamaCppEmbeddingProvider struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

// LlamaCppEmbedRequest represents the request body for llama.cpp embeddings API.
type LlamaCppEmbedRequest struct {
	Content string `json:"content"`
}

// LlamaCppEmbedResponse represents a single embedding result from llama.cpp.
type LlamaCppEmbedResponse struct {
	Index     int         `json:"index"`
	Embedding [][]float64 `json:"embedding"` // Nested array: [[...vectors...]]
}

// NewLlamaCppEmbeddingProvider creates a new llama.cpp embedding provider.
func NewLlamaCppEmbeddingProvider(baseURL string, logger *slog.Logger) *LlamaCppEmbeddingProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &LlamaCppEmbeddingProvider{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Local models may be slower
		},
		logger: logger,
	}
}

// Embed generates an embedding for the given text using llama.cpp server.
// For Qodo-Embed-1, documents are embedded as-is without prefix.
// The model was trained on natural language <-> code pairs directly.
func (l *LlamaCppEmbeddingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// Qodo-Embed models: no prefix needed (trained on raw pairs)
	// See: https://huggingface.co/Qodo/Qodo-Embed-1-1.5B

	// Build request
	reqBody := LlamaCppEmbedRequest{
		Content: text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	url := l.baseURL + "/embedding"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request (is llama-server running at %s?): %w", l.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Handle errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llama.cpp API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response - llama.cpp returns an array of embedding objects
	var embedResps []LlamaCppEmbedResponse
	if err := json.Unmarshal(body, &embedResps); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(embedResps) == 0 || len(embedResps[0].Embedding) == 0 {
		return nil, fmt.Errorf("llama.cpp returned empty embedding")
	}

	// Get the first (and usually only) embedding vector from the nested array
	vectors := embedResps[0].Embedding
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("llama.cpp returned empty embedding vector")
	}

	// Convert float64 to float32
	embedding := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		embedding[i] = float32(v)
	}

	// Normalize to unit vector
	embedding = normalizeEmbedding(embedding)

	return embedding, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// normalizeEmbedding normalizes an embedding vector to unit length (L2 norm = 1).
func normalizeEmbedding(embedding []float32) []float32 {
	if len(embedding) == 0 {
		return embedding
	}

	// Calculate L2 norm
	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)

	// Avoid division by zero
	if norm == 0 {
		return embedding
	}

	// Normalize
	normf := float32(norm)
	for i := range embedding {
		embedding[i] /= normf
	}

	return embedding
}
