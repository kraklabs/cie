// Copyright 2026 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package output provides utilities for consistent CLI output formatting.
//
// This package handles JSON encoding for machine-readable output, ensuring
// consistent formatting across all CIE CLI commands. It complements the
// ui package (for human-readable output) and errors package (for error handling).
//
// # Usage
//
// For JSON output in CLI commands:
//
//	type Result struct {
//	    ProjectID string `json:"project_id"`
//	    Count     int    `json:"count"`
//	}
//	result := &Result{ProjectID: "my-project", Count: 42}
//	if err := output.JSON(result); err != nil {
//	    errors.FatalError(err, true)
//	}
//
// For compact JSON (e.g., streaming):
//
//	if err := output.JSONCompact(result); err != nil {
//	    errors.FatalError(err, true)
//	}
//
// For error output (always goes to stderr):
//
//	if err := doSomething(); err != nil {
//	    output.JSONError(err)
//	}
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSON writes data as pretty-printed JSON to stdout.
//
// The output is formatted with 2-space indentation for readability.
// This is the standard format for --json output in CIE CLI commands.
//
// Returns an error if JSON encoding fails (e.g., for unencodable types
// like channels or functions).
func JSON(data any) error {
	return JSONTo(os.Stdout, data)
}

// JSONTo writes data as pretty-printed JSON to the specified writer.
//
// This is useful for testing or when output needs to go somewhere
// other than stdout.
func JSONTo(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}
	return nil
}

// JSONCompact writes data as compact JSON to stdout.
//
// The output contains no extra whitespace, making it suitable for
// streaming output or when size matters.
//
// Returns an error if JSON encoding fails.
func JSONCompact(data any) error {
	return JSONCompactTo(os.Stdout, data)
}

// JSONCompactTo writes data as compact JSON to the specified writer.
//
// This is useful for testing or when output needs to go somewhere
// other than stdout.
func JSONCompactTo(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("JSON encoding failed: %w", err)
	}
	return nil
}

// ErrorJSON represents an error in JSON format for machine consumption.
type ErrorJSON struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// JSONError writes an error as JSON to stderr.
//
// The error is wrapped in a JSON object with an "error" field.
// This ensures consistent error output format when --json mode is active.
//
// Returns an error only if JSON encoding itself fails (rare).
func JSONError(err error) error {
	return JSONErrorTo(os.Stderr, err)
}

// JSONErrorTo writes an error as JSON to the specified writer.
//
// This is useful for testing.
func JSONErrorTo(w io.Writer, err error) error {
	errObj := ErrorJSON{Error: err.Error()}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(errObj); encErr != nil {
		return fmt.Errorf("JSON error encoding failed: %w", encErr)
	}
	return nil
}
