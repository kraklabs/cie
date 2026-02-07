// Copyright 2026 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package errors provides structured error handling for the CIE CLI.
//
// This package defines UserError, a type that carries structured error information
// including what went wrong, why it happened, and how to fix it. It also defines
// consistent exit codes for different error categories.
//
// # Usage Example
//
// Creating and displaying errors:
//
//	err := errors.NewConfigError(
//	    "Cannot open CIE database",
//	    "The database file is locked by another process",
//	    "Close other CIE instances or run: cie reset --yes",
//	    underlyingErr,
//	)
//	if err != nil {
//	    // Simple approach: print and exit with colored output
//	    errors.FatalError(err, false)
//	}
//
// # Formatted Output
//
// The Format() method provides colored terminal output:
//
//	err := errors.NewDatabaseError(
//	    "Cannot open CIE database",
//	    "The database file is locked by another process",
//	    "Close other CIE instances or run: cie reset --yes",
//	    underlyingErr,
//	)
//	fmt.Fprint(os.Stderr, err.Format(false))
//	// Output (with colors):
//	// Error: Cannot open CIE database
//	// Cause: The database file is locked by another process
//	// Fix:   Close other CIE instances or run: cie reset --yes
//
// For JSON output:
//
//	jsonData := err.ToJSON()
//	json.NewEncoder(os.Stderr).Encode(jsonData)
//	// Output:
//	// {
//	//   "error": "Cannot open CIE database",
//	//   "cause": "The database file is locked by another process",
//	//   "fix": "Close other CIE instances or run: cie reset --yes",
//	//   "exit_code": 2
//	// }
//
// # Exit Codes
//
// The package defines semantic exit codes following Unix conventions:
//   - ExitSuccess (0): Successful execution
//   - ExitConfig (1): Configuration errors (missing/invalid config)
//   - ExitDatabase (2): Database errors (locked, corrupted, etc.)
//   - ExitNetwork (3): Network/API errors (connection failed, timeout)
//   - ExitInput (4): Invalid user input (bad arguments, validation errors)
//   - ExitPermission (5): Permission denied (file access, etc.)
//   - ExitNotFound (6): Resource not found (project, file, etc.)
//   - ExitInternal (10): Internal errors (bugs, panics)
package errors

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

// Exit codes for different error categories.
const (
	// ExitSuccess indicates successful execution.
	ExitSuccess = 0

	// ExitConfig indicates configuration errors (missing/invalid config files).
	ExitConfig = 1

	// ExitDatabase indicates database errors (file locked, corrupted, etc.).
	ExitDatabase = 2

	// ExitNetwork indicates network or API errors (connection failed, timeout).
	ExitNetwork = 3

	// ExitInput indicates invalid user input (bad arguments, validation errors).
	ExitInput = 4

	// ExitPermission indicates permission denied errors (file access, etc.).
	ExitPermission = 5

	// ExitNotFound indicates resource not found errors (project, file, etc.).
	ExitNotFound = 6

	// ExitInternal indicates internal errors (bugs, unexpected panics).
	// Exit code 10 signals "this is a bug that should be reported".
	ExitInternal = 10
)

// UserError represents an error with structured context for end users.
//
// It provides three levels of information:
//   - Message: What went wrong (user-facing error description)
//   - Cause: Why it happened (diagnostic information)
//   - Fix: How to fix it (actionable suggestion)
//
// UserError also carries an exit code for consistent CLI exit behavior
// and optionally wraps an underlying error for error chain compatibility.
type UserError struct {
	// Message describes what went wrong in user-friendly language.
	Message string

	// Cause explains why the error occurred (diagnostic information).
	Cause string

	// Fix provides an actionable suggestion on how to resolve the error.
	Fix string

	// ExitCode is the exit code that should be used when exiting due to this error.
	ExitCode int

	// Err is the underlying error that caused this error (optional).
	// This enables error wrapping and compatibility with errors.Is/As.
	Err error
}

// Error implements the error interface.
//
// It returns a simple error message string. If an underlying error is present,
// it appends that error's message for context.
func (e *UserError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap implements error unwrapping for compatibility with errors.Is and errors.As.
//
// It returns the underlying error, allowing standard library error inspection
// functions to work with error chains.
func (e *UserError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a configuration error with exit code ExitConfig.
//
// Use this for errors related to missing, invalid, or malformed configuration files.
//
// Example:
//
//	return NewConfigError(
//	    "Cannot load CIE configuration",
//	    "The config file ~/.cie/config.yaml is missing",
//	    "Run 'cie init' to create a new configuration",
//	    nil,
//	)
func NewConfigError(msg, cause, fix string, err error) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitConfig,
		Err:      err,
	}
}

// NewDatabaseError creates a database error with exit code ExitDatabase.
//
// Use this for errors related to database operations, such as locked files,
// corruption, or failed transactions.
//
// Example:
//
//	return NewDatabaseError(
//	    "Cannot open CIE database",
//	    "The database file is locked by another process",
//	    "Close other CIE instances or run: cie reset --yes",
//	    err,
//	)
func NewDatabaseError(msg, cause, fix string, err error) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitDatabase,
		Err:      err,
	}
}

// NewNetworkError creates a network error with exit code ExitNetwork.
//
// Use this for errors related to network connectivity, API calls, or remote operations.
//
// Example:
//
//	return NewNetworkError(
//	    "Cannot connect to embedding API",
//	    "Connection timed out after 30 seconds",
//	    "Check your network connection and try again",
//	    err,
//	)
func NewNetworkError(msg, cause, fix string, err error) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitNetwork,
		Err:      err,
	}
}

