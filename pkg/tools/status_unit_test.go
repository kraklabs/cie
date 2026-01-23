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

// Unit tests for status.go helper functions.
package tools

import (
	"context"
	"testing"
)

func TestIndexStatus_ErrorFormatting(t *testing.T) {
	state := &indexStatusState{
		ctx:    context.Background(),
		client: &CIEClient{BaseURL: "http://localhost:8080"},
		errors: []string{"query1 failed", "query2 timeout"},
	}

	output := state.formatErrors()

	if output == "" {
		t.Error("expected non-empty error output")
	}
	assertContains(t, output, "query1 failed")
	assertContains(t, output, "query2 timeout")
	assertContains(t, output, "Query Errors")
}

func TestIndexStatus_EmptyIndexHelp(t *testing.T) {
	help := formatEmptyIndexHelp()

	assertContains(t, help, "Index is empty")
	assertContains(t, help, "cie index")
	assertContains(t, help, "Possible causes")
}

func TestIndexCounts_Formatting(t *testing.T) {
	tests := []struct {
		name   string
		counts indexCounts
		want   []string
	}{
		{
			name: "with embeddings and HNSW",
			counts: indexCounts{
				files:      100,
				functions:  500,
				embeddings: 500,
				hasHNSW:    true,
			},
			want: []string{"100", "500", "HNSW Index", "ready"},
		},
		{
			name: "without embeddings",
			counts: indexCounts{
				files:      50,
				functions:  200,
				embeddings: 0,
				hasHNSW:    false,
			},
			want: []string{"50", "200", "No embeddings found"},
		},
		{
			name: "embeddings without HNSW",
			counts: indexCounts{
				files:      75,
				functions:  300,
				embeddings: 300,
				hasHNSW:    false,
			},
			want: []string{"75", "300", "HNSW index missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &indexStatusState{
				ctx:    context.Background(),
				client: &CIEClient{},
			}
			output := state.formatOverallStats(tt.counts)

			for _, expected := range tt.want {
				assertContains(t, output, expected)
			}
		})
	}
}
