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

// Package storage provides storage backend abstractions for CIE.
//
// This package defines the Backend interface that allows CIE to work with
// different storage implementations:
//   - EmbeddedBackend: Local CozoDB for standalone/open-source use
//   - RemoteBackend: Connects to CIE Enterprise (Primary Hub/Edge Cache)
//
// The same MCP tools work with any backend implementation.
package storage

import (
	"context"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// Backend is the interface that all storage backends must implement.
// It provides methods for executing queries and mutations on the code index.
type Backend interface {
	// Query executes a read-only Datalog query and returns the results.
	Query(ctx context.Context, datalog string) (*QueryResult, error)

	// Execute runs a Datalog mutation (insert, update, delete).
	Execute(ctx context.Context, datalog string) error

	// Close releases any resources held by the backend.
	Close() error
}

// QueryResult represents the result of a Datalog query.
type QueryResult struct {
	Headers []string
	Rows    [][]any
}

// ToNamedRows converts QueryResult to CozoDB NamedRows for compatibility.
func (r *QueryResult) ToNamedRows() cozo.NamedRows {
	return cozo.NamedRows{
		Headers: r.Headers,
		Rows:    r.Rows,
	}
}

// FromNamedRows converts CozoDB NamedRows to QueryResult.
func FromNamedRows(nr cozo.NamedRows) *QueryResult {
	return &QueryResult{
		Headers: nr.Headers,
		Rows:    nr.Rows,
	}
}