// NewInputError creates an input validation error with exit code ExitInput.
//
// Use this for errors related to invalid user input, such as bad command-line
// arguments or failed validation checks. Input errors typically do not wrap
// an underlying error.
//
// Example:
//
//	return NewInputError(
//	    "Invalid project name",
//	    "Project name must contain only alphanumeric characters",
//	    "Use a name like 'my-project' or 'myproject123'",
//	)
func NewInputError(msg, cause, fix string) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitInput,
		Err:      nil, // Input errors typically don't wrap underlying errors
	}
}

// NewPermissionError creates a permission denied error with exit code ExitPermission.
//
// Use this for errors related to insufficient permissions, such as file access
// or operation authorization failures.
//
// Example:
//
//	return NewPermissionError(
//	    "Cannot write to index directory",
//	    "Permission denied for ~/.cie/indexes/",
//	    "Run with appropriate permissions or change the index directory",
//	    err,
//	)
func NewPermissionError(msg, cause, fix string, err error) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitPermission,
		Err:      err,
	}
}

// NewNotFoundError creates a resource not found error with exit code ExitNotFound.
//
// Use this for errors when a requested resource (project, file, etc.) cannot be found.
// Not found errors typically do not wrap an underlying error.
//
// Example:
//
//	return NewNotFoundError(
//	    "Project not found",
//	    "No project named 'myproject' exists in the index",
//	    "Run 'cie status' to list indexed projects",
//	)
func NewNotFoundError(msg, cause, fix string) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitNotFound,
		Err:      nil, // Not found errors typically don't wrap underlying errors
	}
}

// NewInternalError creates an internal error with exit code ExitInternal.
//
// Use this for unexpected errors that indicate bugs in the program, such as
// assertion failures, unexpected nil values, or unhandled error cases.
// Internal errors should be reported to the maintainers.
//
// Example:
//
//	return NewInternalError(
//	    "Unexpected nil pointer",
//	    "The function indexer returned nil unexpectedly",
//	    "This is a bug. Please report it at github.com/kraklabs/kraken/issues",
//	    err,
//	)
func NewInternalError(msg, cause, fix string, err error) *UserError {
	return &UserError{
		Message:  msg,
		Cause:    cause,
		Fix:      fix,
		ExitCode: ExitInternal,
		Err:      err,
	}
}

// Color definitions for error formatting.
var (
	colorError = color.New(color.FgRed, color.Bold)
	colorCause = color.New(color.FgYellow)
	colorFix   = color.New(color.FgGreen)
)

// Format returns a formatted error message for terminal display.
//
// The output includes colored sections for Error (red/bold), Cause (yellow),
// and Fix (green). Color output respects the NO_COLOR environment variable
// and can be explicitly disabled with the noColor parameter.
//
// Example output:
//
//	Error: Cannot open the CIE database
//	Cause: The database file is locked by another process
//	Fix:   Close other CIE instances or run: cie reset --yes
//
// Empty Cause or Fix fields are omitted from the output.
//
// Note: This method temporarily modifies the global color.NoColor state
// and restores it after formatting to ensure thread safety.
func (e *UserError) Format(noColor bool) string {
	// Save and restore global color state to avoid side effects
	originalNoColor := color.NoColor
	defer func() { color.NoColor = originalNoColor }()

	if noColor || os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}

	var out strings.Builder
	out.WriteString(colorError.Sprint("Error: "))
	out.WriteString(e.Message)
	out.WriteString("\n")

	if e.Cause != "" {
		out.WriteString(colorCause.Sprint("Cause: "))
		out.WriteString(e.Cause)
		out.WriteString("\n")
	}

	if e.Fix != "" {
		out.WriteString(colorFix.Sprint("Fix:   "))
		out.WriteString(e.Fix)
		out.WriteString("\n")
	}

	return out.String()
}

// ErrorJSON represents error information in JSON format.
//
// This structure is suitable for machine consumption and integrates with
// CLI commands that support --json output mode.
type ErrorJSON struct {
	Error    string `json:"error"`
	Cause    string `json:"cause,omitempty"`
	Fix      string `json:"fix,omitempty"`
	ExitCode int    `json:"exit_code"`
}

// ToJSON converts the UserError to a JSON-serializable structure.
//
// Fields with empty values (Cause, Fix) are omitted from JSON output
// using the omitempty tag. This keeps JSON output clean when additional
// context is not available.
func (e *UserError) ToJSON() ErrorJSON {
	return ErrorJSON{
		Error:    e.Message,
		Cause:    e.Cause,
		Fix:      e.Fix,
		ExitCode: e.ExitCode,
	}
}

// FatalError prints the error and exits with the appropriate code.
//
// If the error is a UserError, it uses Format() for colored output or
// ToJSON() for JSON mode. For non-UserError types, it prints a simple
// error message and exits with ExitInternal.
//
// This function never returns - it always calls os.Exit().
//
// Usage:
//
//	if err := doSomething(); err != nil {
//	    errors.FatalError(err, jsonMode)
//	}
func FatalError(err error, jsonOutput bool) {
	if err == nil {
		return
	}

	if ue, ok := err.(*UserError); ok {
		if jsonOutput {
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("", "  ")
			// Encode error is intentionally ignored since we're about to exit.
			// If JSON encoding fails, the program will still exit with the correct code.
			_ = enc.Encode(ue.ToJSON())
		} else {
			fmt.Fprint(os.Stderr, ue.Format(false))
		}
		os.Exit(ue.ExitCode)
	}

	// Fallback for non-UserError
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(ExitInternal)
}
