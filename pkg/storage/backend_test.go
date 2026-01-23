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

//go:build cgo

package storage

import (
	"testing"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// TestBackendInterface verifies that EmbeddedBackend implements the Backend interface.
func TestBackendInterface(t *testing.T) {
	var _ Backend = &EmbeddedBackend{}
}

// TestQueryResult_ToNamedRows tests the conversion from QueryResult to CozoDB NamedRows.
func TestQueryResult_ToNamedRows(t *testing.T) {
	qr := &QueryResult{
		Headers: []string{"id", "name", "value"},
		Rows: [][]any{
			{"1", "test", 42},
			{"2", "example", 100},
		},
	}

	nr := qr.ToNamedRows()

	if len(nr.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(nr.Headers))
	}
	if nr.Headers[0] != "id" || nr.Headers[1] != "name" || nr.Headers[2] != "value" {
		t.Errorf("headers mismatch: got %v", nr.Headers)
	}
	if len(nr.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(nr.Rows))
	}
	if len(nr.Rows[0]) != 3 {
		t.Errorf("expected 3 columns in first row, got %d", len(nr.Rows[0]))
	}
}

// TestFromNamedRows tests the conversion from CozoDB NamedRows to QueryResult.
func TestFromNamedRows(t *testing.T) {
	nr := cozo.NamedRows{
		Headers: []string{"function_id", "name"},
		Rows: [][]any{
			{"fn1", "TestFunc"},
			{"fn2", "AnotherFunc"},
		},
	}

	qr := FromNamedRows(nr)

	if qr == nil {
		t.Fatal("FromNamedRows returned nil")
	}
	if len(qr.Headers) != 2 {
		t.Errorf("expected 2 headers, got %d", len(qr.Headers))
	}
	if qr.Headers[0] != "function_id" || qr.Headers[1] != "name" {
		t.Errorf("headers mismatch: got %v", qr.Headers)
	}
	if len(qr.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(qr.Rows))
	}
	if qr.Rows[0][0] != "fn1" || qr.Rows[0][1] != "TestFunc" {
		t.Errorf("row data mismatch: got %v", qr.Rows[0])
	}
}
