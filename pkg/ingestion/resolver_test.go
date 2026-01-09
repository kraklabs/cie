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

package ingestion

import (
	"testing"
)

func TestCallResolver_BuildIndex(t *testing.T) {
	// Create test data mimicking two packages:
	// - internal/handlers (package handlers)
	// - internal/routes (package routes)

	files := []FileEntity{
		{ID: "file:handlers/user.go", Path: "internal/handlers/user.go", Language: "go"},
		{ID: "file:routes/auth.go", Path: "internal/routes/auth.go", Language: "go"},
	}

	functions := []FunctionEntity{
		{ID: "fn:HandleUser", Name: "HandleUser", FilePath: "internal/handlers/user.go"},
		{ID: "fn:ValidateToken", Name: "ValidateToken", FilePath: "internal/handlers/user.go"},
		{ID: "fn:RegisterAuthRoutes", Name: "RegisterAuthRoutes", FilePath: "internal/routes/auth.go"},
	}

	imports := []ImportEntity{
		{
			ID:         GenerateImportID("internal/routes/auth.go", "project/internal/handlers"),
			FilePath:   "internal/routes/auth.go",
			ImportPath: "project/internal/handlers",
			Alias:      "",
			StartLine:  3,
		},
	}

	packageNames := map[string]string{
		"internal/handlers/user.go": "handlers",
		"internal/routes/auth.go":   "routes",
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)

	// Verify package index
	pkgs, funcs, imps := resolver.Stats()
	if pkgs != 2 {
		t.Errorf("expected 2 packages, got %d", pkgs)
	}
	if funcs != 3 {
		t.Errorf("expected 3 functions indexed, got %d", funcs)
	}
	if imps != 1 {
		t.Errorf("expected 1 import indexed, got %d", imps)
	}
}

func TestCallResolver_ResolveCalls_QualifiedCall(t *testing.T) {
	// Setup: routes/auth.go imports handlers and calls handlers.HandleUser()

	files := []FileEntity{
		{ID: "file:handlers/user.go", Path: "internal/handlers/user.go", Language: "go"},
		{ID: "file:routes/auth.go", Path: "internal/routes/auth.go", Language: "go"},
	}

	functions := []FunctionEntity{
		{ID: "fn:HandleUser", Name: "HandleUser", FilePath: "internal/handlers/user.go"},
		{ID: "fn:RegisterAuthRoutes", Name: "RegisterAuthRoutes", FilePath: "internal/routes/auth.go"},
	}

	imports := []ImportEntity{
		{
			ID:         GenerateImportID("internal/routes/auth.go", "project/internal/handlers"),
			FilePath:   "internal/routes/auth.go",
			ImportPath: "project/internal/handlers",
			Alias:      "",
			StartLine:  3,
		},
	}

	packageNames := map[string]string{
		"internal/handlers/user.go": "handlers",
		"internal/routes/auth.go":   "routes",
	}

	// Unresolved call: handlers.HandleUser() from RegisterAuthRoutes
	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:RegisterAuthRoutes",
			CalleeName: "handlers.HandleUser",
			FilePath:   "internal/routes/auth.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call, got %d", len(resolvedCalls))
	}

	if resolvedCalls[0].CallerID != "fn:RegisterAuthRoutes" {
		t.Errorf("expected caller fn:RegisterAuthRoutes, got %s", resolvedCalls[0].CallerID)
	}
	if resolvedCalls[0].CalleeID != "fn:HandleUser" {
		t.Errorf("expected callee fn:HandleUser, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveCalls_UnexportedIgnored(t *testing.T) {
	// Setup: unexported function calls should not be resolved cross-package

	files := []FileEntity{
		{ID: "file:handlers/user.go", Path: "internal/handlers/user.go", Language: "go"},
		{ID: "file:routes/auth.go", Path: "internal/routes/auth.go", Language: "go"},
	}

	functions := []FunctionEntity{
		{ID: "fn:privateFunc", Name: "privateFunc", FilePath: "internal/handlers/user.go"},
		{ID: "fn:RegisterAuthRoutes", Name: "RegisterAuthRoutes", FilePath: "internal/routes/auth.go"},
	}

	imports := []ImportEntity{
		{
			ID:         GenerateImportID("internal/routes/auth.go", "project/internal/handlers"),
			FilePath:   "internal/routes/auth.go",
			ImportPath: "project/internal/handlers",
			Alias:      "",
			StartLine:  3,
		},
	}

	packageNames := map[string]string{
		"internal/handlers/user.go": "handlers",
		"internal/routes/auth.go":   "routes",
	}

	// Unresolved call: handlers.privateFunc() - should NOT resolve (unexported)
	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:RegisterAuthRoutes",
			CalleeName: "handlers.privateFunc",
			FilePath:   "internal/routes/auth.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 0 {
		t.Errorf("expected 0 resolved calls for unexported function, got %d", len(resolvedCalls))
	}
}

