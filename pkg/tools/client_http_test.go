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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kraklabs/cie/pkg/llm"
)

// TestNewCIEClient tests client initialization.
func TestNewCIEClient(t *testing.T) {
	client := NewCIEClient("http://localhost:8080", "test-project")

	if client.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q; want %q", client.BaseURL, "http://localhost:8080")
	}
	if client.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q; want %q", client.ProjectID, "test-project")
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient is nil")
	}
	if client.HTTPClient.Timeout != 90*time.Second {
		t.Errorf("HTTPClient.Timeout = %v; want %v", client.HTTPClient.Timeout, 90*time.Second)
	}
}

// TestCIEClient_Query_Success tests successful HTTP query.
func TestCIEClient_Query_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("Method = %q; want POST", r.Method)
		}
		// Verify URL path
		if r.URL.Path != "/v1/query" {
			t.Errorf("URL.Path = %q; want /v1/query", r.URL.Path)
		}
		// Verify Content-Type header
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q; want application/json", ct)
		}

		// Return successful response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Headers":["name","file"],"Rows":[["func1","file1.go"],["func2","file2.go"]]}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	result, err := client.Query(ctx, "?[name, file] := *cie_function { name, file_path: file }")
	assertNoError(t, err)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Headers) != 2 {
		t.Errorf("len(Headers) = %d; want 2", len(result.Headers))
	}
	if len(result.Rows) != 2 {
		t.Errorf("len(Rows) = %d; want 2", len(result.Rows))
	}
}

// TestCIEClient_Query_ServerError tests handling of HTTP 500 errors.
func TestCIEClient_Query_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	_, err := client.Query(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for 500 status")
	}
	assertContains(t, err.Error(), "status 500")
}

// TestCIEClient_Query_NotFound tests handling of HTTP 404 errors.
func TestCIEClient_Query_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	_, err := client.Query(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for 404 status")
	}
	assertContains(t, err.Error(), "status 404")
}

// TestCIEClient_Query_MalformedJSON tests handling of malformed JSON responses.
func TestCIEClient_Query_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	_, err := client.Query(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
	assertContains(t, err.Error(), "parse response")
}

// TestCIEClient_Query_ContextCancellation tests context cancellation handling.
func TestCIEClient_Query_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Headers":[],"Rows":[]}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Query(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestCIEClient_QueryRaw_Success tests successful raw query.
func TestCIEClient_QueryRaw_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Headers":["name"],"Rows":[["func1"],["func2"]]}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	result, err := client.QueryRaw(ctx, "?[name] := *cie_function { name }")
	assertNoError(t, err)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if _, ok := result["Headers"]; !ok {
		t.Error("expected Headers key in result")
	}
	if _, ok := result["Rows"]; !ok {
		t.Error("expected Rows key in result")
	}
}

// TestCIEClient_QueryRaw_ServerError tests error handling in raw query.
func TestCIEClient_QueryRaw_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"database error"}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	_, err := client.QueryRaw(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for 500 status")
	}
	assertContains(t, err.Error(), "status 500")
}

// TestCIEClient_QueryRaw_MalformedJSON tests malformed JSON in raw query.
func TestCIEClient_QueryRaw_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json at all`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	_, err := client.QueryRaw(ctx, "?[name] := *cie_function { name }")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
	assertContains(t, err.Error(), "parse response")
}

// TestCIEClient_SetLLMProvider tests LLM provider configuration.
func TestCIEClient_SetLLMProvider(t *testing.T) {
	client := NewCIEClient("http://localhost:8080", "test-project")
	mockProvider := &mockLLMProvider{}

	// Test with custom maxTokens
	client.SetLLMProvider(mockProvider, 3000)
	if client.LLMClient == nil {
		t.Error("LLMClient not set")
	}
	if client.LLMMaxTokens != 3000 {
		t.Errorf("LLMMaxTokens = %d; want 3000", client.LLMMaxTokens)
	}

	// Test with zero maxTokens (should default to 2000)
	client.SetLLMProvider(mockProvider, 0)
	if client.LLMMaxTokens != 2000 {
		t.Errorf("LLMMaxTokens = %d; want 2000 (default)", client.LLMMaxTokens)
	}

	// Test with negative maxTokens (should default to 2000)
	client.SetLLMProvider(mockProvider, -100)
	if client.LLMMaxTokens != 2000 {
		t.Errorf("LLMMaxTokens = %d; want 2000 (default)", client.LLMMaxTokens)
	}
}

// TestCIEClient_SetEmbeddingConfig tests embedding configuration.
func TestCIEClient_SetEmbeddingConfig(t *testing.T) {
	client := NewCIEClient("http://localhost:8080", "test-project")

	client.SetEmbeddingConfig("http://localhost:11434", "nomic-embed-text")
	if client.EmbeddingURL != "http://localhost:11434" {
		t.Errorf("EmbeddingURL = %q; want http://localhost:11434", client.EmbeddingURL)
	}
	if client.EmbeddingModel != "nomic-embed-text" {
		t.Errorf("EmbeddingModel = %q; want nomic-embed-text", client.EmbeddingModel)
	}
}

// TestCIEClient_Query_EmptyResponse tests handling of empty but valid responses.
func TestCIEClient_Query_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Headers":[],"Rows":[]}`))
	}))
	defer server.Close()

	client := NewCIEClient(server.URL, "test-project")
	ctx := context.Background()

	result, err := client.Query(ctx, "?[name] := *cie_function { name }, name = 'nonexistent'")
	assertNoError(t, err)

	if result == nil {
		t.Fatal("expected non-nil result for empty response")
	}
	if len(result.Headers) != 0 {
		t.Errorf("len(Headers) = %d; want 0", len(result.Headers))
	}
	if len(result.Rows) != 0 {
		t.Errorf("len(Rows) = %d; want 0", len(result.Rows))
	}
}

// mockLLMProvider is a mock implementation of llm.Provider for testing.
type mockLLMProvider struct{}

func (m *mockLLMProvider) Generate(ctx context.Context, req llm.GenerateRequest) (*llm.GenerateResponse, error) {
	return &llm.GenerateResponse{Text: "mock response", Done: true}, nil
}

func (m *mockLLMProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		Message: llm.Message{Role: "assistant", Content: "mock response"},
		Done:    true,
	}, nil
}

func (m *mockLLMProvider) Name() string {
	return "mock"
}

func (m *mockLLMProvider) Models(ctx context.Context) ([]string, error) {
	return []string{"mock-model"}, nil
}
