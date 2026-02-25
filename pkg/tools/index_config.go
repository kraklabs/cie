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

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kraklabs/cie/pkg/storage"
	"gopkg.in/yaml.v3"
)

// IndexConfigArgs contains arguments for the IndexConfig tool.
type IndexConfigArgs struct {
	// AutoReindex enables or disables file watching for auto-reindex
	AutoReindex *bool

	// DebounceMs sets the delay before triggering reindex after file change
	DebounceMs *int

	// ExcludePatterns sets glob patterns to ignore during file watching
	ExcludePatterns []string

	// WatchExtensions sets file extensions to watch (e.g., ".go", ".js")
	WatchExtensions []string

	// ConfigPath is the path to save configuration (defaults to .cie/project.yaml)
	ConfigPath string

	// ProjectRoot is the project root directory
	ProjectRoot string

	// GetOnly if true, only returns current config without modifying
	GetOnly bool
}

// IndexConfigResult contains the current configuration.
type IndexConfigResult struct {
	AutoReindex     bool     `yaml:"auto_reindex" json:"auto_reindex"`
	DebounceMs      int      `yaml:"debounce_ms" json:"debounce_ms"`
	ExcludePatterns []string `yaml:"exclude_patterns" json:"exclude_patterns"`
	WatchExtensions []string `yaml:"watch_extensions" json:"watch_extensions"`
	ConfigPath      string   `yaml:"-" json:"config_path"`
}

// IndexConfig manages auto-reindex configuration.
// It persists settings to .cie/project.yaml and updates the running configuration.
func IndexConfig(args IndexConfigArgs) (*ToolResult, error) {
	coordinator := storage.GetReindexCoordinator()

	// Determine config path
	configPath := args.ConfigPath
	if configPath == "" {
		projectRoot := args.ProjectRoot
		if projectRoot == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("cannot determine working directory: %w", err)
			}
			projectRoot = cwd
		}
		configPath = filepath.Join(projectRoot, ".cie", "project.yaml")
	}

	// Load existing auto-reindex config if present
	existingConfig := loadAutoReindexConfig(configPath)

	if args.GetOnly {
		// Just return current config
		return formatConfigResult(existingConfig, configPath), nil
	}

	// Apply changes
	modified := false

	if args.AutoReindex != nil {
		existingConfig.AutoReindex = *args.AutoReindex
		modified = true
	}

	if args.DebounceMs != nil {
		if *args.DebounceMs < 100 {
			*args.DebounceMs = 100 // Minimum 100ms
		}
		if *args.DebounceMs > 60000 {
			*args.DebounceMs = 60000 // Maximum 60s
		}
		existingConfig.DebounceMs = *args.DebounceMs
		modified = true
	}

	if len(args.ExcludePatterns) > 0 {
		existingConfig.ExcludePatterns = args.ExcludePatterns
		modified = true
	}

	if len(args.WatchExtensions) > 0 {
		// Validate extensions start with "."
		for i, ext := range args.WatchExtensions {
			if !strings.HasPrefix(ext, ".") {
				args.WatchExtensions[i] = "." + ext
			}
		}
		existingConfig.WatchExtensions = args.WatchExtensions
		modified = true
	}

	// Save to file
	if modified {
		if err := saveAutoReindexConfig(configPath, existingConfig); err != nil {
			return nil, fmt.Errorf("failed to save configuration: %w", err)
		}

		// Update the running coordinator
		coordinator.SetConfig(existingConfig)
	}

	return formatConfigResult(existingConfig, configPath), nil
}

