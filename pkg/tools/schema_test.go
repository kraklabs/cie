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
	"context"
	"testing"
)

func TestGetSchema(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"returns schema documentation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := GetSchema(ctx)

			assertNoError(t, err)
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Text == "" {
				t.Error("expected non-empty output")
			}
			assertContains(t, result.Text, "CIE Database Schema")
			assertContains(t, result.Text, "cie_function")
			assertContains(t, result.Text, "cie_file")
		})
	}
}

func TestGetSchema_WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Schema should still work since it returns a constant
	result, err := GetSchema(ctx)
	assertNoError(t, err)
	if result == nil {
		t.Fatal("expected result even with canceled context")
	}
}

func TestGetSchema_OutputContainsExpectedSections(t *testing.T) {
	ctx := context.Background()
	result, err := GetSchema(ctx)
	assertNoError(t, err)

	// Verify key sections are present
	expectedSections := []string{
		"Core Tables",
		"cie_function",
		"cie_file",
		"cie_type",
		"cie_function_code",
		"cie_function_embedding",
		"Edge Tables",
		"cie_defines",
		"cie_calls",
		"CozoScript Operators",
		"Example Queries",
		"CIE Tools Quick Reference",
	}

	for _, section := range expectedSections {
		assertContains(t, result.Text, section)
	}
}

func TestGetSchema_ContainsSchemav3Description(t *testing.T) {
	ctx := context.Background()
	result, err := GetSchema(ctx)
	assertNoError(t, err)

	// Verify schema version is documented
	assertContains(t, result.Text, "Schema v3")
	assertContains(t, result.Text, "vertical partitioning")
}
