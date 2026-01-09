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

package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// DefaultProvider creates a provider from environment variables.
// Checks in order: OLLAMA_HOST, OPENAI_API_KEY, ANTHROPIC_API_KEY
// Falls back to mock if nothing is configured.
func DefaultProvider() (Provider, error) {
	// Check for Ollama first (local, free)
	if os.Getenv("OLLAMA_HOST") != "" || os.Getenv("OLLAMA_BASE_URL") != "" || os.Getenv("OLLAMA_MODEL") != "" {
		return NewProvider(ProviderConfig{Type: "ollama"})
	}

	// Check for OpenAI
	if os.Getenv("OPENAI_API_KEY") != "" {
		return NewProvider(ProviderConfig{Type: "openai"})
	}

	// Check for Anthropic
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return NewProvider(ProviderConfig{Type: "anthropic"})
	}

	// Default to mock for development
	return NewProvider(ProviderConfig{Type: "mock"})
}

// ProviderFromEnv creates a provider from a specific environment variable.
// Example: LLM_PROVIDER=ollama will use Ollama.
func ProviderFromEnv(envVar string) (Provider, error) {
	providerType := os.Getenv(envVar)
	if providerType == "" {
		return DefaultProvider()
	}
	return NewProvider(ProviderConfig{Type: providerType})
}

// QuickGenerate is a convenience function for simple text generation.
func QuickGenerate(ctx context.Context, prompt string) (string, error) {
	provider, err := DefaultProvider()
	if err != nil {
		return "", err
	}
	resp, err := provider.Generate(ctx, GenerateRequest{Prompt: prompt})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// QuickChat is a convenience function for simple chat.
func QuickChat(ctx context.Context, messages ...string) (string, error) {
	provider, err := DefaultProvider()
	if err != nil {
		return "", err
	}

	msgs := make([]Message, len(messages))
	for i, m := range messages {
		if i%2 == 0 {
			msgs[i] = Message{Role: "user", Content: m}
		} else {
			msgs[i] = Message{Role: "assistant", Content: m}
		}
	}

	resp, err := provider.Chat(ctx, ChatRequest{Messages: msgs})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

// CodePrompt helps build prompts for code-related tasks.
type CodePrompt struct {
	Task        string
	Language    string
	Code        string
	Context     string
	Constraints []string
}

// Build generates a formatted prompt for code tasks.
func (cp CodePrompt) Build() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Task: %s\n\n", cp.Task))

	if cp.Language != "" {
		sb.WriteString(fmt.Sprintf("Language: %s\n\n", cp.Language))
	}

	if cp.Context != "" {
		sb.WriteString(fmt.Sprintf("Context:\n%s\n\n", cp.Context))
	}

	if cp.Code != "" {
		sb.WriteString(fmt.Sprintf("Code:\n```%s\n%s\n```\n\n", cp.Language, cp.Code))
	}

	if len(cp.Constraints) > 0 {
		sb.WriteString("Constraints:\n")
		for _, c := range cp.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SystemPrompts contains common system prompts for code tasks.
var SystemPrompts = struct {
	CodeReview   string
	CodeExplain  string
	CodeRefactor string
	CodeGenerate string
	CodeDocument string
	CodeDebug    string
	CodeTest     string
}{
	CodeReview: `You are an expert code reviewer. Analyze the provided code for:
- Bugs and potential issues
- Security vulnerabilities
- Performance problems
- Code style and best practices
- Maintainability concerns
Provide specific, actionable feedback with line numbers when possible.`,

	CodeExplain: `You are a helpful programming tutor. Explain the provided code clearly and concisely.
Break down complex logic into understandable steps. Use analogies when helpful.
Identify key patterns and techniques used.`,

	CodeRefactor: `You are an expert software engineer specializing in code refactoring.
Improve the provided code while maintaining functionality. Focus on:
- Readability and clarity
- Performance optimizations
- Design patterns where appropriate
- Reducing complexity
Show before and after with explanations.`,

	CodeGenerate: `You are an expert programmer. Generate high-quality, production-ready code.
Follow best practices for the target language. Include:
- Clear variable and function names
- Appropriate error handling
- Comments for complex logic
- Type annotations where applicable`,

	CodeDocument: `You are a technical writer specializing in code documentation.
Generate clear, comprehensive documentation for the provided code including:
- Function/method descriptions
- Parameter explanations
- Return value descriptions
- Usage examples
- Edge cases and gotchas`,

	CodeDebug: `You are an expert debugger. Analyze the provided code and error message.
Identify the root cause of the issue and suggest fixes. Consider:
- Common pitfalls in the language
- Edge cases that might cause the error
- Potential race conditions or resource issues
Explain your reasoning step by step.`,

	CodeTest: `You are a QA engineer specializing in test automation.
Generate comprehensive tests for the provided code including:
- Unit tests for individual functions
- Edge cases and boundary conditions
- Error handling scenarios
- Mock objects where needed
Use the appropriate testing framework for the language.`,
}

// BuildChatMessages creates a chat message array with system prompt.
func BuildChatMessages(systemPrompt, userPrompt string, history ...Message) []Message {
	messages := make([]Message, 0, len(history)+2)
	messages = append(messages, Message{Role: "system", Content: systemPrompt})
	messages = append(messages, history...)
	messages = append(messages, Message{Role: "user", Content: userPrompt})
	return messages
}
