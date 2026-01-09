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

// Package llm provides a unified interface for Large Language Model providers.
// Supports multiple backends: Ollama, OpenAI-compatible APIs, and more.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider defines the interface for LLM text generation.
type Provider interface {
	// Generate produces a text completion for the given prompt.
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)

	// Chat handles multi-turn conversations.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// Name returns the provider identifier.
	Name() string

	// Models returns available models for this provider.
	Models(ctx context.Context) ([]string, error)
}

// GenerateRequest represents a text generation request.
type GenerateRequest struct {
	Prompt      string         `json:"prompt"`
	Model       string         `json:"model,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
}

// GenerateResponse contains the LLM response.
type GenerateResponse struct {
	Text         string        `json:"text"`
	Model        string        `json:"model"`
	PromptTokens int           `json:"prompt_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	TotalTokens  int           `json:"total_tokens,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Done         bool          `json:"done"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Messages    []Message      `json:"messages"`
	Model       string         `json:"model,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
}

// ChatResponse contains the chat completion response.
type ChatResponse struct {
	Message      Message       `json:"message"`
	Model        string        `json:"model"`
	PromptTokens int           `json:"prompt_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	TotalTokens  int           `json:"total_tokens,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	Done         bool          `json:"done"`
}

// ProviderConfig holds configuration for creating providers.
type ProviderConfig struct {
	// Provider type: "ollama", "openai", "anthropic", "mock"
	Type string `json:"type"`

	// BaseURL for the API endpoint
	BaseURL string `json:"base_url,omitempty"`

	// APIKey for authenticated providers (OpenAI, Anthropic)
	APIKey string `json:"api_key,omitempty"`

	// DefaultModel to use if not specified in requests
	DefaultModel string `json:"default_model,omitempty"`

	// Timeout for API requests
	Timeout time.Duration `json:"timeout,omitempty"`

	// MaxRetries for transient failures
	MaxRetries int `json:"max_retries,omitempty"`
}

// NewProvider creates a Provider based on configuration.
// Supported types: "ollama", "openai", "anthropic", "mock"
//
// Environment variables:
//   - OLLAMA_HOST: Ollama server URL (default: http://localhost:11434)
//   - OLLAMA_MODEL: Default Ollama model
//   - OPENAI_API_KEY: OpenAI API key
//   - OPENAI_BASE_URL: OpenAI-compatible API URL
//   - OPENAI_MODEL: Default OpenAI model
//   - ANTHROPIC_API_KEY: Anthropic API key
func NewProvider(cfg ProviderConfig) (Provider, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	switch strings.ToLower(cfg.Type) {
	case "ollama", "local", "":
		return newOllamaProvider(cfg)
	case "openai", "openai-compatible":
		return newOpenAIProvider(cfg)
	case "anthropic", "claude":
		return newAnthropicProvider(cfg)
	case "mock", "test":
		return &MockProvider{model: cfg.DefaultModel}, nil
	default:
		return nil, fmt.Errorf("unknown LLM provider type: %s (supported: ollama, openai, anthropic, mock)", cfg.Type)
	}
}

// =============================================================================
// OLLAMA PROVIDER
// =============================================================================

type ollamaProvider struct {
	baseURL      string
	defaultModel string
	client       *http.Client
	maxRetries   int
}

func newOllamaProvider(cfg ProviderConfig) (*ollamaProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_HOST")
	}
	if baseURL == "" {
		baseURL = os.Getenv("OLLAMA_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := cfg.DefaultModel
	if model == "" {
		model = os.Getenv("OLLAMA_MODEL")
	}

	return &ollamaProvider{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		defaultModel: model,
		client:       &http.Client{Timeout: cfg.Timeout},
		maxRetries:   cfg.MaxRetries,
	}, nil
}

func (p *ollamaProvider) Name() string { return "ollama" }

func (p *ollamaProvider) Models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}
	return models, nil
}

func (p *ollamaProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("ollama: model not specified (set OLLAMA_MODEL or pass in request)")
	}

	payload := map[string]any{
		"model":  model,
		"prompt": req.Prompt,
		"stream": false,
	}
	if req.MaxTokens > 0 {
		if payload["options"] == nil {
			payload["options"] = map[string]any{}
		}
		payload["options"].(map[string]any)["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		if payload["options"] == nil {
			payload["options"] = map[string]any{}
		}
		payload["options"].(map[string]any)["temperature"] = req.Temperature
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama generate error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Response        string `json:"response"`
		Model           string `json:"model"`
		Done            bool   `json:"done"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
		TotalDuration   int64  `json:"total_duration"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &GenerateResponse{
		Text:         result.Response,
		Model:        result.Model,
		PromptTokens: result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		TotalTokens:  result.PromptEvalCount + result.EvalCount,
		Duration:     time.Since(start),
		Done:         result.Done,
	}, nil
}

