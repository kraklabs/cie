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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir  = ".cie"
	defaultConfigFile = "project.yaml"
	configVersion     = "1"
)

// Config represents the .cie/project.yaml configuration file.
type Config struct {
	Version   string          `yaml:"version"`
	ProjectID string          `yaml:"project_id"`
	CIE       CIEConfig       `yaml:"cie"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	Indexing  IndexingConfig  `yaml:"indexing"`
	Roles     RolesConfig     `yaml:"roles,omitempty"` // Custom role patterns
	LLM       LLMConfig       `yaml:"llm,omitempty"`   // LLM for narrative generation
}

// CIEConfig contains CIE server configuration.
type CIEConfig struct {
	PrimaryHub string `yaml:"primary_hub"` // gRPC address for writes
	EdgeCache  string `yaml:"edge_cache"`  // HTTP URL for queries
}

// EmbeddingConfig contains embedding provider configuration.
type EmbeddingConfig struct {
	Provider string `yaml:"provider"` // ollama, nomic, openai, mock
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	APIKey   string `yaml:"api_key,omitempty"` // API key (optional for local models)
}

// IndexingConfig contains indexing settings.
type IndexingConfig struct {
	ParserMode  string   `yaml:"parser_mode"`   // auto, treesitter
	BatchTarget int      `yaml:"batch_target"`  // mutations per batch
	MaxFileSize int64    `yaml:"max_file_size"` // bytes
	Exclude     []string `yaml:"exclude"`       // glob patterns
}

// RolesConfig contains custom role pattern definitions.
type RolesConfig struct {
	// Custom role patterns for this project
	// Key is role name, value is RolePattern
	Custom map[string]RolePattern `yaml:"custom"`
}

// RolePattern defines how to identify a role in code.
type RolePattern struct {
	// FilePattern is a regex to match file paths (e.g., ".*/routes/.*\\.go")
	FilePattern string `yaml:"file_pattern,omitempty"`
	// NamePattern is a regex to match function names (e.g., ".*Handler$")
	NamePattern string `yaml:"name_pattern,omitempty"`
	// CodePattern is a regex to match code content (e.g., "\\.GET\\(")
	CodePattern string `yaml:"code_pattern,omitempty"`
	// Description explains what this role represents
	Description string `yaml:"description,omitempty"`
}

// LLMConfig holds LLM provider settings for narrative generation in analyze.
type LLMConfig struct {
	Enabled   bool   `yaml:"enabled"`              // Enable LLM narrative generation
	BaseURL   string `yaml:"base_url"`             // OpenAI-compatible API URL
	Model     string `yaml:"model"`                // Model name
	APIKey    string `yaml:"api_key,omitempty"`    // API key (optional for local models)
	MaxTokens int    `yaml:"max_tokens,omitempty"` // Max tokens for response (default: 2000)
}

// DefaultConfig returns a config with sensible defaults for local development.
//
// The default configuration uses localhost URLs for both Primary Hub and Edge Cache,
// and configures Ollama as the embedding provider. Environment variables can override
// these defaults after the config is loaded.
//
// Parameters:
//   - projectID: Project identifier (typically the directory name)
//
// Returns a Config struct with default values for all fields.
func DefaultConfig(projectID string) *Config {
	return &Config{
		Version:   configVersion,
		ProjectID: projectID,
		CIE: CIEConfig{
			PrimaryHub: getEnv("CIE_PRIMARY_HUB", "localhost:50051"),
			EdgeCache:  getEnv("CIE_BASE_URL", "http://localhost:8080"),
		},
		Embedding: EmbeddingConfig{
			Provider: "ollama",
			BaseURL:  getEnv("OLLAMA_HOST", "http://localhost:11434"),
			Model:    getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text"),
		},
		Indexing: IndexingConfig{
			ParserMode:  "auto",
			BatchTarget: 500,     // Smaller batches for stability over slow networks
			MaxFileSize: 1048576, // 1MB
			Exclude: []string{
				".git/**",
				"node_modules/**",
				"vendor/**",
				"dist/**",
				"build/**",
				"*.o",
				"*.so",
				"*.dylib",
				"*.exe",
			},
		},
	}
}

// LoadConfig loads configuration from the specified path or finds it automatically.
//
// If configPath is empty, it searches for .cie/project.yaml in the current directory
// and parent directories. The CIE_CONFIG_PATH environment variable can override the
// search path.
//
// After loading, environment variables are applied to override file-based configuration.
//
// Parameters:
//   - configPath: Path to config file (empty string to auto-detect)
//
// Returns the loaded and merged configuration, or an error if the file cannot be
// found, read, or parsed.
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		// Find .cie/project.yaml in current or parent directories
		var err error
		configPath, err = findConfigFile()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Validate version
	if cfg.Version != configVersion {
		return nil, fmt.Errorf("unsupported config version: %s (expected %s)", cfg.Version, configVersion)
	}

	// Override with environment variables if set
	cfg.applyEnvOverrides()

	return &cfg, nil
}

// SaveConfig writes the configuration to the specified path as YAML.
//
// It creates the .cie directory if it doesn't exist, marshals the config to YAML,
// and writes it to disk with permissions 0644.
//
// Parameters:
//   - cfg: Configuration to save
//   - configPath: Absolute path where the config file should be written
//
// Returns an error if marshaling, directory creation, or file writing fails.
func SaveConfig(cfg *Config, configPath string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// ConfigPath returns the path to the config file in the given directory.
//
// Constructs the path as <dir>/.cie/project.yaml.
//
// Parameters:
//   - dir: Base directory (typically the repository root)
//
// Returns the absolute path to the config file.
func ConfigPath(dir string) string {
	return filepath.Join(dir, defaultConfigDir, defaultConfigFile)
}

// ConfigDir returns the path to the .cie directory in the given directory.
//
// Constructs the path as <dir>/.cie.
//
// Parameters:
//   - dir: Base directory (typically the repository root)
//
// Returns the absolute path to the .cie directory.
func ConfigDir(dir string) string {
	return filepath.Join(dir, defaultConfigDir)
}

// findConfigFile searches for .cie/project.yaml in current and parent directories.
//
// The search algorithm:
//  1. If CIE_CONFIG_PATH is set, use that path directly
//  2. Otherwise, start from current directory and walk up to filesystem root
//  3. In each directory, check for .cie/project.yaml
//  4. Return the first match found
//
// Returns the absolute path to the config file, or an error if not found.
func findConfigFile() (string, error) {
	// Check for explicit config path from environment
	if configPath := os.Getenv("CIE_CONFIG_PATH"); configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		return "", fmt.Errorf("config file not found at CIE_CONFIG_PATH=%s", configPath)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		configPath := ConfigPath(dir)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no .cie/project.yaml found in current or parent directories; run 'cie init' first")
}

// applyEnvOverrides applies environment variable overrides to the configuration.
//
// Environment variables take precedence over file-based configuration. This allows
// users to override settings without modifying the .cie/project.yaml file.
//
// Supported environment variables:
//   - CIE_PROJECT_ID: Override project identifier
//   - CIE_PRIMARY_HUB: Override Primary Hub gRPC address
//   - CIE_BASE_URL: Override Edge Cache HTTP URL
//   - OLLAMA_HOST: Override Ollama base URL
//   - OLLAMA_EMBED_MODEL: Override embedding model
//   - CIE_LLM_URL: Enable LLM and set API URL
//   - CIE_LLM_MODEL: Set LLM model name
//   - CIE_LLM_API_KEY: Set LLM API key
func (c *Config) applyEnvOverrides() {
	if url := os.Getenv("CIE_BASE_URL"); url != "" {
		c.CIE.EdgeCache = url
	}
	if url := os.Getenv("CIE_PRIMARY_HUB"); url != "" {
		c.CIE.PrimaryHub = url
	}
	if id := os.Getenv("CIE_PROJECT_ID"); id != "" {
		c.ProjectID = id
	}
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		c.Embedding.BaseURL = host
	}
	if model := os.Getenv("OLLAMA_EMBED_MODEL"); model != "" {
		c.Embedding.Model = model
	}
	// LLM overrides
	if url := os.Getenv("CIE_LLM_URL"); url != "" {
		c.LLM.BaseURL = url
		c.LLM.Enabled = true
	}
	if model := os.Getenv("CIE_LLM_MODEL"); model != "" {
		c.LLM.Model = model
	}
	if key := os.Getenv("CIE_LLM_API_KEY"); key != "" {
		c.LLM.APIKey = key
	}
}

// getEnv retrieves an environment variable or returns a fallback value if not set.
//
// Parameters:
//   - key: Environment variable name
//   - fallback: Value to return if the environment variable is not set or empty
//
// Returns the environment variable value or the fallback.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
