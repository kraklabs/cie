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
	"context"
)

// MockCIEClient is a mock implementation of the Querier interface for unit testing.
// It allows configurable responses without requiring CozoDB C bindings.
//
// Usage:
//
//	client := NewMockClientWithResults(
//	    []string{"name", "file"},
//	    [][]any{{"MyFunc", "/path/file.go"}},
//	)
//	result, err := client.Query(ctx, "some script")
type MockCIEClient struct {
	// QueryFunc is called when Query() is invoked. If nil, returns empty result.
	QueryFunc func(ctx context.Context, script string) (*QueryResult, error)

	// QueryRawFunc is called when QueryRaw() is invoked. If nil, returns empty map.
	QueryRawFunc func(ctx context.Context, script string) (map[string]any, error)
}

// Query implements the Querier interface.
func (m *MockCIEClient) Query(ctx context.Context, script string) (*QueryResult, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, script)
	}
	return &QueryResult{Headers: []string{}, Rows: [][]any{}}, nil
}

// QueryRaw implements the Querier interface.
func (m *MockCIEClient) QueryRaw(ctx context.Context, script string) (map[string]any, error) {
	if m.QueryRawFunc != nil {
		return m.QueryRawFunc(ctx, script)
	}
	return map[string]any{"Headers": []string{}, "Rows": [][]any{}}, nil
}

// NewMockClientWithResults creates a mock client that returns the specified results.
// This is useful for testing successful query scenarios.
//
// Example:
//
//	client := NewMockClientWithResults(
//	    []string{"name", "file_path"},
//	    [][]any{
//	        {"FindFunction", "/pkg/tools/search.go"},
//	        {"SearchText", "/pkg/tools/search.go"},
//	    },
//	)
func NewMockClientWithResults(headers []string, rows [][]any) *MockCIEClient {
	result := NewMockQueryResult(headers, rows)
	return &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return result, nil
		},
		QueryRawFunc: func(ctx context.Context, script string) (map[string]any, error) {
			return map[string]any{
				"Headers": result.Headers,
				"Rows":    result.Rows,
			}, nil
		},
	}
}

// NewMockClientWithError creates a mock client that returns the specified error.
// This is useful for testing error handling scenarios.
//
// Example:
//
//	client := NewMockClientWithError(fmt.Errorf("database connection failed"))
//	_, err := client.Query(ctx, "?[name] := *cie_function {name}")
//	// err will be the configured error
func NewMockClientWithError(err error) *MockCIEClient {
	return &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return nil, err
		},
		QueryRawFunc: func(ctx context.Context, script string) (map[string]any, error) {
			return nil, err
		},
	}
}

// NewMockClientEmpty creates a mock client that returns empty results.
// This is useful for testing "no results found" scenarios.
//
// Example:
//
//	client := NewMockClientEmpty()
//	result, _ := client.Query(ctx, "?[name] := *cie_function {name}")
//	// result.Rows will be empty
func NewMockClientEmpty() *MockCIEClient {
	return NewMockClientWithResults([]string{}, [][]any{})
}

// NewMockQueryResult creates a QueryResult with the specified headers and rows.
// This matches the structure returned by CozoDB queries.
//
// Example:
//
//	result := NewMockQueryResult(
//	    []string{"name", "file_path", "line"},
//	    [][]any{
//	        {"HandleRequest", "/internal/handler.go", 42},
//	        {"ProcessData", "/internal/processor.go", 15},
//	    },
//	)
func NewMockQueryResult(headers []string, rows [][]any) *QueryResult {
	return &QueryResult{
		Headers: headers,
		Rows:    rows,
	}
}

// NewMockClientCustom creates a mock client with custom Query and QueryRaw functions.
// This provides maximum flexibility for complex test scenarios.
//
// Example:
//
//	client := NewMockClientCustom(
//	    func(ctx context.Context, script string) (*QueryResult, error) {
//	        if strings.Contains(script, "error") {
//	            return nil, fmt.Errorf("query failed")
//	        }
//	        return NewMockQueryResult([]string{"result"}, [][]any{{"ok"}}), nil
//	    },
//	    nil, // Use default QueryRaw behavior
//	)
func NewMockClientCustom(
	queryFunc func(ctx context.Context, script string) (*QueryResult, error),
	queryRawFunc func(ctx context.Context, script string) (map[string]any, error),
) *MockCIEClient {
	return &MockCIEClient{
		QueryFunc:    queryFunc,
		QueryRawFunc: queryRawFunc,
	}
}
