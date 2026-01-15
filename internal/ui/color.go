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

// Package ui provides user interface utilities for the CIE CLI.
//
// This package offers color output helpers that respect the --no-color flag
// and NO_COLOR environment variable. Colors are automatically disabled when
// the output is not a TTY (e.g., when piped).
//
// Color usage guidelines:
//   - Red: Errors, failures
//   - Yellow: Warnings, cautions
//   - Green: Success, completions
//   - Cyan: Info, neutral messages
//   - Bold: Headers, important labels
//   - Dim: Less important details, paths
package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Pre-configured color instances for consistent CLI output.
//
// These are initialized at package load time and respect the global
// color.NoColor setting when called.
var (
	// Red is used for error messages and failures.
	Red = color.New(color.FgRed)

	// Yellow is used for warnings and cautions.
	Yellow = color.New(color.FgYellow)

	// Green is used for success messages and completions.
	Green = color.New(color.FgGreen)

	// Cyan is used for informational messages.
	Cyan = color.New(color.FgCyan)

	// Bold is used for headers and important labels.
	Bold = color.New(color.Bold)

	// Dim is used for less important details like paths.
	Dim = color.New(color.Faint)
)

// InitColors configures global color output based on the noColor flag.
//
// This should be called early in main() after parsing flags to ensure
// all color output respects the --no-color flag and NO_COLOR environment variable.
//
// The fatih/color library already respects NO_COLOR automatically, but this
// function provides explicit control via the CLI flag.
func InitColors(noColor bool) {
	color.NoColor = noColor
}

// Success prints a green success message with a checkmark prefix.
//
// Example output: "✓ Successfully indexed 42 files"
func Success(msg string) {
	_, _ = Green.Println("✓ " + msg)
}

// Successf prints a formatted green success message with a checkmark prefix.
func Successf(format string, args ...any) {
	_, _ = Green.Printf("✓ "+format+"\n", args...)
}

// Warning prints a yellow warning message with a warning symbol prefix.
//
// Example output: "⚠ Skipped 3 files with errors"
func Warning(msg string) {
	_, _ = Yellow.Println("⚠ " + msg)
}

// Warningf prints a formatted yellow warning message with a warning symbol prefix.
func Warningf(format string, args ...any) {
	_, _ = Yellow.Printf("⚠ "+format+"\n", args...)
}

// Error prints a red error message with an X prefix.
//
// Example output: "✗ Failed to connect to database"
func Error(msg string) {
	_, _ = Red.Println("✗ " + msg)
}

// Errorf prints a formatted red error message with an X prefix.
func Errorf(format string, args ...any) {
	_, _ = Red.Printf("✗ "+format+"\n", args...)
}

// Info prints a cyan informational message with an info symbol prefix.
//
// Example output: "ℹ Processing embeddings..."
func Info(msg string) {
	_, _ = Cyan.Println("ℹ " + msg)
}

// Infof prints a formatted cyan informational message with an info symbol prefix.
func Infof(format string, args ...any) {
	_, _ = Cyan.Printf("ℹ "+format+"\n", args...)
}

// Header prints a bold header with an underline separator.
//
// Example output:
//
//	CIE Project Status
//	==================
func Header(text string) {
	_, _ = Bold.Println(text)
	fmt.Println(strings.Repeat("=", len(text)))
}

// SubHeader prints a bold sub-header without an underline.
//
// Example output: "Entities:"
func SubHeader(text string) {
	_, _ = Bold.Println(text)
}

// Label returns a bold-formatted label string for inline use.
//
// Example: fmt.Printf("%s %s\n", ui.Label("Project ID:"), projectID)
func Label(text string) string {
	return Bold.Sprint(text)
}

// DimText returns a dim-formatted string for less important text.
//
// Example: fmt.Printf("Data stored in: %s\n", ui.DimText(dataDir))
func DimText(text string) string {
	return Dim.Sprint(text)
}

// CountText returns a cyan-formatted count value for statistics display.
//
// Example: fmt.Printf("  Files: %s\n", ui.CountText(42))
func CountText(count int) string {
	return Cyan.Sprint(count)
}
