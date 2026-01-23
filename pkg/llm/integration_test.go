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
