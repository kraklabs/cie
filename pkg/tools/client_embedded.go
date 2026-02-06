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
	"fmt"

	"github.com/kraklabs/cie/pkg/storage"
)

// EmbeddedQuerier implements Querier by wrapping a storage.EmbeddedBackend.
// This allows tools to query directly against an embedded CozoDB instance
// without going through the HTTP API.
type EmbeddedQuerier struct {
	backend *storage.EmbeddedBackend
}

// NewEmbeddedQuerier creates a new EmbeddedQuerier wrapping the given backend.
func NewEmbeddedQuerier(backend *storage.EmbeddedBackend) *EmbeddedQuerier {
	return &EmbeddedQuerier{backend: backend}
}

// Query executes a Datalog query against the embedded backend.
func (q *EmbeddedQuerier) Query(ctx context.Context, script string) (*QueryResult, error) {
	result, err := q.backend.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("embedded query: %w", err)
	}

	return &QueryResult{
		Headers: result.Headers,
		Rows:    result.Rows,
	}, nil
}

// QueryRaw executes a query and returns raw results as a map.
func (q *EmbeddedQuerier) QueryRaw(ctx context.Context, script string) (map[string]any, error) {
	result, err := q.backend.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("embedded raw query: %w", err)
	}

	return map[string]any{
		"Headers": result.Headers,
		"Rows":    result.Rows,
	}, nil
}
