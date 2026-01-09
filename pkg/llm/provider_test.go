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

package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewProvider_MockType(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Type: "mock"})
	if err != nil {
		t.Fatalf("NewProvider(mock) error = %v", err)
	}
	if p == nil {
		t.Fatal("NewProvider(mock) returned nil")
	}
	if p.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", p.Name())
	}
}

func TestNewProvider_OllamaType(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Type: "ollama"})
	if err != nil {
		t.Fatalf("NewProvider(ollama) error = %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %q", p.Name())
	}
}

func TestNewProvider_OpenAIType(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Type: "openai"})
	if err != nil {
		t.Fatalf("NewProvider(openai) error = %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", p.Name())
	}
}

func TestNewProvider_AnthropicType(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Type: "anthropic"})
	if err != nil {
		t.Fatalf("NewProvider(anthropic) error = %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", p.Name())
	}
}

func TestNewProvider_UnknownType(t *testing.T) {
	_, err := NewProvider(ProviderConfig{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
	if !strings.Contains(err.Error(), "unknown LLM provider type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestMockProvider_Generate(t *testing.T) {
	p := &MockProvider{}

	ctx := context.Background()
	resp, err := p.Generate(ctx, GenerateRequest{
		Prompt: "Hello, world!",
	})
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	if resp == nil {
		t.Fatal("Generate returned nil response")
	}
	if !strings.Contains(resp.Text, "[mock]") {
		t.Errorf("expected mock response, got %q", resp.Text)
	}
	if resp.Model != "mock-model" {
		t.Errorf("expected model 'mock-model', got %q", resp.Model)
	}
	if !resp.Done {
		t.Error("expected Done=true")
	}
}

func TestMockProvider_Chat(t *testing.T) {
	p := &MockProvider{}

	ctx := context.Background()
	resp, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hello!"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	if resp == nil {
		t.Fatal("Chat returned nil response")
	}
	if resp.Message.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", resp.Message.Role)
	}
	if !strings.Contains(resp.Message.Content, "[mock]") {
		t.Errorf("expected mock response, got %q", resp.Message.Content)
	}
}

func TestMockProvider_CustomGenerateFunc(t *testing.T) {
	p := &MockProvider{
		GenerateFunc: func(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
			return &GenerateResponse{
				Text:  "Custom response for: " + req.Prompt,
				Model: "custom-model",
				Done:  true,
			}, nil
		},
	}

	ctx := context.Background()
	resp, err := p.Generate(ctx, GenerateRequest{Prompt: "test"})
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	if resp.Text != "Custom response for: test" {
		t.Errorf("unexpected response: %q", resp.Text)
	}
}

func TestMockProvider_Models(t *testing.T) {
	p := &MockProvider{}
	ctx := context.Background()

	models, err := p.Models(ctx)
	if err != nil {
		t.Fatalf("Models error = %v", err)
	}
	if len(models) != 1 || models[0] != "mock-model" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestOllamaProvider_Generate_WithMockServer(t *testing.T) {
	// Create mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"response": "This is a test response",
				"model": "test-model",
				"done": true,
				"prompt_eval_count": 10,
				"eval_count": 5
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	p, err := NewProvider(ProviderConfig{
		Type:         "ollama",
		BaseURL:      server.URL,
		DefaultModel: "test-model",
		Timeout:      5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewProvider error = %v", err)
	}

	ctx := context.Background()
	resp, err := p.Generate(ctx, GenerateRequest{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}

	if resp.Text != "This is a test response" {
		t.Errorf("unexpected text: %q", resp.Text)
	}
	if resp.Model != "test-model" {
		t.Errorf("unexpected model: %q", resp.Model)
	}
	if resp.PromptTokens != 10 {
		t.Errorf("unexpected prompt tokens: %d", resp.PromptTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("unexpected output tokens: %d", resp.OutputTokens)
	}
}

func TestOllamaProvider_Chat_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"message": {"role": "assistant", "content": "Hello! How can I help?"},
				"model": "test-model",
				"done": true,
				"prompt_eval_count": 15,
				"eval_count": 8
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	p, err := NewProvider(ProviderConfig{
		Type:         "ollama",
		BaseURL:      server.URL,
		DefaultModel: "test-model",
	})
	if err != nil {
		t.Fatalf("NewProvider error = %v", err)
	}

	ctx := context.Background()
	resp, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi!"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	if resp.Message.Content != "Hello! How can I help?" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
	if resp.Message.Role != "assistant" {
		t.Errorf("unexpected role: %q", resp.Message.Role)
	}
}

func TestOpenAIProvider_Chat_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"choices": [{
					"message": {"role": "assistant", "content": "OpenAI response"},
					"finish_reason": "stop"
				}],
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 20,
					"completion_tokens": 10,
					"total_tokens": 30
				}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	p, err := NewProvider(ProviderConfig{
		Type:    "openai",
		BaseURL: server.URL,
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatalf("NewProvider error = %v", err)
	}

	ctx := context.Background()
	resp, err := p.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	})
	if err != nil {
		t.Fatalf("Chat error = %v", err)
	}

	if resp.Message.Content != "OpenAI response" {
		t.Errorf("unexpected content: %q", resp.Message.Content)
	}
	if resp.TotalTokens != 30 {
		t.Errorf("unexpected total tokens: %d", resp.TotalTokens)
	}
}

func TestCodePrompt_Build(t *testing.T) {
	cp := CodePrompt{
		Task:     "Review this code for bugs",
		Language: "go",
		Code:     "func main() { fmt.Println(\"hello\") }",
		Context:  "This is a simple hello world program",
		Constraints: []string{
			"Focus on error handling",
			"Check for edge cases",
		},
	}

	result := cp.Build()

	if !strings.Contains(result, "Review this code for bugs") {
		t.Error("missing task")
	}
	if !strings.Contains(result, "Language: go") {
		t.Error("missing language")
	}
	if !strings.Contains(result, "```go") {
		t.Error("missing code block")
	}
	if !strings.Contains(result, "Focus on error handling") {
		t.Error("missing constraint")
	}
}

func TestBuildChatMessages(t *testing.T) {
	msgs := BuildChatMessages(
		"You are a helpful assistant",
		"What is 2+2?",
		Message{Role: "user", Content: "Hi"},
		Message{Role: "assistant", Content: "Hello!"},
	)

	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("expected first message to be system, got %q", msgs[0].Role)
	}
	if msgs[len(msgs)-1].Content != "What is 2+2?" {
		t.Errorf("expected last message to be user prompt")
	}
}
