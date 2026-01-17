// Copyright 2026 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-only

package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestJSON verifies that JSON produces pretty-printed output with 2-space indentation.
func TestJSON(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]any{
		"project_id": "test-project",
		"count":      42,
	}

	if err := JSONTo(&buf, data); err != nil {
		t.Fatalf("JSONTo failed: %v", err)
	}

	output := buf.String()

	// Check for pretty-printing (2-space indentation)
	if !strings.Contains(output, "  \"project_id\"") {
		t.Errorf("Expected 2-space indentation, got: %s", output)
	}

	// Check for expected content
	if !strings.Contains(output, `"project_id": "test-project"`) {
		t.Errorf("Missing project_id field, got: %s", output)
	}
	if !strings.Contains(output, `"count": 42`) {
		t.Errorf("Missing count field, got: %s", output)
	}

	// Check for trailing newline (json.Encoder adds it)
	if !strings.HasSuffix(output, "}\n") {
		t.Errorf("Expected trailing newline, got: %q", output)
	}
}

// TestJSONCompact verifies that JSONCompact produces single-line output.
func TestJSONCompact(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]any{
		"project_id": "test-project",
		"count":      42,
	}

	if err := JSONCompactTo(&buf, data); err != nil {
		t.Fatalf("JSONCompactTo failed: %v", err)
	}

	output := buf.String()

	// Compact output should not have indentation
	if strings.Contains(output, "  ") {
		t.Errorf("Compact JSON should not have indentation, got: %s", output)
	}

	// Check for expected content (on single line)
	if !strings.Contains(output, `"project_id":"test-project"`) {
		t.Errorf("Missing project_id field in compact output, got: %s", output)
	}
}

// TestJSONError verifies that JSONError produces properly formatted error JSON.
func TestJSONError(t *testing.T) {
	var buf bytes.Buffer

	err := errors.New("something went wrong")

	if encErr := JSONErrorTo(&buf, err); encErr != nil {
		t.Fatalf("JSONErrorTo failed: %v", encErr)
	}

	output := buf.String()

	// Check for error field
	if !strings.Contains(output, `"error": "something went wrong"`) {
		t.Errorf("Missing error field, got: %s", output)
	}

	// Check for pretty-printing
	if !strings.Contains(output, "  \"error\"") {
		t.Errorf("Expected 2-space indentation in error output, got: %s", output)
	}
}

// TestJSONSpecialCharacters verifies proper handling of special characters.
func TestJSONSpecialCharacters(t *testing.T) {
	var buf bytes.Buffer

	data := map[string]string{
		"message": "Hello \"world\" with <html> & special chars",
		"path":    "/usr/local/bin\ttest",
	}

	if err := JSONTo(&buf, data); err != nil {
		t.Fatalf("JSONTo failed: %v", err)
	}

	output := buf.String()

	// JSON should properly escape quotes
	if !strings.Contains(output, `\"world\"`) {
		t.Errorf("Expected escaped quotes, got: %s", output)
	}

	// JSON should properly escape tabs
	if !strings.Contains(output, `\t`) {
		t.Errorf("Expected escaped tab, got: %s", output)
	}
}

// TestJSONStructWithTags verifies that struct JSON tags are respected.
func TestJSONStructWithTags(t *testing.T) {
	type TestStruct struct {
		ProjectID   string `json:"project_id"`
		Count       int    `json:"count"`
		OmitEmpty   string `json:"omit_empty,omitempty"`
		IgnoreField string `json:"-"`
	}

	var buf bytes.Buffer

	data := TestStruct{
		ProjectID:   "my-project",
		Count:       100,
		OmitEmpty:   "", // Should be omitted
		IgnoreField: "should-not-appear",
	}

	if err := JSONTo(&buf, data); err != nil {
		t.Fatalf("JSONTo failed: %v", err)
	}

	output := buf.String()

	// Check that tags are respected
	if !strings.Contains(output, `"project_id"`) {
		t.Errorf("Expected project_id (not ProjectID), got: %s", output)
	}

	// Check omitempty
	if strings.Contains(output, `"omit_empty"`) {
		t.Errorf("Expected omit_empty to be omitted, got: %s", output)
	}

	// Check ignored field
	if strings.Contains(output, "should-not-appear") {
		t.Errorf("Expected IgnoreField to be excluded, got: %s", output)
	}
}

// TestJSONNestedStructure verifies proper handling of nested structures.
func TestJSONNestedStructure(t *testing.T) {
	type Inner struct {
		Value string `json:"value"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	var buf bytes.Buffer

	data := Outer{
		Name:  "outer",
		Inner: Inner{Value: "inner-value"},
	}

	if err := JSONTo(&buf, data); err != nil {
		t.Fatalf("JSONTo failed: %v", err)
	}

	output := buf.String()

	// Check nested structure is properly indented
	if !strings.Contains(output, `"inner": {`) {
		t.Errorf("Expected nested object, got: %s", output)
	}
	if !strings.Contains(output, `"value": "inner-value"`) {
		t.Errorf("Expected nested value, got: %s", output)
	}
}

// TestJSONNilValue verifies proper handling of nil values.
func TestJSONNilValue(t *testing.T) {
	var buf bytes.Buffer

	type MaybeNil struct {
		Ptr *string `json:"ptr"`
	}

	data := MaybeNil{Ptr: nil}

	if err := JSONTo(&buf, data); err != nil {
		t.Fatalf("JSONTo failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, `"ptr": null`) {
		t.Errorf("Expected null for nil pointer, got: %s", output)
	}
}
