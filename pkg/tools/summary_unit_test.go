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
	"testing"
)

func TestNormalizeDirPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no trailing slash", "internal/cie", "internal/cie"},
		{"trailing slash", "internal/cie/", "internal/cie"},
		{"root with slash", "/", ""},
		{"multiple trailing slashes", "apps/gateway//", "apps/gateway/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeDirPath(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDirPath(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDirFuncResults(t *testing.T) {
	tests := []struct {
		name string
		rows [][]any
		want int // number of funcs
	}{
		{"empty rows", [][]any{}, 0},
		{"single function", [][]any{{"HandleRequest", "func HandleRequest()", "10"}}, 1},
		{"multiple functions", [][]any{
			{"HandleRequest", "func HandleRequest()", "10"},
			{"processData", "func processData()", "20"},
		}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := parseDirFuncResults(tt.rows)
			if len(funcs) != tt.want {
				t.Errorf("len(parseDirFuncResults()) = %d; want %d", len(funcs), tt.want)
			}
		})
	}
}

func TestParseDirFuncResults_ExportedDetection(t *testing.T) {
	rows := [][]any{
		{"HandleRequest", "sig1", "10"}, // Exported (capital H)
		{"processData", "sig2", "20"},   // Not exported (lowercase p)
	}

	funcs := parseDirFuncResults(rows)

	if !funcs[0].exported {
		t.Error("HandleRequest should be detected as exported")
	}
	if funcs[1].exported {
		t.Error("processData should not be detected as exported")
	}
}

func TestFormatExportedFunc(t *testing.T) {
	f := dirFuncInfo{
		name:      "NewClient",
		signature: "func NewClient(url string) *Client",
		line:      "42",
		exported:  true,
	}

	output := formatExportedFunc(f)

	assertContains(t, output, "NewClient")
	assertContains(t, output, "line 42")
	assertContains(t, output, "func NewClient")
}

func TestFormatExportedFunc_LongSignature(t *testing.T) {
	longSig := "func VeryLongFunctionNameWithManyParameters(param1 string, param2 int, param3 bool, param4 interface{}, param5 error) (result string, err error)"
	f := dirFuncInfo{
		name:      "VeryLongFunctionNameWithManyParameters",
		signature: longSig,
		line:      "100",
		exported:  true,
	}

	output := formatExportedFunc(f)

	// Should be truncated
	if len(output) > 200 {
		// Check that signature was truncated with "..."
		assertContains(t, output, "...")
	}
}

func TestFormatDirFuncs(t *testing.T) {
	funcs := []dirFuncInfo{
		{name: "NewClient", signature: "func NewClient()", line: "10", exported: true},
		{name: "ProcessData", signature: "func ProcessData()", line: "20", exported: true},
		{name: "helper", signature: "func helper()", line: "30", exported: false},
	}

	// Test with maxFuncs = 2
	output := formatDirFuncs(funcs, 2)

	// Should show first 2 exported functions
	assertContains(t, output, "NewClient")
	assertContains(t, output, "ProcessData")
	// Should not show unexported function (maxFuncs reached)
	assertNotContains(t, output, "helper")
}

func TestFormatDirFuncs_ExportedFirst(t *testing.T) {
	funcs := []dirFuncInfo{
		{name: "helper", signature: "func helper()", line: "10", exported: false},
		{name: "NewClient", signature: "func NewClient()", line: "20", exported: true},
	}

	output := formatDirFuncs(funcs, 2)

	// Exported should come first in output
	exportedPos := -1
	unexportedPos := -1
	for i := range output {
		if i < len(output)-len("NewClient") && output[i:i+len("NewClient")] == "NewClient" {
			exportedPos = i
		}
		if i < len(output)-len("helper") && output[i:i+len("helper")] == "helper" {
			unexportedPos = i
		}
	}

	if exportedPos > unexportedPos && unexportedPos > 0 {
		t.Error("exported function should appear before unexported")
	}
}
