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
// Package tools provides shared CIE tool implementations that can be used
// by both the MCP server and the agent.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kraklabs/cie/pkg/llm"
)

// Querier is the interface for executing CIE queries.
// Both CIEClient (HTTP) and TestCIEClient (embedded CozoDB) implement this.
type Querier interface {
	Query(ctx context.Context, script string) (*QueryResult, error)
	QueryRaw(ctx context.Context, script string) (map[string]any, error)
}

// CIEClient provides access to the CIE Edge Cache API.
type CIEClient struct {
	BaseURL        string
	ProjectID      string
	HTTPClient     *http.Client
	LLMClient      llm.Provider // Optional LLM for narrative generation
	LLMMaxTokens   int          // Max tokens for LLM responses (default: 2000)
	EmbeddingURL   string       // Ollama URL for embeddings (e.g., http://localhost:11434)
	EmbeddingModel string       // Embedding model name (e.g., nomic-embed-text)
}

// NewCIEClient creates a new CIE client.
func NewCIEClient(baseURL, projectID string) *CIEClient {
	return &CIEClient{
		BaseURL:   baseURL,
		ProjectID: projectID,
		HTTPClient: &http.Client{
			Timeout: 90 * time.Second, // Increased for large HNSW queries (k=2000)
		},
	}
}

// QueryResult represents the response from a CIE query.
type QueryResult struct {
	Headers []string `json:"Headers"`
	Rows    [][]any  `json:"Rows"`
}

// Query executes a CozoScript query against the CIE Edge Cache.
func (c *CIEClient) Query(ctx context.Context, script string) (*QueryResult, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"project_id": c.ProjectID,
		"script":     script,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/query", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query error (status %d): %s", resp.StatusCode, string(body))
	}

	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

// QueryRaw returns the raw result as map for MCP compatibility.
func (c *CIEClient) QueryRaw(ctx context.Context, script string) (map[string]any, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"project_id": c.ProjectID,
		"script":     script,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/v1/query", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query error (status %d): %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result, nil
}

// SetLLMProvider configures an LLM provider for narrative generation.
func (c *CIEClient) SetLLMProvider(provider llm.Provider, maxTokens int) {
	c.LLMClient = provider
	c.LLMMaxTokens = maxTokens
	if c.LLMMaxTokens <= 0 {
		c.LLMMaxTokens = 2000 // Default
	}
}

// SetEmbeddingConfig configures embedding provider for semantic search.
func (c *CIEClient) SetEmbeddingConfig(url, model string) {
	c.EmbeddingURL = url
	c.EmbeddingModel = model
}
