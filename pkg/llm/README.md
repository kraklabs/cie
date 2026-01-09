# LLM Provider Interface

Generic interface for Large Language Model providers. Supports multiple backends with a unified API.

## Supported Providers

| Provider | Type | Local/Cloud | Models |
|----------|------|-------------|--------|
| **Ollama** | `ollama` | Local | qwen2.5-coder, llama3.2, codellama, deepseek-coder, etc. |
| **OpenAI** | `openai` | Cloud | gpt-4o, gpt-4o-mini, o1, etc. |
| **Anthropic** | `anthropic` | Cloud | claude-3.5-sonnet, claude-3-opus, etc. |
| **OpenAI-compatible** | `openai` | Either | Any OpenAI-compatible API (vLLM, LocalAI, etc.) |
| **Mock** | `mock` | Local | Testing only |

## Quick Start

### Using Ollama (Recommended for Local)

```bash
# Install a code model
ollama pull qwen2.5-coder:7b     # 4.7GB - Fast, good for code
ollama pull qwen2.5-coder:32b    # 19GB - Better quality (needs RTX 3090)
ollama pull deepseek-coder:6.7b  # 3.8GB - Alternative
ollama pull codellama:13b        # 7.3GB - Meta's code model
```

### Go Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/kraklabs/kraken/internal/cie/llm"
)

func main() {
    // Create provider (auto-detects from environment)
    provider, err := llm.DefaultProvider()
    if err != nil {
        panic(err)
    }

    // Simple text generation
    ctx := context.Background()
    resp, err := provider.Generate(ctx, llm.GenerateRequest{
        Prompt: "Write a Go function to reverse a string",
        Model:  "qwen2.5-coder:7b", // Optional if default is set
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(resp.Text)

    // Multi-turn chat
    chatResp, err := provider.Chat(ctx, llm.ChatRequest{
        Messages: []llm.Message{
            {Role: "system", Content: llm.SystemPrompts.CodeGenerate},
            {Role: "user", Content: "Write a function to parse JSON in Go"},
        },
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(chatResp.Message.Content)
}
```

## Configuration

### Environment Variables

```bash
# Ollama (local)
export OLLAMA_HOST=http://localhost:11434
export OLLAMA_MODEL=qwen2.5-coder:7b

# OpenAI
export OPENAI_API_KEY=sk-...
export OPENAI_MODEL=gpt-4o-mini

# Anthropic
export ANTHROPIC_API_KEY=sk-ant-...
export ANTHROPIC_MODEL=claude-3-5-sonnet-20241022

# OpenAI-compatible (e.g., vLLM, LocalAI)
export OPENAI_BASE_URL=http://localhost:8000/v1
export OPENAI_API_KEY=dummy  # Some servers require any key
export OPENAI_MODEL=Qwen/Qwen2.5-Coder-32B-Instruct
```

### Programmatic Configuration

```go
// Ollama with custom settings
provider, _ := llm.NewProvider(llm.ProviderConfig{
    Type:         "ollama",
    BaseURL:      "http://gpu-server:11434",
    DefaultModel: "qwen2.5-coder:32b",
    Timeout:      5 * time.Minute,
    MaxRetries:   3,
})

// OpenAI-compatible API (e.g., vLLM on remote server)
provider, _ := llm.NewProvider(llm.ProviderConfig{
    Type:         "openai",
    BaseURL:      "http://fedora-server:8000/v1",
    APIKey:       "dummy",
    DefaultModel: "Qwen/Qwen2.5-Coder-32B-Instruct",
})
```

## Code-Specific Helpers

### Code Prompts

```go
prompt := llm.CodePrompt{
    Task:     "Review this code for security vulnerabilities",
    Language: "go",
    Code:     `func HandleUser(w http.ResponseWriter, r *http.Request) {
        id := r.URL.Query().Get("id")
        user, _ := db.Query("SELECT * FROM users WHERE id = " + id)
        json.NewEncoder(w).Encode(user)
    }`,
    Context:  "HTTP handler for user API",
    Constraints: []string{
        "Focus on SQL injection",
        "Check for error handling",
    },
}.Build()
```

### System Prompts

Pre-built system prompts for common tasks:

```go
llm.SystemPrompts.CodeReview    // Code review and bug detection
llm.SystemPrompts.CodeExplain   // Code explanation
llm.SystemPrompts.CodeRefactor  // Refactoring suggestions
llm.SystemPrompts.CodeGenerate  // Code generation
llm.SystemPrompts.CodeDocument  // Documentation generation
llm.SystemPrompts.CodeDebug     // Debugging assistance
llm.SystemPrompts.CodeTest      // Test generation
```

## Architecture Examples

### Hybrid: Mac + Fedora Server

```
┌─────────────────────────┐         ┌─────────────────────────┐
│   Mac M1 Max (32GB)     │         │  Fedora (RTX 3090)      │
│                         │   API   │                         │
│  ┌───────────────────┐  │◄────────│  ┌───────────────────┐  │
│  │ CIE Stack         │  │         │  │ Ollama            │  │
│  │ + Embeddings      │  │         │  │ qwen2.5-coder:32b │  │
│  │ (nomic-embed-text)│  │         │  └───────────────────┘  │
│  └───────────────────┘  │         │                         │
└─────────────────────────┘         └─────────────────────────┘
```

```go
// On Mac, connect to Fedora for LLM
provider, _ := llm.NewProvider(llm.ProviderConfig{
    Type:         "ollama",
    BaseURL:      "http://fedora-server:11434",
    DefaultModel: "qwen2.5-coder:32b",
})
```

### Cloud Hybrid

```go
// Use local embeddings + cloud LLM
embeddingProvider, _ := embedding.NewProvider("ollama", nil)

llmProvider, _ := llm.NewProvider(llm.ProviderConfig{
    Type:   "anthropic",
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
})
```

## Model Recommendations

### For RTX 3090 (24GB VRAM)

| Use Case | Model | VRAM | Quality |
|----------|-------|------|---------|
| Fast code completion | `qwen2.5-coder:7b` | ~5GB | Good |
| Balanced | `qwen2.5-coder:14b` | ~10GB | Better |
| Best local quality | `qwen2.5-coder:32b` | ~20GB | Excellent |
| Code explanation | `deepseek-coder:33b` | ~20GB | Excellent |

### For Mac M1 Max (32GB)

| Use Case | Model | Memory | Notes |
|----------|-------|--------|-------|
| Fast responses | `qwen2.5-coder:7b` | ~5GB | Native Metal |
| Embeddings | `nomic-embed-text` | ~300MB | Fast |
| Quality | `llama3.2:8b` | ~5GB | Good general |

## Testing

```bash
# Run unit tests
go test ./internal/cie/llm/... -v

# Test with real Ollama (requires running server + model)
OLLAMA_MODEL=qwen2.5-coder:7b go test ./internal/cie/llm/... -v -tags=integration
```
