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
//go:build cozodb
// +build cozodb

// Test infrastructure for CozoDB integration tests.
// This file provides a test client that wraps CozoDB directly for testing.

package tools

import (
	"context"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// TestCIEClient wraps a CozoDB instance for integration testing.
// It implements the same Query interface as CIEClient but executes locally.
type TestCIEClient struct {
	DB *cozo.CozoDB
}

// NewTestCIEClient creates a new test client wrapping a CozoDB instance.
func NewTestCIEClient(db *cozo.CozoDB) *TestCIEClient {
	return &TestCIEClient{DB: db}
}

// Query executes a CozoScript query directly against the embedded CozoDB.
func (c *TestCIEClient) Query(ctx context.Context, script string) (*QueryResult, error) {
	result, err := c.DB.Run(script, nil)
	if err != nil {
		return nil, err
	}

	// Convert cozo.Result to QueryResult
	return &QueryResult{
		Headers: result.Headers,
		Rows:    result.Rows,
	}, nil
}

// QueryRaw executes a query and returns raw results.
func (c *TestCIEClient) QueryRaw(ctx context.Context, script string) (map[string]any, error) {
	result, err := c.DB.Run(script, nil)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"Headers": result.Headers,
		"Rows":    result.Rows,
	}, nil
}
