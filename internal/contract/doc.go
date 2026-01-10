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

// Package contract provides validation constants and utilities for CIE.
//
// This internal package contains configuration constants and validation
// functions used throughout CIE. It provides a minimal subset of validation
// logic needed for standalone CIE operation.
//
// # Batch Size Limits
//
// CIE enforces soft limits on batch operations to prevent memory exhaustion:
//
//	// Default limit is 64 MiB
//	limit := contract.SoftLimitBytes()
//
//	// Validate a batch script before execution
//	result := contract.ValidateBatchScript(script)
//	if !result.OK {
//	    log.Printf("Validation failed: %s", result.Message)
//	}
//
// # Configuration via Environment
//
// The soft limit can be adjusted via the CIE_SOFT_LIMIT_BYTES environment
// variable. This is useful for environments with limited memory or when
// processing very large batches:
//
//	export CIE_SOFT_LIMIT_BYTES=33554432  # 32 MiB
//
// If the environment variable is not set or invalid, the default limit
// of 64 MiB (DefaultSoftLimitBytes) is used.
//
// # Constants
//
// The package exports these constants:
//
//   - DefaultSoftLimitBytes: Baseline soft limit (64 MiB)
//   - RequestIDMaxBytes: Maximum length for request identifiers (128 bytes)
package contract
