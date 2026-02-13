// Copyright 2025 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kraklabs/cie/internal/errors"
)

// dataRootFromConfig resolves the storage root with precedence:
// CIE_DATA_DIR > indexing.local_data_dir > ~/.cie/data.
func dataRootFromConfig(cfg *Config, configPath string) (string, error) {
	if envDir := os.Getenv("CIE_DATA_DIR"); envDir != "" {
		return absPath(envDir)
	}

	if cfg != nil && cfg.Indexing.LocalDataDir != "" {
		custom := cfg.Indexing.LocalDataDir
		if filepath.IsAbs(custom) {
			return filepath.Clean(custom), nil
		}

		cfgFilePath, err := resolvedConfigPath(configPath)
		if err == nil {
			baseDir := filepath.Dir(cfgFilePath)
			return filepath.Clean(filepath.Join(baseDir, custom)), nil
		}

		return absPath(custom)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.NewInternalError(
			"Cannot determine home directory",
			"Operating system did not provide user home directory path",
			"Check your system configuration or set HOME environment variable",
			err,
		)
	}
	return filepath.Join(home, ".cie", "data"), nil
}

// projectDataDir resolves the effective per-project data directory.
func projectDataDir(cfg *Config, configPath string) (string, error) {
	root, err := dataRootFromConfig(cfg, configPath)
	if err != nil {
		return "", err
	}
	if cfg == nil || cfg.ProjectID == "" {
		return root, nil
	}
	return filepath.Join(root, cfg.ProjectID), nil
}

// legacyDefaultProjectDataDir returns ~/.cie/data/<project_id> regardless of overrides.
func legacyDefaultProjectDataDir(projectID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cie", "data", projectID), nil
}

func resolvedConfigPath(configPath string) (string, error) {
	if configPath != "" {
		return absPath(configPath)
	}
	if envPath := os.Getenv("CIE_CONFIG_PATH"); envPath != "" {
		return absPath(envPath)
	}
	path, err := findConfigFile()
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return absPath(path)
}

func absPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
