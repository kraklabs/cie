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

package ui

import (
	"testing"

	"github.com/fatih/color"
)

func TestInitColors(t *testing.T) {
	// Save original state
	original := color.NoColor
	defer func() { color.NoColor = original }()

	tests := []struct {
		name     string
		noColor  bool
		expected bool
	}{
		{
			name:     "colors enabled when noColor is false",
			noColor:  false,
			expected: false,
		},
		{
			name:     "colors disabled when noColor is true",
			noColor:  true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitColors(tt.noColor)
			if color.NoColor != tt.expected {
				t.Errorf("InitColors(%v): color.NoColor = %v, expected %v",
					tt.noColor, color.NoColor, tt.expected)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	// Disable colors for predictable output
	original := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = original }()

	result := Label("Project ID:")
	expected := "Project ID:"
	if result != expected {
		t.Errorf("Label() = %q, expected %q", result, expected)
	}
}

func TestDimText(t *testing.T) {
	// Disable colors for predictable output
	original := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = original }()

	result := DimText("/path/to/data")
	expected := "/path/to/data"
	if result != expected {
		t.Errorf("DimText() = %q, expected %q", result, expected)
	}
}

func TestCountText(t *testing.T) {
	// Disable colors for predictable output
	original := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = original }()

	result := CountText(42)
	expected := "42"
	if result != expected {
		t.Errorf("CountText() = %q, expected %q", result, expected)
	}
}

func TestColorVariablesInitialized(t *testing.T) {
	// Verify all color variables are properly initialized
	if Red == nil {
		t.Error("Red color not initialized")
	}
	if Yellow == nil {
		t.Error("Yellow color not initialized")
	}
	if Green == nil {
		t.Error("Green color not initialized")
	}
	if Cyan == nil {
		t.Error("Cyan color not initialized")
	}
	if Bold == nil {
		t.Error("Bold color not initialized")
	}
	if Dim == nil {
		t.Error("Dim color not initialized")
	}
}

func TestMessageFunctions(t *testing.T) {
	// Save original state and disable colors for predictable output
	original := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = original }()

	// Test that message functions don't panic and can be called
	// (they write to stdout so we can't easily capture their output without
	// more complex test setup, but we can verify they execute without error)
	t.Run("Success", func(t *testing.T) {
		Success("test success")
		// If we reach here, no panic occurred
	})

	t.Run("Successf", func(t *testing.T) {
		Successf("test %s with %d items", "success", 42)
	})

	t.Run("Warning", func(t *testing.T) {
		Warning("test warning")
	})

	t.Run("Warningf", func(t *testing.T) {
		Warningf("test %s with %d items", "warning", 42)
	})

	t.Run("Error", func(t *testing.T) {
		Error("test error")
	})

	t.Run("Errorf", func(t *testing.T) {
		Errorf("test %s with %d items", "error", 42)
	})

	t.Run("Info", func(t *testing.T) {
		Info("test info")
	})

	t.Run("Infof", func(t *testing.T) {
		Infof("test %s with %d items", "info", 42)
	})

	t.Run("Header", func(t *testing.T) {
		Header("Test Header")
	})

	t.Run("SubHeader", func(t *testing.T) {
		SubHeader("Test SubHeader")
	})
}

func TestEdgeCases(t *testing.T) {
	// Save original state and disable colors for predictable output
	original := color.NoColor
	color.NoColor = true
	defer func() { color.NoColor = original }()

	t.Run("empty string label", func(t *testing.T) {
		result := Label("")
		if result != "" {
			t.Errorf("Label(\"\") = %q, expected empty string", result)
		}
	})

	t.Run("empty string dimText", func(t *testing.T) {
		result := DimText("")
		if result != "" {
			t.Errorf("DimText(\"\") = %q, expected empty string", result)
		}
	})

	t.Run("zero countText", func(t *testing.T) {
		result := CountText(0)
		if result != "0" {
			t.Errorf("CountText(0) = %q, expected \"0\"", result)
		}
	})

	t.Run("negative countText", func(t *testing.T) {
		result := CountText(-1)
		if result != "-1" {
			t.Errorf("CountText(-1) = %q, expected \"-1\"", result)
		}
	})

	t.Run("special characters in label", func(t *testing.T) {
		result := Label("Test: <>\"'&")
		expected := "Test: <>\"'&"
		if result != expected {
			t.Errorf("Label() with special chars = %q, expected %q", result, expected)
		}
	})
}
