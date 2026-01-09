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
	"context"
	"math"
	"testing"
)

func TestMockEmbeddingProvider_Embed(t *testing.T) {
	provider := NewMockEmbeddingProvider(384, nil)

	ctx := context.Background()
	text := "func main() { fmt.Println(\"Hello, World!\") }"

	embedding, err := provider.Embed(ctx, text)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	// Check dimension
	if len(embedding) != 384 {
		t.Errorf("Embed() dimension = %d, want 384", len(embedding))
	}

	// Check normalization (L2 norm should be ~1.0)
	var norm float64
	for _, v := range embedding {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.001 {
		t.Errorf("Embed() L2 norm = %f, want ~1.0", norm)
	}

	// Check determinism - same text should produce same embedding
	embedding2, err := provider.Embed(ctx, text)
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	for i := range embedding {
		if embedding[i] != embedding2[i] {
			t.Errorf("Embed() not deterministic at index %d: %f != %f", i, embedding[i], embedding2[i])
			break
		}
	}

	// Different text should produce different embedding
	embedding3, err := provider.Embed(ctx, "different text")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	same := true
	for i := range embedding {
		if embedding[i] != embedding3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Embed() should produce different embeddings for different texts")
	}
}

func TestNormalizeEmbedding(t *testing.T) {
	tests := []struct {
		name  string
		input []float32
	}{
		{
			name:  "typical vector",
			input: []float32{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:  "already normalized",
			input: []float32{0.5773, 0.5773, 0.5773}, // ~1/sqrt(3) each
		},
		{
			name:  "large values",
			input: []float32{1000.0, 2000.0, 3000.0},
		},
		{
			name:  "small values",
			input: []float32{0.001, 0.002, 0.003},
		},
		{
			name:  "negative values",
			input: []float32{-1.0, 2.0, -3.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEmbedding(tt.input)

			// Check L2 norm is ~1.0
			var norm float64
			for _, v := range result {
				norm += float64(v) * float64(v)
			}
			norm = math.Sqrt(norm)

			if math.Abs(norm-1.0) > 0.001 {
				t.Errorf("normalizeEmbedding() L2 norm = %f, want ~1.0", norm)
			}
		})
	}
}

func TestNormalizeEmbedding_ZeroVector(t *testing.T) {
	// Zero vector should remain zero (can't normalize)
	input := []float32{0.0, 0.0, 0.0}
	result := normalizeEmbedding(input)

	for i, v := range result {
		if v != 0.0 {
			t.Errorf("normalizeEmbedding() expected 0.0 at index %d, got %f", i, v)
		}
	}
}

func TestNormalizeEmbedding_Empty(t *testing.T) {
	// Empty vector should remain empty
	input := []float32{}
	result := normalizeEmbedding(input)

	if len(result) != 0 {
		t.Errorf("normalizeEmbedding() expected empty, got %d elements", len(result))
	}
}

func TestCreateEmbeddingProvider_Mock(t *testing.T) {
	provider, err := CreateEmbeddingProvider("mock", nil)
	if err != nil {
		t.Fatalf("CreateEmbeddingProvider(mock) error = %v", err)
	}

	if provider == nil {
		t.Fatal("CreateEmbeddingProvider(mock) returned nil")
	}

	// Verify it works
	ctx := context.Background()
	embedding, err := provider.Embed(ctx, "test")
	if err != nil {
		t.Fatalf("provider.Embed() error = %v", err)
	}
	if len(embedding) != 384 {
		t.Errorf("provider.Embed() dimension = %d, want 384", len(embedding))
	}
}

func TestCreateEmbeddingProvider_NomicRequiresAPIKey(t *testing.T) {
	// Clear the env var to ensure clean test
	t.Setenv("NOMIC_API_KEY", "")

	_, err := CreateEmbeddingProvider("nomic", nil)
	if err == nil {
		t.Error("CreateEmbeddingProvider(nomic) should error without API key")
	}
}

func TestCreateEmbeddingProvider_OpenAIRequiresAPIKey(t *testing.T) {
	// Clear the env var to ensure clean test
	t.Setenv("OPENAI_API_KEY", "")

	_, err := CreateEmbeddingProvider("openai", nil)
	if err == nil {
		t.Error("CreateEmbeddingProvider(openai) should error without API key")
	}
}

func TestCreateEmbeddingProvider_OllamaNoKeyRequired(t *testing.T) {
	provider, err := CreateEmbeddingProvider("ollama", nil)
	if err != nil {
		t.Fatalf("CreateEmbeddingProvider(ollama) error = %v", err)
	}

	if provider == nil {
		t.Fatal("CreateEmbeddingProvider(ollama) returned nil")
	}

	// Verify it's an OllamaEmbeddingProvider
	_, ok := provider.(*OllamaEmbeddingProvider)
	if !ok {
		t.Error("CreateEmbeddingProvider(ollama) should return *OllamaEmbeddingProvider")
	}
}

func TestCreateEmbeddingProvider_LocalModelAlias(t *testing.T) {
	provider, err := CreateEmbeddingProvider("local_model", nil)
	if err != nil {
		t.Fatalf("CreateEmbeddingProvider(local_model) error = %v", err)
	}

	// local_model is an alias for ollama
	_, ok := provider.(*OllamaEmbeddingProvider)
	if !ok {
		t.Error("CreateEmbeddingProvider(local_model) should return *OllamaEmbeddingProvider")
	}
}

func TestCreateEmbeddingProvider_Unknown(t *testing.T) {
	_, err := CreateEmbeddingProvider("unknown_provider", nil)
	if err == nil {
		t.Error("CreateEmbeddingProvider(unknown) should error")
	}
}

func TestNomicEmbeddingProvider_Structure(t *testing.T) {
	provider := NewNomicEmbeddingProvider("test-key", "https://api.test.com", "test-model", nil)

	if provider.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want 'test-key'", provider.apiKey)
	}
	if provider.baseURL != "https://api.test.com" {
		t.Errorf("baseURL = %q, want 'https://api.test.com'", provider.baseURL)
	}
	if provider.model != "test-model" {
		t.Errorf("model = %q, want 'test-model'", provider.model)
	}
	if provider.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestOllamaEmbeddingProvider_Structure(t *testing.T) {
	provider := NewOllamaEmbeddingProvider("http://localhost:11434", "nomic-embed-text", nil)

	if provider.baseURL != "http://localhost:11434" {
		t.Errorf("baseURL = %q, want 'http://localhost:11434'", provider.baseURL)
	}
	if provider.model != "nomic-embed-text" {
		t.Errorf("model = %q, want 'nomic-embed-text'", provider.model)
	}
	if provider.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestOpenAIEmbeddingProvider_Structure(t *testing.T) {
	provider := NewOpenAIEmbeddingProvider("sk-test", "https://api.openai.com/v1", "text-embedding-3-small", nil)

	if provider.apiKey != "sk-test" {
		t.Errorf("apiKey = %q, want 'sk-test'", provider.apiKey)
	}
	if provider.baseURL != "https://api.openai.com/v1" {
		t.Errorf("baseURL = %q, want 'https://api.openai.com/v1'", provider.baseURL)
	}
	if provider.model != "text-embedding-3-small" {
		t.Errorf("model = %q, want 'text-embedding-3-small'", provider.model)
	}
	if provider.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestCreateEmbeddingProvider_EnvVarConfiguration(t *testing.T) {
	// Test Nomic with env vars
	t.Setenv("NOMIC_API_KEY", "test-nomic-key")
	t.Setenv("NOMIC_API_BASE", "https://custom.nomic.api")
	t.Setenv("NOMIC_MODEL", "custom-model")

	provider, err := CreateEmbeddingProvider("nomic", nil)
	if err != nil {
		t.Fatalf("CreateEmbeddingProvider(nomic) error = %v", err)
	}

	np, ok := provider.(*NomicEmbeddingProvider)
	if !ok {
		t.Fatal("expected *NomicEmbeddingProvider")
	}

	if np.apiKey != "test-nomic-key" {
		t.Errorf("apiKey = %q, want 'test-nomic-key'", np.apiKey)
	}
	if np.baseURL != "https://custom.nomic.api" {
		t.Errorf("baseURL = %q, want 'https://custom.nomic.api'", np.baseURL)
	}
	if np.model != "custom-model" {
		t.Errorf("model = %q, want 'custom-model'", np.model)
	}
}
