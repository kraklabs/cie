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

//go:build integration
// +build integration

package llm

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFedoraServer_Integration(t *testing.T) {
	serverURL := os.Getenv("LLM_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://100.117.59.45:8000/v1"
	}

	provider, err := NewProvider(ProviderConfig{
		Type:         "openai",
		BaseURL:      serverURL,
		DefaultModel: "qwen2.5-coder-32b-instruct-q5_k_m.gguf",
		Timeout:      2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewProvider error: %v", err)
	}

	t.Logf("Provider: %s", provider.Name())

	ctx := context.Background()
	resp, err := provider.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a helpful coding assistant. Be concise."},
			{Role: "user", Content: "What is 2+2? Answer with just the number."},
		},
		MaxTokens:   10,
		Temperature: 0.1,
	})
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	t.Logf("Response: %s", resp.Message.Content)
	t.Logf("Tokens: %d prompt + %d output = %d total", resp.PromptTokens, resp.OutputTokens, resp.TotalTokens)
	t.Logf("Duration: %v", resp.Duration)
}
