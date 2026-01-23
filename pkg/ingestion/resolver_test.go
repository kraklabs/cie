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
