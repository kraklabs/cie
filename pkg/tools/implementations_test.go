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

// Integration tests for implementations.go functions.
// Run with: go test -tags=cozodb ./internal/cie/tools/...

package tools

import (
	"context"
	"strings"
	"testing"
)

func TestFindImplementations_Integration(t *testing.T) {
	db := openTestDB(t)

	// Setup test data with interface and implementations
	// Create interface
	insertTestType(t, db, "type1", "Handler", "interface", "internal/handler.go",
		`type Handler interface {
	Handle(ctx context.Context) error
	Close() error
}`, 10)

	// Create struct that implements the interface
	insertTestType(t, db, "type2", "RequestHandler", "struct", "internal/request.go",
		`type RequestHandler struct {
	service *Service
}`, 20)

	// Create methods for the struct
	insertTestFunction(t, db, "func1", "RequestHandler.Handle", "internal/request.go",
		"func (h *RequestHandler) Handle(ctx context.Context) error",
		"func (h *RequestHandler) Handle(ctx context.Context) error { return nil }", 30)
	insertTestFunction(t, db, "func2", "RequestHandler.Close", "internal/request.go",
		"func (h *RequestHandler) Close() error",
		"func (h *RequestHandler) Close() error { return nil }", 40)

	// Create another implementing struct
	insertTestType(t, db, "type3", "MockHandler", "struct", "internal/mock.go",
		`type MockHandler struct {}`, 10)
	insertTestFunction(t, db, "func3", "MockHandler.Handle", "internal/mock.go",
		"func (m *MockHandler) Handle(ctx context.Context) error",
		"func (m *MockHandler) Handle(ctx context.Context) error { return nil }", 20)
	insertTestFunction(t, db, "func4", "MockHandler.Close", "internal/mock.go",
		"func (m *MockHandler) Close() error",
		"func (m *MockHandler) Close() error { return nil }", 30)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	tests := []struct {
		name        string
		args        FindImplementationsArgs
		wantContain []string
	}{
		{
			name: "find implementations of Handler",
			args: FindImplementationsArgs{
				InterfaceName: "Handler",
				Limit:         10,
			},
			wantContain: []string{"Handler", "Interface"}, // Note: "Interface" capitalized in output
		},
		{
			name: "interface not found falls back to text search",
			args: FindImplementationsArgs{
				InterfaceName: "NonExistent",
				Limit:         10,
			},
			wantContain: []string{"NonExistent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FindImplementations(ctx, client, tt.args)
			if err != nil {
				t.Fatalf("FindImplementations() error = %v", err)
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(result.Text, want) {
					t.Errorf("FindImplementations() should contain %q, got:\n%s", want, result.Text)
				}
			}
		})
	}
}

func TestFindImplementations_EmptyInterfaceName(t *testing.T) {
	db := openTestDB(t)
	client := NewTestCIEClient(db)
	ctx := context.Background()

	result, err := FindImplementations(ctx, client, FindImplementationsArgs{InterfaceName: ""})
	if err != nil {
		t.Fatalf("FindImplementations() error = %v", err)
	}

	if !result.IsError {
		t.Error("FindImplementations() should return error for empty interface name")
	}
	if !strings.Contains(result.Text, "required") {
		t.Errorf("Error should mention 'required', got: %s", result.Text)
	}
}

func TestExtractMethodNames(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []string
	}{
		{
			name: "go interface with methods",
			code: `type Handler interface {
	Handle(ctx context.Context) error
	Close() error
}`,
			want: []string{"Handle", "Close"},
		},
		{
			name: "go interface with single method",
			code: `type Reader interface {
	Read(p []byte) (n int, err error)
}`,
			want: []string{"Read"},
		},
		{
			name: "empty interface",
			code: `type Empty interface {}`,
			want: nil,
		},
		{
			name: "typescript interface",
			code: `interface Handler {
	handle(): Promise<void>;
	close(): void;
}`,
			want: nil, // Only matches Go-style uppercase methods
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMethodNames(tt.code)
			if len(got) != len(tt.want) {
				t.Errorf("extractMethodNames() returned %d methods, want %d", len(got), len(tt.want))
				t.Errorf("got: %v, want: %v", got, tt.want)
				return
			}
			for i, m := range got {
				if m != tt.want[i] {
					t.Errorf("extractMethodNames()[%d] = %q, want %q", i, m, tt.want[i])
				}
			}
		})
	}
}

func TestFindImplementationsArgs_Defaults(t *testing.T) {
	args := FindImplementationsArgs{
		InterfaceName: "Handler",
	}

	if args.PathPattern != "" {
		t.Error("Default PathPattern should be empty")
	}
	if args.Limit != 0 {
		t.Error("Default Limit should be 0 (filled by function)")
	}
}
