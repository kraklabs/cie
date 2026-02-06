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

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
	"github.com/kraklabs/cie/internal/output"
	"github.com/kraklabs/cie/internal/ui"
)

// ConfigOutput represents the configuration for JSON output.
// It mirrors the Config struct but uses JSON tags appropriate for external consumption.
type ConfigOutput struct {
	ConfigPath string             `json:"config_path"`
	Version    string             `json:"version"`
	ProjectID  string             `json:"project_id"`
	CIE        CIEConfigOutput    `json:"cie"`
	Embedding  EmbeddingOutput    `json:"embedding"`
	Indexing   IndexingOutput     `json:"indexing"`
	Roles      *RolesConfigOutput `json:"roles,omitempty"`
}

// CIEConfigOutput represents CIE server configuration for JSON output.
type CIEConfigOutput struct {
	PrimaryHub string `json:"primary_hub,omitempty"`
	EdgeCache  string `json:"edge_cache,omitempty"`
}

// EmbeddingOutput represents embedding provider configuration for JSON output.
type EmbeddingOutput struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
	// APIKey is intentionally omitted from JSON output for security
}

// IndexingOutput represents indexing settings for JSON output.
type IndexingOutput struct {
	ParserMode  string   `json:"parser_mode"`
	BatchTarget int      `json:"batch_target"`
	MaxFileSize int64    `json:"max_file_size"`
	Exclude     []string `json:"exclude"`
}

// RolesConfigOutput represents custom role patterns for JSON output.
type RolesConfigOutput struct {
	Custom map[string]RolePatternOutput `json:"custom,omitempty"`
}

// RolePatternOutput represents a role pattern for JSON output.
type RolePatternOutput struct {
	FilePattern string `json:"file_pattern,omitempty"`
	NamePattern string `json:"name_pattern,omitempty"`
	CodePattern string `json:"code_pattern,omitempty"`
	Description string `json:"description,omitempty"`
}