func TestCallResolver_ResolveCalls_AliasedImport(t *testing.T) {
	// Setup: import with alias - import h "project/internal/handlers"

	files := []FileEntity{
		{ID: "file:handlers/user.go", Path: "internal/handlers/user.go", Language: "go"},
		{ID: "file:routes/auth.go", Path: "internal/routes/auth.go", Language: "go"},
	}

	functions := []FunctionEntity{
		{ID: "fn:HandleUser", Name: "HandleUser", FilePath: "internal/handlers/user.go"},
		{ID: "fn:RegisterAuthRoutes", Name: "RegisterAuthRoutes", FilePath: "internal/routes/auth.go"},
	}

	imports := []ImportEntity{
		{
			ID:         GenerateImportID("internal/routes/auth.go", "project/internal/handlers"),
			FilePath:   "internal/routes/auth.go",
			ImportPath: "project/internal/handlers",
			Alias:      "h", // aliased import
			StartLine:  3,
		},
	}

	packageNames := map[string]string{
		"internal/handlers/user.go": "handlers",
		"internal/routes/auth.go":   "routes",
	}

	// Unresolved call: h.HandleUser() (using alias)
	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:RegisterAuthRoutes",
			CalleeName: "h.HandleUser",
			FilePath:   "internal/routes/auth.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call with aliased import, got %d", len(resolvedCalls))
	}

	if resolvedCalls[0].CalleeID != "fn:HandleUser" {
		t.Errorf("expected callee fn:HandleUser, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_NoDuplicates(t *testing.T) {
	// Ensure no duplicate edges are created

	files := []FileEntity{
		{ID: "file:handlers/user.go", Path: "internal/handlers/user.go", Language: "go"},
		{ID: "file:routes/auth.go", Path: "internal/routes/auth.go", Language: "go"},
	}

	functions := []FunctionEntity{
		{ID: "fn:HandleUser", Name: "HandleUser", FilePath: "internal/handlers/user.go"},
		{ID: "fn:RegisterAuthRoutes", Name: "RegisterAuthRoutes", FilePath: "internal/routes/auth.go"},
	}

	imports := []ImportEntity{
		{
			ID:         GenerateImportID("internal/routes/auth.go", "project/internal/handlers"),
			FilePath:   "internal/routes/auth.go",
			ImportPath: "project/internal/handlers",
			Alias:      "",
			StartLine:  3,
		},
	}

	packageNames := map[string]string{
		"internal/handlers/user.go": "handlers",
		"internal/routes/auth.go":   "routes",
	}

	// Same call twice (should deduplicate)
	unresolvedCalls := []UnresolvedCall{
		{CallerID: "fn:RegisterAuthRoutes", CalleeName: "handlers.HandleUser", FilePath: "internal/routes/auth.go", Line: 10},
		{CallerID: "fn:RegisterAuthRoutes", CalleeName: "handlers.HandleUser", FilePath: "internal/routes/auth.go", Line: 15},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Errorf("expected 1 deduplicated call, got %d", len(resolvedCalls))
	}
}