// loadAutoReindexConfig loads auto-reindex configuration from project.yaml.
// Uses the main.Config structure for proper parsing.
func loadAutoReindexConfig(configPath string) storage.ReindexConfig {
	// Start with defaults
	config := storage.DefaultReindexConfig()

	// Read existing file if present
	data, err := os.ReadFile(configPath) //nolint:gosec // G304: Path validated by caller
	if err != nil {
		return config // File doesn't exist, return defaults
	}

	// Parse YAML to find auto_reindex section
	var root map[string]interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return config // Parse error, return defaults
	}

	// Extract auto_reindex config
	if autoReindexRaw, ok := root["auto_reindex"]; ok {
		if autoReindex, ok := autoReindexRaw.(map[string]interface{}); ok {
			if v, ok := autoReindex["enabled"].(bool); ok {
				config.AutoReindex = v
			}
			// Handle debounce_ms (could be int or float64 from YAML)
			if v, ok := autoReindex["debounce_ms"].(int); ok {
				config.DebounceMs = v
			} else if v, ok := autoReindex["debounce_ms"].(float64); ok {
				config.DebounceMs = int(v)
			}
			if patternsRaw, ok := autoReindex["exclude_patterns"].([]interface{}); ok {
				config.ExcludePatterns = make([]string, 0, len(patternsRaw))
				for _, p := range patternsRaw {
					if s, ok := p.(string); ok {
						config.ExcludePatterns = append(config.ExcludePatterns, s)
					}
				}
			}
			if extsRaw, ok := autoReindex["watch_extensions"].([]interface{}); ok {
				config.WatchExtensions = make([]string, 0, len(extsRaw))
				for _, e := range extsRaw {
					if s, ok := e.(string); ok {
						config.WatchExtensions = append(config.WatchExtensions, s)
					}
				}
			}
		}
	}

	return config
}

// saveAutoReindexConfig saves auto-reindex configuration to project.yaml.
func saveAutoReindexConfig(configPath string, config storage.ReindexConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Read existing file or create new
	var root map[string]interface{}
	data, err := os.ReadFile(configPath) //nolint:gosec // G304: Path validated by caller
	if err == nil {
		yaml.Unmarshal(data, &root) //nolint:errcheck // Continue with empty root on error
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	// Set required fields if not present
	if _, ok := root["version"]; !ok {
		root["version"] = "1"
	}
	if _, ok := root["project_id"]; !ok {
		root["project_id"] = filepath.Base(dir)
	}

	// Update auto_reindex section
	root["auto_reindex"] = map[string]interface{}{
		"enabled":           config.AutoReindex,
		"debounce_ms":       config.DebounceMs,
		"exclude_patterns":  config.ExcludePatterns,
		"watch_extensions":  config.WatchExtensions,
	}

	// Marshal and write
	output, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// formatConfigResult formats the configuration result for display.
func formatConfigResult(config storage.ReindexConfig, configPath string) *ToolResult {
	status := "disabled"
	if config.AutoReindex {
		status = "enabled"
	}

	text := fmt.Sprintf(
		"# Auto-Reindex Configuration\n\n"+
			"**Status:** %s\n"+
			"**Config File:** `%s`\n\n"+
			"## Settings\n\n"+
			"| Setting | Value |\n"+
			"|---------|-------|\n"+
			"| Auto-reindex | %v |\n"+
			"| Debounce | %d ms |\n",
		status,
		configPath,
		config.AutoReindex,
		config.DebounceMs,
	)

	// Exclude patterns
	text += "\n### Exclude Patterns\n"
	if len(config.ExcludePatterns) == 0 {
		text += "_No custom exclude patterns (using defaults)_\n"
	} else {
		text += "```\n"
		for _, p := range config.ExcludePatterns {
			text += fmt.Sprintf("- %s\n", p)
		}
		text += "```\n"
	}

	// Watch extensions
	text += "\n### Watched Extensions\n"
	if len(config.WatchExtensions) == 0 {
		text += "_Watching all supported code files_\n"
	} else {
		text += "```\n"
		for _, ext := range config.WatchExtensions {
			text += fmt.Sprintf("%s ", ext)
		}
		text += "\n```\n"
	}

	// Usage help
	text += "\n## Usage\n\n"
	text += "**Enable auto-reindex:**\n"
	text += "```\ncie_index_config(auto_reindex=true)\n```\n\n"
	text += "**Change debounce delay:**\n"
	text += "```\ncie_index_config(debounce_ms=5000)\n```\n\n"
	text += "**Add exclude patterns:**\n"
	text += "```\ncie_index_config(exclude_patterns=[\"*.log\", \"temp/**\"])\n```\n"

	return NewResult(text)
}

// GetAutoReindexConfig returns the current auto-reindex configuration.
func GetAutoReindexConfig(configPath string) storage.ReindexConfig {
	return loadAutoReindexConfig(configPath)
}
