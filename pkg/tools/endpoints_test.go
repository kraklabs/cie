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

// Integration tests for endpoints.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestListEndpoints_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with HTTP route patterns
	insertTestFile(t, db, "file1", "internal/routes/api.go", "go")
	insertTestFile(t, db, "file2", "internal/routes/users.go", "go")

	// Gin-style routes
	insertTestFunction(t, db, "func1", "RegisterRoutes", "internal/routes/api.go",
		"func RegisterRoutes(r *gin.Engine)",
		`func RegisterRoutes(r *gin.Engine) {
	r.GET("/api/health", healthHandler)
	r.POST("/api/users", createUser)
	r.PUT("/api/users/:id", updateUser)
	r.DELETE("/api/users/:id", deleteUser)
}`, 10)

	// Echo-style routes
	insertTestFunction(t, db, "func2", "SetupUserRoutes", "internal/routes/users.go",
		"func SetupUserRoutes(e *echo.Echo)",
		`func SetupUserRoutes(e *echo.Echo) {
	e.GET("/users", listUsers)
	e.POST("/users", createUser)
}`, 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        ListEndpointsArgs
		wantContain []string
	}{
		{
			name:        "list all endpoints",
			args:        ListEndpointsArgs{Limit: 100},
			wantContain: []string{"GET", "POST"},
		},
		{
			name:        "filter by GET method",
			args:        ListEndpointsArgs{Method: "GET", Limit: 100},
			wantContain: []string{"GET"},
		},
		{
			name:        "filter by file path pattern",
			args:        ListEndpointsArgs{PathPattern: "api.go", Limit: 100},
			wantContain: []string{"health", "users"},
		},
		{
			name:        "filter by endpoint path",
			args:        ListEndpointsArgs{PathFilter: "health", Limit: 100},
			wantContain: []string{"/api/health"},
		},
		{
			name:        "filter by endpoint path connections",
			args:        ListEndpointsArgs{PathFilter: "users/:id", Limit: 100},
			wantContain: []string{"/api/users/:id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ListEndpoints(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("ListEndpoints() error = %v", err)
			}

			if result.IsError {
				t.Errorf("ListEndpoints() returned error: %s", result.Text)
				return
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("ListEndpoints() should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestExtractPathPrefix(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/api/users", "/api/users"}, // 2 segments -> returns both
		{"/api/v1/users", "/api/v1"}, // 3 segments -> returns first 2
		{"/users", "/users"},         // 1 segment -> returns it
		{"/", "/"},                   // empty after trim -> returns original
		{"users", "/users"},          // no leading slash -> still gets 1 segment
		{"", "/"},                    // empty string -> returns "/"
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractPathPrefix(tt.path)
			if got != tt.want {
				t.Errorf("extractPathPrefix(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
