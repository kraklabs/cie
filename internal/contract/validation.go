// Copyright 2025 KrakLabs
// SPDX-License-Identifier: Apache-2.0

// Package contract provides validation constants and utilities for CIE.
// This is a minimal subset for standalone operation.
package contract

import (
	"os"
	"strconv"
)

const (
	// DefaultSoftLimitBytes is the baseline soft limit for batch operations.
	DefaultSoftLimitBytes = 64 << 20 // 64 MiB

	// RequestIDMaxBytes is the maximum length for request_id field.
	RequestIDMaxBytes = 128
)

// SoftLimitBytes returns the effective soft limit for batch_script size.
// Controlled via env CIE_SOFT_LIMIT_BYTES; falls back to DefaultSoftLimitBytes.
func SoftLimitBytes() int {
	if v := os.Getenv("CIE_SOFT_LIMIT_BYTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultSoftLimitBytes
}

// ValidationResult represents the result of a validation check.
type ValidationResult struct {
	OK      bool
	Message string
}

// ValidateBatchScript performs basic validation on a batch script.
// For standalone CIE, this just checks size limits.
func ValidateBatchScript(script string) *ValidationResult {
	if len(script) > SoftLimitBytes() {
		return &ValidationResult{
			OK:      false,
			Message: "batch_script exceeds soft limit",
		}
	}
	return &ValidationResult{OK: true}
}