func (p *ollamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("ollama: model not specified (set OLLAMA_MODEL or pass in request)")
	}

	// Convert messages to Ollama format
	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	if req.MaxTokens > 0 {
		if payload["options"] == nil {
			payload["options"] = map[string]any{}
		}
		payload["options"].(map[string]any)["num_predict"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		if payload["options"] == nil {
			payload["options"] = map[string]any{}
		}
		payload["options"].(map[string]any)["temperature"] = req.Temperature
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama chat error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Model           string `json:"model"`
		Done            bool   `json:"done"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &ChatResponse{
		Message: Message{
			Role:    result.Message.Role,
			Content: result.Message.Content,
		},
		Model:        result.Model,
		PromptTokens: result.PromptEvalCount,
		OutputTokens: result.EvalCount,
		TotalTokens:  result.PromptEvalCount + result.EvalCount,
		Duration:     time.Since(start),
		Done:         result.Done,
	}, nil
}

// =============================================================================
// OPENAI-COMPATIBLE PROVIDER
// =============================================================================

type openaiProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	client       *http.Client
	maxRetries   int
}

func newOpenAIProvider(cfg ProviderConfig) (*openaiProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	model := cfg.DefaultModel
	if model == "" {
		model = os.Getenv("OPENAI_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	return &openaiProvider{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		client:       &http.Client{Timeout: cfg.Timeout},
		maxRetries:   cfg.MaxRetries,
	}, nil
}

func (p *openaiProvider) Name() string { return "openai" }

func (p *openaiProvider) Models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai list models: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Data))
	for i, m := range result.Data {
		models[i] = m.ID
	}
	return models, nil
}

func (p *openaiProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	// OpenAI doesn't have a direct generate endpoint, use chat completions
	chatReq := ChatRequest{
		Messages:    []Message{{Role: "user", Content: req.Prompt}},
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}
	chatResp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return &GenerateResponse{
		Text:         chatResp.Message.Content,
		Model:        chatResp.Model,
		PromptTokens: chatResp.PromptTokens,
		OutputTokens: chatResp.OutputTokens,
		TotalTokens:  chatResp.TotalTokens,
		Duration:     chatResp.Duration,
		Done:         chatResp.Done,
	}, nil
}

func (p *openaiProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	messages := make([]map[string]string, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = map[string]string{
			"role":    m.Role,
			"content": m.Content,
		}
	}

	payload := map[string]any{
		"model":    model,
		"messages": messages,
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.TopP > 0 {
		payload["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		payload["stop"] = req.Stop
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai chat error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	return &ChatResponse{
		Message: Message{
			Role:    result.Choices[0].Message.Role,
			Content: result.Choices[0].Message.Content,
		},
		Model:        result.Model,
		PromptTokens: result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
		TotalTokens:  result.Usage.TotalTokens,
		Duration:     time.Since(start),
		Done:         result.Choices[0].FinishReason == "stop",
	}, nil
}

// =============================================================================
// ANTHROPIC PROVIDER
// =============================================================================

type anthropicProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	client       *http.Client
	maxRetries   int
}

func newAnthropicProvider(cfg ProviderConfig) (*anthropicProvider, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	model := cfg.DefaultModel
	if model == "" {
		model = os.Getenv("ANTHROPIC_MODEL")
	}
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	return &anthropicProvider{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		apiKey:       apiKey,
		defaultModel: model,
		client:       &http.Client{Timeout: cfg.Timeout},
		maxRetries:   cfg.MaxRetries,
	}, nil
}

func (p *anthropicProvider) Name() string { return "anthropic" }

func (p *anthropicProvider) Models(ctx context.Context) ([]string, error) {
	// Anthropic doesn't have a models endpoint, return known models
	return []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-5-haiku-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}, nil
}

func (p *anthropicProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	chatReq := ChatRequest{
		Messages:    []Message{{Role: "user", Content: req.Prompt}},
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}
	chatResp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return &GenerateResponse{
		Text:         chatResp.Message.Content,
		Model:        chatResp.Model,
		PromptTokens: chatResp.PromptTokens,
		OutputTokens: chatResp.OutputTokens,
		TotalTokens:  chatResp.TotalTokens,
		Duration:     chatResp.Duration,
		Done:         chatResp.Done,
	}, nil
}

func (p *anthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// Anthropic has different message format
	// System messages go in a separate field
	var systemPrompt string
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemPrompt = m.Content
			continue
		}
		messages = append(messages, map[string]string{
			"role":    m.Role,
			"content": m.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	payload := map[string]any{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.TopP > 0 {
		payload["top_p"] = req.TopP
	}
	if len(req.Stop) > 0 {
		payload["stop_sequences"] = req.Stop
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/messages", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic chat error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var content string
	for _, c := range result.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &ChatResponse{
		Message: Message{
			Role:    "assistant",
			Content: content,
		},
		Model:        result.Model,
		PromptTokens: result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		TotalTokens:  result.Usage.InputTokens + result.Usage.OutputTokens,
		Duration:     time.Since(start),
		Done:         result.StopReason == "end_turn",
	}, nil
}

// =============================================================================
// MOCK PROVIDER (for testing)
// =============================================================================

// MockProvider is a test provider that returns predictable responses.
type MockProvider struct {
	model        string
	GenerateFunc func(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
	ChatFunc     func(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) Models(ctx context.Context) ([]string, error) {
	return []string{"mock-model"}, nil
}

func (p *MockProvider) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	if p.GenerateFunc != nil {
		return p.GenerateFunc(ctx, req)
	}
	return &GenerateResponse{
		Text:         fmt.Sprintf("[mock] Generated response for: %.50s...", req.Prompt),
		Model:        "mock-model",
		PromptTokens: len(req.Prompt) / 4,
		OutputTokens: 20,
		TotalTokens:  len(req.Prompt)/4 + 20,
		Duration:     10 * time.Millisecond,
		Done:         true,
	}, nil
}

func (p *MockProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p.ChatFunc != nil {
		return p.ChatFunc(ctx, req)
	}
	lastMsg := ""
	if len(req.Messages) > 0 {
		lastMsg = req.Messages[len(req.Messages)-1].Content
	}
	return &ChatResponse{
		Message: Message{
			Role:    "assistant",
			Content: fmt.Sprintf("[mock] Response to: %.50s...", lastMsg),
		},
		Model:        "mock-model",
		PromptTokens: 50,
		OutputTokens: 20,
		TotalTokens:  70,
		Duration:     10 * time.Millisecond,
		Done:         true,
	}, nil
}
