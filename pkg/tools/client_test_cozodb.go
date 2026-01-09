// Copyright 2025 KrakLabs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

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
