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

// Integration tests for services.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestListServices_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with proto-like patterns
	// Note: ListServices looks for .proto files and gRPC service definitions
	insertTestFile(t, db, "file1", "api/proto/user.proto", "proto")
	insertTestFile(t, db, "file2", "api/proto/order.proto", "proto")

	// Proto-style service definitions (would be parsed from .proto files)
	insertTestFunction(t, db, "func1", "UserService", "api/proto/user.proto",
		"service UserService",
		`service UserService {
  rpc GetUser(GetUserRequest) returns (User);
  rpc CreateUser(CreateUserRequest) returns (User);
  rpc DeleteUser(DeleteUserRequest) returns (Empty);
}`, 10)

	insertTestFunction(t, db, "func2", "OrderService", "api/proto/order.proto",
		"service OrderService",
		`service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (Order);
  rpc GetOrder(GetOrderRequest) returns (Order);
}`, 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		pathPattern string
		serviceName string
		wantContain []string
	}{
		{
			name:        "list all services",
			pathPattern: "",
			serviceName: "",
			wantContain: []string{"proto"},
		},
		{
			name:        "filter by service name",
			pathPattern: "",
			serviceName: "UserService",
			wantContain: []string{"User"},
		},
		{
			name:        "filter by path",
			pathPattern: "order",
			serviceName: "",
			wantContain: []string{"order"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ListServices(ctx, client, tt.pathPattern, tt.serviceName)
			if err != nil {
				t.Fatalf("ListServices() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(strings.ToLower(result.Text), strings.ToLower(want)) {
					t.Errorf("ListServices() should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestRoleFiltersWithCustom(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		custom  map[string]RolePattern
		wantLen int
	}{
		{
			name:    "source role",
			role:    "source",
			custom:  nil,
			wantLen: 2, // test and generated exclusions
		},
		{
			name:    "any role",
			role:    "any",
			custom:  nil,
			wantLen: 0,
		},
		{
			name: "source with custom pattern (custom not applied)",
			role: "source",
			custom: map[string]RolePattern{
				"internal": {FilePattern: "internal/", Description: "internal files"},
			},
			wantLen: 2, // source role not in custom map, falls back to RoleFilters("source")
		},
		{
			name: "custom internal role",
			role: "internal",
			custom: map[string]RolePattern{
				"internal": {FilePattern: "internal/", Description: "internal files"},
			},
			wantLen: 1, // only the custom file pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoleFiltersWithCustom(tt.role, tt.custom)
			if len(got) != tt.wantLen {
				t.Errorf("RoleFiltersWithCustom() returned %d filters, want %d", len(got), tt.wantLen)
			}
		})
	}
}
