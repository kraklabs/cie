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

// Package llm provides a unified interface for Large Language Model providers.
//
// This package abstracts the differences between various LLM APIs, providing
// a consistent interface for text generation and chat completions. It is used
// by CIE's analyze tool to generate natural language responses about code.
//
// # Supported Providers
//
// The following LLM providers are supported:
//   - Ollama: Local models, no API key required (default)
//   - OpenAI: GPT-4, GPT-4o-mini, and OpenAI-compatible APIs
//   - Anthropic: Claude models
//   - Mock: For testing without real API calls
//
// # Quick Start
//
// Create a provider explicitly:
//
//	provider, err := llm.NewProvider(llm.ProviderConfig{
//	    Type:   "openai",
//	    APIKey: os.Getenv("OPENAI_API_KEY"),
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	resp, err := provider.Generate(ctx, llm.GenerateRequest{
//	    Prompt: "Explain this Go code: ...",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(resp.Text)
//
// Or use convenience functions that auto-detect the provider:
//
//	response, err := llm.QuickGenerate(ctx, "Explain this code")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(response)
//
// # Chat Completions
//
// For multi-turn conversations, use the Chat method:
//
//	messages := []llm.Message{
//	    {Role: "system", Content: "You are a helpful code assistant."},
//	    {Role: "user", Content: "What does this function do?"},
//	}
//
//	resp, err := provider.Chat(ctx, llm.ChatRequest{
//	    Messages: messages,
//	})
//
// Or use the BuildChatMessages helper:
//
//	messages := llm.BuildChatMessages(
//	    llm.SystemPrompts.CodeExplain, // system prompt
//	    "What does this function do?", // user prompt
//	)
//
// # Provider Selection
//
// The [DefaultProvider] function automatically selects a provider based on
// available environment variables, checking in order:
//  1. OLLAMA_HOST or OLLAMA_MODEL set - Uses Ollama (local)
//  2. OPENAI_API_KEY set - Uses OpenAI
//  3. ANTHROPIC_API_KEY set - Uses Anthropic
//  4. No credentials - Falls back to mock provider
//
// # Environment Variables
//
// Ollama (local, free):
//   - OLLAMA_HOST: Server URL (default: http://localhost:11434)
//   - OLLAMA_MODEL: Model name (e.g., "llama2", "codellama")
//
// OpenAI:
//   - OPENAI_API_KEY: API key (required)
//   - OPENAI_BASE_URL: API URL for compatible services (e.g., Azure)
//   - OPENAI_MODEL: Model name (default: gpt-4o-mini)
//
// Anthropic:
//   - ANTHROPIC_API_KEY: API key (required)
//   - ANTHROPIC_MODEL: Model name (default: claude-3-5-sonnet-20241022)
//
// # Code Analysis Helpers
//
// The package provides pre-built system prompts for common code tasks:
//
//	// Use predefined prompts
//	messages := llm.BuildChatMessages(
//	    llm.SystemPrompts.CodeReview,  // or CodeExplain, CodeDebug, etc.
//	    "Review this function for bugs...",
//	)
//
// Or build structured code prompts:
//
//	prompt := llm.CodePrompt{
//	    Task:     "Explain this function",
//	    Language: "go",
//	    Code:     "func main() { ... }",
//	}.Build()
//
// # Error Handling
//
// All provider methods return descriptive errors that include context about
// the failure. Network errors, API errors, and validation errors are all
// wrapped with appropriate context.
//
//	resp, err := provider.Generate(ctx, req)
//	if err != nil {
//	    // Error includes provider name and context
//	    // e.g., "openai chat error (status 401): invalid api key"
//	    return err
//	}
package llm
