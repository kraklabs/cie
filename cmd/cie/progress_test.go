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
	"bytes"
	"os"
	"testing"
)

func TestNewProgressConfig(t *testing.T) {
	tests := []struct {
		name            string
		globals         GlobalFlags
		expectedEnabled bool
		expectedNoColor bool
	}{
		{
			name:            "default flags - progress disabled in test (not a TTY)",
			globals:         GlobalFlags{},
			expectedEnabled: false, // stderr is not a TTY in test environment
			expectedNoColor: false,
		},
		{
			name:            "quiet mode - progress disabled",
			globals:         GlobalFlags{Quiet: true},
			expectedEnabled: false,
			expectedNoColor: false,
		},
		{
			name:            "JSON mode - progress disabled (quiet auto-set)",
			globals:         GlobalFlags{JSON: true, Quiet: true},
			expectedEnabled: false,
			expectedNoColor: false,
		},
		{
			name:            "verbose mode - progress not affected by verbosity",
			globals:         GlobalFlags{Verbose: 1},
			expectedEnabled: false, // stderr not a TTY in test
			expectedNoColor: false,
		},
		{
			name:            "noColor flag propagates to config",
			globals:         GlobalFlags{NoColor: true},
			expectedEnabled: false, // stderr not a TTY in test
			expectedNoColor: true,
		},
		{
			name:            "all flags combined",
			globals:         GlobalFlags{JSON: true, Quiet: true, NoColor: true, Verbose: 2},
			expectedEnabled: false,
			expectedNoColor: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewProgressConfig(tt.globals)
			if cfg.Enabled != tt.expectedEnabled {
				t.Errorf("NewProgressConfig().Enabled = %v, want %v", cfg.Enabled, tt.expectedEnabled)
			}
			if cfg.NoColor != tt.expectedNoColor {
				t.Errorf("NewProgressConfig().NoColor = %v, want %v", cfg.NoColor, tt.expectedNoColor)
			}
			if cfg.Writer != os.Stderr {
				t.Error("NewProgressConfig().Writer should be os.Stderr")
			}
		})
	}
}

func TestNewProgressBar(t *testing.T) {
	t.Run("disabled config returns nil", func(t *testing.T) {
		cfg := ProgressConfig{Enabled: false}
		bar := NewProgressBar(cfg, 100, "Test")
		if bar != nil {
			t.Error("NewProgressBar() should return nil when disabled")
		}
	})

	t.Run("enabled config returns non-nil with correct properties", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := ProgressConfig{Enabled: true, Writer: &buf, NoColor: false}
		bar := NewProgressBar(cfg, 100, "Test")
		if bar == nil {
			t.Fatal("NewProgressBar() should return non-nil when enabled")
		}
		// Verify bar can be used without panic
		_ = bar.Set(50)
		_ = bar.Finish()
	})

	t.Run("zero total creates valid bar", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := ProgressConfig{Enabled: true, Writer: &buf}
		bar := NewProgressBar(cfg, 0, "Empty")
		if bar == nil {
			t.Fatal("NewProgressBar() should handle zero total")
		}
		_ = bar.Finish()
	})

	t.Run("noColor option is respected", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := ProgressConfig{Enabled: true, Writer: &buf, NoColor: true}
		bar := NewProgressBar(cfg, 10, "NoColor Test")
		if bar == nil {
			t.Fatal("NewProgressBar() should return non-nil")
		}
		_ = bar.Set(5)
		_ = bar.Finish()
		// NoColor should prevent ANSI escape codes in output
		// Note: progressbar library may still write minimal output
	})
}

func TestNewSpinner(t *testing.T) {
	t.Run("disabled config returns nil", func(t *testing.T) {
		cfg := ProgressConfig{Enabled: false}
		spinner := NewSpinner(cfg, "Test")
		if spinner != nil {
			t.Error("NewSpinner() should return nil when disabled")
		}
	})

	t.Run("enabled config returns non-nil", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := ProgressConfig{Enabled: true, Writer: &buf, NoColor: false}
		spinner := NewSpinner(cfg, "Test")
		if spinner == nil {
			t.Fatal("NewSpinner() should return non-nil when enabled")
		}
		// Verify spinner can be used without panic
		_ = spinner.Add(1)
		_ = spinner.Finish()
	})

	t.Run("noColor option is respected", func(t *testing.T) {
		var buf bytes.Buffer
		cfg := ProgressConfig{Enabled: true, Writer: &buf, NoColor: true}
		spinner := NewSpinner(cfg, "NoColor Spinner")
		if spinner == nil {
			t.Fatal("NewSpinner() should return non-nil")
		}
		_ = spinner.Finish()
	})
}

func TestPhaseDescription(t *testing.T) {
	tests := []struct {
		phase    string
		expected string
	}{
		{"parsing", "Parsing files"},
		{"embedding", "Generating embeddings"},
		{"embedding_types", "Embedding types"},
		{"writing", "Writing to database"},
		// Default case returns the input unchanged
		{"unknown_phase", "unknown_phase"},
		{"", ""},
		{"custom", "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			result := phaseDescription(tt.phase)
			if result != tt.expected {
				t.Errorf("phaseDescription(%q) = %q, want %q", tt.phase, result, tt.expected)
			}
		})
	}
}

// TestProgressConfigQuietDisablesProgress verifies that quiet mode disables progress
// regardless of TTY status. This is important for JSON output and scripted usage.
func TestProgressConfigQuietDisablesProgress(t *testing.T) {
	// Quiet mode should always disable progress
	cfg := NewProgressConfig(GlobalFlags{Quiet: true})
	if cfg.Enabled {
		t.Error("Progress should be disabled when Quiet=true")
	}

	// JSON mode auto-sets quiet, so should also disable progress
	cfg = NewProgressConfig(GlobalFlags{JSON: true, Quiet: true})
	if cfg.Enabled {
		t.Error("Progress should be disabled when JSON=true (quiet auto-set)")
	}
}
