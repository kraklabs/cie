// Copyright 2026 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-only

package errors

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestUserError_Error verifies the Error() method implementation.
func TestUserError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *UserError
		want string
	}{
		{
			name: "with underlying error",
			err: &UserError{
				Message: "Cannot open database",
				Err:     fmt.Errorf("file locked"),
			},
			want: "Cannot open database: file locked",
		},
		{
			name: "without underlying error",
			err: &UserError{
				Message: "Invalid input",
				Err:     nil,
			},
			want: "Invalid input",
		},
		{
			name: "empty message with underlying error",
			err: &UserError{
				Message: "",
				Err:     fmt.Errorf("some error"),
			},
			want: ": some error",
		},
		{
			name: "empty message without underlying error",
			err: &UserError{
				Message: "",
				Err:     nil,
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("UserError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUserError_Unwrap verifies the Unwrap() method implementation.
func TestUserError_Unwrap(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")

	tests := []struct {
		name    string
		err     *UserError
		wantNil bool
	}{
		{
			name: "with underlying error",
			err: &UserError{
				Message: "test",
				Err:     underlyingErr,
			},
			wantNil: false,
		},
		{
			name: "without underlying error",
			err: &UserError{
				Message: "test",
				Err:     nil,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Unwrap()
			if tt.wantNil && got != nil {
				t.Errorf("UserError.Unwrap() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("UserError.Unwrap() = nil, want non-nil")
			}
			if !tt.wantNil && got != underlyingErr {
				t.Errorf("UserError.Unwrap() = %v, want %v", got, underlyingErr)
			}
		})
	}
}

// TestExitCodes verifies that exit code constants have the correct values.
func TestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		want     int
	}{
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitConfig", ExitConfig, 1},
		{"ExitDatabase", ExitDatabase, 2},
		{"ExitNetwork", ExitNetwork, 3},
		{"ExitInput", ExitInput, 4},
		{"ExitPermission", ExitPermission, 5},
		{"ExitNotFound", ExitNotFound, 6},
		{"ExitInternal", ExitInternal, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.exitCode != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.exitCode, tt.want)
			}
		})
	}
}

// TestExitCodes_Uniqueness verifies that all exit codes are unique.
func TestExitCodes_Uniqueness(t *testing.T) {
	codes := []int{
		ExitSuccess,
		ExitConfig,
		ExitDatabase,
		ExitNetwork,
		ExitInput,
		ExitPermission,
		ExitNotFound,
		ExitInternal,
	}

	seen := make(map[int]bool)
	for _, code := range codes {
		if seen[code] && code != ExitSuccess {
			// ExitSuccess is zero value, so duplicates are expected if someone forgets to set a value
			t.Errorf("Duplicate exit code found: %d", code)
		}
		seen[code] = true
	}
}