// runConfig executes the 'config' CLI command, displaying current configuration.
//
// It loads the configuration file and displays its contents in either
// human-readable format (default) or JSON format (with --json flag).
//
// Global flags from main:
//   - --json: Output results as JSON (from globals.JSON)
//   - --quiet: Suppress non-essential output (from globals.Quiet)
//
// Examples:
//
//	cie config           Display formatted configuration
//	cie config --json    Output as JSON for programmatic use
func runConfig(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("config", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie config [options]

Description:
  Display the current CIE configuration including project settings,
  embedding provider, and indexing options.

  This reads the .cie/project.yaml configuration file and displays
  its contents. Environment variable overrides are applied.

  Note: API keys are never displayed for security reasons.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Show human-readable configuration
  cie config

  # Output as JSON for programmatic use
  cie config --json

  # Pipe to jq for specific field extraction
  cie config --json | jq '.project_id'
  cie config --json | jq '.embedding.model'

Output Fields:
  - config_path:    Path to the configuration file
  - version:        Configuration file version
  - project_id:     Project identifier
  - cie:            CIE server settings (primary_hub, edge_cache)
  - embedding:      Embedding provider settings (provider, base_url, model)
  - indexing:       Indexing settings (parser_mode, batch_target, exclude)
  - roles:          Custom role patterns (if defined)

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Find configuration file path
	var cfgPath string
	var err error

	if configPath != "" {
		cfgPath = configPath
	} else {
		// Use findConfigFile logic to locate the config
		cfgPath, err = findConfigPath()
		if err != nil {
			errors.FatalError(err, globals.JSON)
		}
	}

	// Make path absolute for display
	if !filepath.IsAbs(cfgPath) {
		if abs, absErr := filepath.Abs(cfgPath); absErr == nil {
			cfgPath = abs
		}
	}

	// Load configuration
	cfg, err := LoadConfig(configPath)
	if err != nil {
		errors.FatalError(err, globals.JSON)
	}

	// Build output structure
	result := buildConfigOutput(cfgPath, cfg)

	if globals.JSON {
		if err := output.JSON(result); err != nil {
			errors.FatalError(errors.NewInternalError(
				"Cannot encode configuration as JSON",
				"JSON encoding failed unexpectedly",
				"This is a bug. Please report it",
				err,
			), globals.JSON)
		}
	} else {
		printConfigHuman(result)
	}
}

// findConfigPath finds the configuration file path without loading it.
func findConfigPath() (string, error) {
	// Check for explicit config path from environment
	if cfgPath := os.Getenv("CIE_CONFIG_PATH"); cfgPath != "" {
		if _, err := os.Stat(cfgPath); err == nil {
			return cfgPath, nil
		}
		return "", errors.NewConfigError(
			"Configuration file not found",
			fmt.Sprintf("CIE_CONFIG_PATH is set to '%s' but the file does not exist", cfgPath),
			"Fix the CIE_CONFIG_PATH environment variable or run 'cie init' to create a config",
			nil,
		)
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", errors.NewInternalError(
			"Cannot access working directory",
			"Failed to determine current directory path",
			"Check system permissions and try again",
			err,
		)
	}

	for {
		cfgPath := ConfigPath(dir)
		if _, err := os.Stat(cfgPath); err == nil {
			return cfgPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", errors.NewConfigError(
		"Configuration not found",
		"No .cie/project.yaml file found in current directory or any parent directory",
		"Run 'cie init' to create a new configuration",
		nil,
	)
}

// buildConfigOutput converts a Config to ConfigOutput for JSON serialization.
func buildConfigOutput(configPath string, cfg *Config) *ConfigOutput {
	result := &ConfigOutput{
		ConfigPath: configPath,
		Version:    cfg.Version,
		ProjectID:  cfg.ProjectID,
		CIE: CIEConfigOutput{
			PrimaryHub: cfg.CIE.PrimaryHub,
			EdgeCache:  cfg.CIE.EdgeCache,
		},
		Embedding: EmbeddingOutput{
			Provider: cfg.Embedding.Provider,
			BaseURL:  cfg.Embedding.BaseURL,
			Model:    cfg.Embedding.Model,
		},
		Indexing: IndexingOutput{
			ParserMode:  cfg.Indexing.ParserMode,
			BatchTarget: cfg.Indexing.BatchTarget,
			MaxFileSize: cfg.Indexing.MaxFileSize,
			Exclude:     cfg.Indexing.Exclude,
		},
	}

	// Add roles if defined
	if len(cfg.Roles.Custom) > 0 {
		rolesOutput := &RolesConfigOutput{
			Custom: make(map[string]RolePatternOutput),
		}
		for name, pattern := range cfg.Roles.Custom {
			rolesOutput.Custom[name] = RolePatternOutput(pattern)
		}
		result.Roles = rolesOutput
	}

	return result
}

// printConfigHuman prints the configuration in human-readable format.
//
//nolint:gocognit // Configuration display has inherent complexity
func printConfigHuman(cfg *ConfigOutput) {
	ui.Header("CIE Configuration")
	fmt.Printf("%s  %s\n", ui.Label("Config File:"), ui.DimText(cfg.ConfigPath))
	fmt.Printf("%s     %s\n", ui.Label("Version:"), cfg.Version)
	fmt.Printf("%s  %s\n", ui.Label("Project ID:"), cfg.ProjectID)
	fmt.Println()

	// CIE Server (only show if configured)
	if cfg.CIE.PrimaryHub != "" || cfg.CIE.EdgeCache != "" {
		ui.SubHeader("CIE Server:")
		if cfg.CIE.PrimaryHub != "" {
			fmt.Printf("  Primary Hub:  %s\n", cfg.CIE.PrimaryHub)
		}
		if cfg.CIE.EdgeCache != "" {
			fmt.Printf("  Edge Cache:   %s\n", cfg.CIE.EdgeCache)
		}
		fmt.Println()
	}

	// Embedding
	ui.SubHeader("Embedding:")
	fmt.Printf("  Provider:     %s\n", cfg.Embedding.Provider)
	fmt.Printf("  Base URL:     %s\n", cfg.Embedding.BaseURL)
	fmt.Printf("  Model:        %s\n", cfg.Embedding.Model)
	fmt.Println()

	// Indexing
	ui.SubHeader("Indexing:")
	fmt.Printf("  Parser Mode:  %s\n", cfg.Indexing.ParserMode)
	fmt.Printf("  Batch Target: %d\n", cfg.Indexing.BatchTarget)
	fmt.Printf("  Max File:     %d bytes\n", cfg.Indexing.MaxFileSize)
	if len(cfg.Indexing.Exclude) > 0 {
		fmt.Printf("  Exclude:      %d patterns\n", len(cfg.Indexing.Exclude))
		for _, pattern := range cfg.Indexing.Exclude {
			fmt.Printf("                - %s\n", ui.DimText(pattern))
		}
	}

	// Roles (if defined)
	if cfg.Roles != nil && len(cfg.Roles.Custom) > 0 {
		fmt.Println()
		ui.SubHeader("Custom Roles:")
		for name, pattern := range cfg.Roles.Custom {
			fmt.Printf("  %s:\n", name)
			if pattern.FilePattern != "" {
				fmt.Printf("    file_pattern: %s\n", pattern.FilePattern)
			}
			if pattern.NamePattern != "" {
				fmt.Printf("    name_pattern: %s\n", pattern.NamePattern)
			}
			if pattern.CodePattern != "" {
				fmt.Printf("    code_pattern: %s\n", pattern.CodePattern)
			}
			if pattern.Description != "" {
				fmt.Printf("    description:  %s\n", ui.DimText(pattern.Description))
			}
		}
	}

}
