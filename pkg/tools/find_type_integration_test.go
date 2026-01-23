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

// Integration tests for find_type.go functions.
// Run with: go test -tags=cozodb ./pkg/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestFindType_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with various types
	insertTestType(t, db, "type1", "UserService", "struct", "internal/service/user.go",
		`type UserService struct {
	repo *UserRepository
	cache *Cache
}`, 10)

	insertTestType(t, db, "type2", "Handler", "interface", "internal/handler.go",
		`type Handler interface {
	Handle(ctx context.Context) error
}`, 20)

	insertTestType(t, db, "type3", "UserRepository", "struct", "internal/repository/user.go",
		`type UserRepository struct {
	db *sql.DB
}`, 15)

	insertTestType(t, db, "type4", "Config", "struct", "internal/config/config.go",
		`type Config struct {
	Port int
	Host string
}`, 5)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        FindTypeArgs
		wantContain []string
		wantExclude []string
	}{
		{
			name: "find by exact name",
			args: FindTypeArgs{
				Name:  "UserService",
				Limit: 10,
			},
			wantContain: []string{"UserService", "struct", "user.go"},
			wantExclude: []string{"Handler", "Config"},
		},
		{
			name: "find by partial name",
			args: FindTypeArgs{
				Name:  "User",
				Limit: 10,
			},
			wantContain: []string{"UserService", "UserRepository"},
			wantExclude: []string{"Config"},
		},
		{
			name: "filter by kind struct",
			args: FindTypeArgs{
				Name:  "User",
				Kind:  "struct",
				Limit: 10,
			},
			wantContain: []string{"UserService", "UserRepository", "struct"},
			wantExclude: []string{"interface"},
		},
		{
			name: "filter by kind interface",
			args: FindTypeArgs{
				Name:  "Handler",
				Kind:  "interface",
				Limit: 10,
			},
			wantContain: []string{"Handler", "interface"},
			wantExclude: []string{"struct"},
		},
		{
			name: "filter by path pattern",
			args: FindTypeArgs{
				Name:        "User",
				PathPattern: "service",
				Limit:       10,
			},
			wantContain: []string{"UserService", "service/user.go"},
			wantExclude: []string{"UserRepository"},
		},
		{
			name: "empty name error",
			args: FindTypeArgs{
				Name: "",
			},
			wantContain: []string{"required"},
		},
		{
			name: "no results",
			args: FindTypeArgs{
				Name:  "NonExistentType",
				Limit: 10,
			},
			wantContain: []string{"No types found"},
		},
		{
			name: "limit results",
			args: FindTypeArgs{
				Name:  "User", // Match UserService and UserRepository
				Kind:  "struct",
				Limit: 1, // Only return 1 result
			},
			wantContain: []string{"struct", "User"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindType(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("FindType() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("FindType() should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("FindType() should NOT contain %q, got:\n%s", exclude, result.Text)
				}
			}
		})
	}
}

func TestGetTypeCode_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data
	insertTestType(t, db, "type1", "UserService", "struct", "internal/service/user.go",
		`type UserService struct {
	repo *UserRepository
	cache *Cache
	logger *Logger
}

func (s *UserService) GetUser(id string) (*User, error) {
	return s.repo.Find(id)
}`, 10)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		typeName    string
		filePath    string
		wantContain []string
		wantExclude []string
	}{
		{
			name:        "get type code by exact name",
			typeName:    "UserService",
			filePath:    "",
			wantContain: []string{"UserService", "UserRepository", "Cache", "Logger"},
		},
		{
			name:        "type not found",
			typeName:    "NonExistentType",
			filePath:    "",
			wantContain: []string{"not found"},
		},
		{
			name:        "empty name error",
			typeName:    "",
			filePath:    "",
			wantContain: []string{"required"},
		},
		{
			name:        "get type code with file path filter",
			typeName:    "UserService",
			filePath:    "internal/service/user.go",
			wantContain: []string{"UserService", "struct"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetTypeCode(ctx, client, tt.typeName, tt.filePath)
			if err != nil {
				t.Fatalf("GetTypeCode() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("GetTypeCode() should contain %q, got:\n%s", want, result.Text)
				}
			}

			for _, exclude := range tt.wantExclude {
				if strings.Contains(result.Text, exclude) {
					t.Errorf("GetTypeCode() should NOT contain %q", exclude)
				}
			}
		})
	}
}