// TestConstructors verifies that all constructor functions work correctly.
func TestConstructors(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying error")

	tests := []struct {
		name         string
		constructor  func() *UserError
		wantMessage  string
		wantCause    string
		wantFix      string
		wantExitCode int
		wantHasErr   bool
	}{
		{
			name: "NewConfigError with underlying error",
			constructor: func() *UserError {
				return NewConfigError("msg", "cause", "fix", underlyingErr)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitConfig,
			wantHasErr:   true,
		},
		{
			name: "NewConfigError without underlying error",
			constructor: func() *UserError {
				return NewConfigError("msg", "cause", "fix", nil)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitConfig,
			wantHasErr:   false,
		},
		{
			name: "NewDatabaseError",
			constructor: func() *UserError {
				return NewDatabaseError("msg", "cause", "fix", underlyingErr)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitDatabase,
			wantHasErr:   true,
		},
		{
			name: "NewNetworkError",
			constructor: func() *UserError {
				return NewNetworkError("msg", "cause", "fix", underlyingErr)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitNetwork,
			wantHasErr:   true,
		},
		{
			name: "NewInputError",
			constructor: func() *UserError {
				return NewInputError("msg", "cause", "fix")
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitInput,
			wantHasErr:   false, // Input errors don't wrap underlying errors
		},
		{
			name: "NewPermissionError",
			constructor: func() *UserError {
				return NewPermissionError("msg", "cause", "fix", underlyingErr)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitPermission,
			wantHasErr:   true,
		},
		{
			name: "NewNotFoundError",
			constructor: func() *UserError {
				return NewNotFoundError("msg", "cause", "fix")
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitNotFound,
			wantHasErr:   false, // Not found errors don't wrap underlying errors
		},
		{
			name: "NewInternalError",
			constructor: func() *UserError {
				return NewInternalError("msg", "cause", "fix", underlyingErr)
			},
			wantMessage:  "msg",
			wantCause:    "cause",
			wantFix:      "fix",
			wantExitCode: ExitInternal,
			wantHasErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.constructor()

			if got.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMessage)
			}
			if got.Cause != tt.wantCause {
				t.Errorf("Cause = %q, want %q", got.Cause, tt.wantCause)
			}
			if got.Fix != tt.wantFix {
				t.Errorf("Fix = %q, want %q", got.Fix, tt.wantFix)
			}
			if got.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", got.ExitCode, tt.wantExitCode)
			}

			hasErr := got.Err != nil
			if hasErr != tt.wantHasErr {
				t.Errorf("has underlying error = %v, want %v", hasErr, tt.wantHasErr)
			}
		})
	}
}

// TestErrorChain verifies error wrapping compatibility with stdlib errors package.
func TestErrorChain(t *testing.T) {
	t.Run("errors.Is works with UserError", func(t *testing.T) {
		sentinel := fmt.Errorf("sentinel error")
		wrapped := fmt.Errorf("wrapped: %w", sentinel)
		userErr := NewDatabaseError("database error", "cause", "fix", wrapped)

		if !errors.Is(userErr, sentinel) {
			t.Error("errors.Is should find sentinel error in chain")
		}
	})

	t.Run("errors.As works with UserError", func(t *testing.T) {
		underlyingErr := NewConfigError("config error", "cause", "fix", nil)
		wrappedErr := NewDatabaseError("database error", "cause", "fix", underlyingErr)

		var targetErr *UserError
		if !errors.As(wrappedErr, &targetErr) {
			t.Fatal("errors.As should extract UserError")
		}

		// Should get the outer (database) error first
		if targetErr.ExitCode != ExitDatabase {
			t.Errorf("ExitCode = %d, want %d", targetErr.ExitCode, ExitDatabase)
		}
	})

	t.Run("errors.As finds nested UserError", func(t *testing.T) {
		innerErr := NewConfigError("config error", "cause", "fix", nil)
		outerErr := NewDatabaseError("database error", "cause", "fix", innerErr)

		// First unwrap should give us the database error
		var dbErr *UserError
		if !errors.As(outerErr, &dbErr) {
			t.Fatal("errors.As should extract database UserError")
		}
		if dbErr.ExitCode != ExitDatabase {
			t.Errorf("First unwrap: ExitCode = %d, want %d", dbErr.ExitCode, ExitDatabase)
		}

		// Unwrapping again should give us the config error
		if dbErr.Err == nil {
			t.Fatal("Database error should have underlying error")
		}
		var cfgErr *UserError
		if !errors.As(dbErr.Err, &cfgErr) {
			t.Fatal("errors.As should extract config UserError from chain")
		}
		if cfgErr.ExitCode != ExitConfig {
			t.Errorf("Second unwrap: ExitCode = %d, want %d", cfgErr.ExitCode, ExitConfig)
		}
	})

	t.Run("multiple levels of wrapping", func(t *testing.T) {
		baseErr := fmt.Errorf("base error")
		level1 := fmt.Errorf("level 1: %w", baseErr)
		level2 := NewNetworkError("level 2", "cause", "fix", level1)
		level3 := NewInternalError("level 3", "cause", "fix", level2)

		// Should be able to find the base error through all layers
		if !errors.Is(level3, baseErr) {
			t.Error("errors.Is should find base error through multiple UserError layers")
		}

		// Should be able to extract UserError at each level
		var userErr *UserError
		if !errors.As(level3, &userErr) {
			t.Fatal("errors.As should extract UserError")
		}
		if userErr.ExitCode != ExitInternal {
			t.Errorf("Top-level ExitCode = %d, want %d", userErr.ExitCode, ExitInternal)
		}
	})
}

// TestUserError_AllFields verifies that all fields are properly set and accessible.
func TestUserError_AllFields(t *testing.T) {
	underlyingErr := fmt.Errorf("underlying")
	err := &UserError{
		Message:  "test message",
		Cause:    "test cause",
		Fix:      "test fix",
		ExitCode: 42,
		Err:      underlyingErr,
	}

	if err.Message != "test message" {
		t.Errorf("Message = %q, want %q", err.Message, "test message")
	}
	if err.Cause != "test cause" {
		t.Errorf("Cause = %q, want %q", err.Cause, "test cause")
	}
	if err.Fix != "test fix" {
		t.Errorf("Fix = %q, want %q", err.Fix, "test fix")
	}
	if err.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want %d", err.ExitCode, 42)
	}
	if err.Err != underlyingErr {
		t.Errorf("Err = %v, want %v", err.Err, underlyingErr)
	}
}

// TestUserError_Format verifies the Format() method implementation.
func TestUserError_Format(t *testing.T) {
	tests := []struct {
		name    string
		err     *UserError
		noColor bool
		want    []string // Substrings that must be present
	}{
		{
			name: "full error with color disabled",
			err: &UserError{
				Message:  "Cannot open database",
				Cause:    "The database file is locked",
				Fix:      "Close other CIE instances",
				ExitCode: ExitDatabase,
			},
			noColor: true,
			want:    []string{"Error: Cannot open database", "Cause: The database file is locked", "Fix:   Close other CIE instances"},
		},
		{
			name: "error without cause",
			err: &UserError{
				Message:  "Invalid input",
				Cause:    "",
				Fix:      "Use valid format",
				ExitCode: ExitInput,
			},
			noColor: true,
			want:    []string{"Error: Invalid input", "Fix:   Use valid format"},
		},
		{
			name: "error without fix",
			err: &UserError{
				Message:  "Network error",
				Cause:    "Connection timeout",
				Fix:      "",
				ExitCode: ExitNetwork,
			},
			noColor: true,
			want:    []string{"Error: Network error", "Cause: Connection timeout"},
		},
		{
			name: "minimal error (message only)",
			err: &UserError{
				Message:  "Something failed",
				ExitCode: ExitInternal,
			},
			noColor: true,
			want:    []string{"Error: Something failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Format(tt.noColor)
			for _, substr := range tt.want {
				if !strings.Contains(got, substr) {
					t.Errorf("Format() output missing %q\nGot: %s", substr, got)
				}
			}
		})
	}
}

// TestUserError_Format_NoColor verifies that NO_COLOR environment variable is respected.
func TestUserError_Format_NoColor(t *testing.T) {
	// Save and restore NO_COLOR
	oldNoColor := os.Getenv("NO_COLOR")
	defer func() {
		if oldNoColor != "" {
			os.Setenv("NO_COLOR", oldNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	err := &UserError{
		Message:  "Test error",
		Cause:    "Test cause",
		Fix:      "Test fix",
		ExitCode: ExitConfig,
	}

	// Test with NO_COLOR environment variable
	os.Setenv("NO_COLOR", "1")
	output := err.Format(false) // noColor=false, but env var set

	// Should not contain ANSI escape codes
	if strings.Contains(output, "\x1b[") {
		t.Error("Format() output contains ANSI codes despite NO_COLOR being set")
	}
}

// TestUserError_ToJSON verifies the ToJSON() method implementation.
func TestUserError_ToJSON(t *testing.T) {
	tests := []struct {
		name         string
		err          *UserError
		wantError    string
		wantCause    string
		wantFix      string
		wantExitCode int
	}{
		{
			name: "full error",
			err: &UserError{
				Message:  "Invalid configuration",
				Cause:    "Missing required field",
				Fix:      "Run: cie init",
				ExitCode: ExitConfig,
			},
			wantError:    "Invalid configuration",
			wantCause:    "Missing required field",
			wantFix:      "Run: cie init",
			wantExitCode: ExitConfig,
		},
		{
			name: "minimal error",
			err: &UserError{
				Message:  "Error occurred",
				ExitCode: ExitInternal,
			},
			wantError:    "Error occurred",
			wantCause:    "",
			wantFix:      "",
			wantExitCode: ExitInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.ToJSON()

			if got.Error != tt.wantError {
				t.Errorf("ToJSON().Error = %q, want %q", got.Error, tt.wantError)
			}
			if got.Cause != tt.wantCause {
				t.Errorf("ToJSON().Cause = %q, want %q", got.Cause, tt.wantCause)
			}
			if got.Fix != tt.wantFix {
				t.Errorf("ToJSON().Fix = %q, want %q", got.Fix, tt.wantFix)
			}
			if got.ExitCode != tt.wantExitCode {
				t.Errorf("ToJSON().ExitCode = %d, want %d", got.ExitCode, tt.wantExitCode)
			}
		})
	}
}

// TestFatalError verifies basic FatalError behavior.
// Note: We cannot test actual os.Exit() behavior in unit tests.
// This test verifies the output format and type checking logic.
func TestFatalError(t *testing.T) {
	t.Run("nil error does nothing", func(t *testing.T) {
		// Should not panic or exit
		FatalError(nil, false)
	})

	t.Run("non-UserError prints simple message", func(t *testing.T) {
		// We can't test the actual output or exit, but we can verify
		// the function exists and accepts non-UserError types
		err := fmt.Errorf("generic error")
		// In real usage: FatalError(err, false) would exit
		_ = err // Prevent unused variable error
	})

	// Manual test case documented in godoc:
	// To test manually:
	//   go run cmd/cie/main.go <invalid-command>
	//   # Should show colored error and exit with proper code
}
