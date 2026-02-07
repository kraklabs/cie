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

func TestCallResolver_ResolveInterfaceFieldCall(t *testing.T) {
	// Setup: Builder.Build calls b.writer.Write() where writer is type Writer
	// CozoDB implements Writer

	files := []FileEntity{
		{ID: "file:store.go", Path: "internal/store/store.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Builder.Build", Name: "Builder.Build", FilePath: "internal/store/store.go"},
		{ID: "fn:CozoDB.Write", Name: "CozoDB.Write", FilePath: "internal/store/store.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"internal/store/store.go": "store",
	}

	fields := []FieldEntity{
		{StructName: "Builder", FieldName: "writer", FieldType: "Writer", FilePath: "internal/store/store.go"},
	}
	implements := []ImplementsEdge{
		{TypeName: "CozoDB", InterfaceName: "Writer", FilePath: "internal/store/store.go"},
	}

	// Unresolved call: writer.Write from Builder.Build
	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Builder.Build",
			CalleeName: "writer.Write",
			FilePath:   "internal/store/store.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call via interface dispatch, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CallerID != "fn:Builder.Build" {
		t.Errorf("expected caller fn:Builder.Build, got %s", resolvedCalls[0].CallerID)
	}
	if resolvedCalls[0].CalleeID != "fn:CozoDB.Write" {
		t.Errorf("expected callee fn:CozoDB.Write, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveInterfaceFieldCall_MultipleImpls(t *testing.T) {
	// Writer implemented by CozoDB and FileStore → produces 2 CallsEdge

	files := []FileEntity{
		{ID: "file:store.go", Path: "internal/store/store.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Builder.Build", Name: "Builder.Build", FilePath: "internal/store/store.go"},
		{ID: "fn:CozoDB.Write", Name: "CozoDB.Write", FilePath: "internal/store/store.go"},
		{ID: "fn:FileStore.Write", Name: "FileStore.Write", FilePath: "internal/store/store.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"internal/store/store.go": "store",
	}

	fields := []FieldEntity{
		{StructName: "Builder", FieldName: "writer", FieldType: "Writer"},
	}
	implements := []ImplementsEdge{
		{TypeName: "CozoDB", InterfaceName: "Writer"},
		{TypeName: "FileStore", InterfaceName: "Writer"},
	}

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Builder.Build",
			CalleeName: "writer.Write",
			FilePath:   "internal/store/store.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 2 {
		t.Fatalf("expected 2 resolved calls (one per implementation), got %d", len(resolvedCalls))
	}

	calleeIDs := map[string]bool{}
	for _, call := range resolvedCalls {
		calleeIDs[call.CalleeID] = true
	}
	if !calleeIDs["fn:CozoDB.Write"] {
		t.Error("expected callee fn:CozoDB.Write")
	}
	if !calleeIDs["fn:FileStore.Write"] {
		t.Error("expected callee fn:FileStore.Write")
	}
}

func TestCallResolver_ResolveInterfaceFieldCall_NonInterfaceIgnored(t *testing.T) {
	// Field "name" is type "string" with no implements edges → 0 resolved calls

	files := []FileEntity{
		{ID: "file:store.go", Path: "internal/store/store.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Builder.Build", Name: "Builder.Build", FilePath: "internal/store/store.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"internal/store/store.go": "store",
	}

	// name field has type string — no implements edges for string
	fields := []FieldEntity{
		{StructName: "Builder", FieldName: "name", FieldType: "string"},
	}
	implements := []ImplementsEdge{} // no implements for string

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Builder.Build",
			CalleeName: "name.Foo",
			FilePath:   "internal/store/store.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 0 {
		t.Errorf("expected 0 resolved calls for non-interface field, got %d", len(resolvedCalls))
	}
}

func TestCallResolver_ResolveInterfaceFieldCall_ChainedAccess(t *testing.T) {
	// Test the critical case: "b.writer.Write" where "b" is the receiver variable
	// The parser captures the full expression including the receiver.
	// The resolver must skip the receiver and identify "writer" as the field name.

	files := []FileEntity{
		{ID: "file:store.go", Path: "internal/store/store.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Builder.Build", Name: "Builder.Build", FilePath: "internal/store/store.go"},
		{ID: "fn:CozoDB.Write", Name: "CozoDB.Write", FilePath: "internal/store/store.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"internal/store/store.go": "store",
	}

	fields := []FieldEntity{
		{StructName: "Builder", FieldName: "writer", FieldType: "Writer"},
	}
	implements := []ImplementsEdge{
		{TypeName: "CozoDB", InterfaceName: "Writer"},
	}

	// Callee name includes receiver prefix: "b.writer.Write"
	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Builder.Build",
			CalleeName: "b.writer.Write",
			FilePath:   "internal/store/store.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call via chained interface dispatch, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CalleeID != "fn:CozoDB.Write" {
		t.Errorf("expected callee fn:CozoDB.Write, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveInterfaceFieldCall_DeeplyChained(t *testing.T) {
	// Even deeper: "m.engine.querier.StoreFact" — should find "querier" in Engine's fields
	// This tests that we scan right-to-left for field name matches.

	files := []FileEntity{
		{ID: "file:memory.go", Path: "pkg/memory/memory.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Engine.Store", Name: "Engine.Store", FilePath: "pkg/memory/memory.go"},
		{ID: "fn:Client.StoreFact", Name: "Client.StoreFact", FilePath: "pkg/memory/client.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"pkg/memory/memory.go": "memory",
	}

	fields := []FieldEntity{
		{StructName: "Engine", FieldName: "querier", FieldType: "Querier"},
	}
	implements := []ImplementsEdge{
		{TypeName: "Client", InterfaceName: "Querier"},
	}

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Engine.Store",
			CalleeName: "m.engine.querier.StoreFact",
			FilePath:   "pkg/memory/memory.go",
			Line:       15,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call for deeply chained access, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CalleeID != "fn:Client.StoreFact" {
		t.Errorf("expected callee fn:Client.StoreFact, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveInterfaceCall_StandaloneFunction(t *testing.T) {
	// Standalone function `storeFact(client Querier, fact string)` calls client.StoreFact
	// No struct, no fields — resolution must go through signature params.

	files := []FileEntity{
		{ID: "file:tools.go", Path: "pkg/tools/tools.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{
			ID:        "fn:storeFact",
			Name:      "storeFact",
			FilePath:  "pkg/tools/tools.go",
			Signature: "func storeFact(client Querier, fact string) error",
		},
		{ID: "fn:CIEClient.StoreFact", Name: "CIEClient.StoreFact", FilePath: "pkg/tools/client.go"},
		{ID: "fn:EmbeddedQuerier.StoreFact", Name: "EmbeddedQuerier.StoreFact", FilePath: "pkg/tools/embedded.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"pkg/tools/tools.go": "tools",
	}

	fields := []FieldEntity{} // No fields — standalone function
	implements := []ImplementsEdge{
		{TypeName: "CIEClient", InterfaceName: "Querier"},
		{TypeName: "EmbeddedQuerier", InterfaceName: "Querier"},
	}

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:storeFact",
			CalleeName: "client.StoreFact",
			FilePath:   "pkg/tools/tools.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 2 {
		t.Fatalf("expected 2 resolved calls via param dispatch, got %d", len(resolvedCalls))
	}

	calleeIDs := map[string]bool{}
	for _, call := range resolvedCalls {
		calleeIDs[call.CalleeID] = true
	}
	if !calleeIDs["fn:CIEClient.StoreFact"] {
		t.Error("expected callee fn:CIEClient.StoreFact")
	}
	if !calleeIDs["fn:EmbeddedQuerier.StoreFact"] {
		t.Error("expected callee fn:EmbeddedQuerier.StoreFact")
	}
}

func TestCallResolver_ResolveInterfaceCall_MethodFallbackToParams(t *testing.T) {
	// Method `Server.Run(q Querier)` calls q.Execute — "q" is a param, not a field.
	// Field-based lookup should fail (no field named "q"), then param-based should succeed.

	files := []FileEntity{
		{ID: "file:server.go", Path: "pkg/server.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{
			ID:        "fn:Server.Run",
			Name:      "Server.Run",
			FilePath:  "pkg/server.go",
			Signature: "func (s *Server) Run(q Querier) error",
		},
		{ID: "fn:LocalRunner.Execute", Name: "LocalRunner.Execute", FilePath: "pkg/runner.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"pkg/server.go": "pkg",
	}

	// Server has no field named "q"
	fields := []FieldEntity{
		{StructName: "Server", FieldName: "name", FieldType: "string"},
	}
	implements := []ImplementsEdge{
		{TypeName: "LocalRunner", InterfaceName: "Querier"},
	}

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Server.Run",
			CalleeName: "q.Execute",
			FilePath:   "pkg/server.go",
			Line:       15,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call via param fallback, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CalleeID != "fn:LocalRunner.Execute" {
		t.Errorf("expected callee fn:LocalRunner.Execute, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveConcreteFieldMethodCall(t *testing.T) {
	// Setup: EmbeddedBackend.Execute calls b.db.Run() where db is *CozoDB (concrete, not interface)
	// CozoDB.Run is in the index as a regular function.

	files := []FileEntity{
		{ID: "file:backend.go", Path: "pkg/storage/backend.go", Language: "go"},
		{ID: "file:cozodb.go", Path: "pkg/cozodb/cozodb.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:EmbeddedBackend.Execute", Name: "EmbeddedBackend.Execute", FilePath: "pkg/storage/backend.go"},
		{ID: "fn:CozoDB.Run", Name: "CozoDB.Run", FilePath: "pkg/cozodb/cozodb.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"pkg/storage/backend.go": "storage",
		"pkg/cozodb/cozodb.go":   "cozodb",
	}

	fields := []FieldEntity{
		{StructName: "EmbeddedBackend", FieldName: "db", FieldType: "CozoDB", FilePath: "pkg/storage/backend.go"},
	}
	implements := []ImplementsEdge{} // CozoDB is NOT an interface — no implements edges

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:EmbeddedBackend.Execute",
			CalleeName: "b.db.Run",
			FilePath:   "pkg/storage/backend.go",
			Line:       42,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call via concrete field dispatch, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CallerID != "fn:EmbeddedBackend.Execute" {
		t.Errorf("expected caller fn:EmbeddedBackend.Execute, got %s", resolvedCalls[0].CallerID)
	}
	if resolvedCalls[0].CalleeID != "fn:CozoDB.Run" {
		t.Errorf("expected callee fn:CozoDB.Run, got %s", resolvedCalls[0].CalleeID)
	}
}

func TestCallResolver_ResolveConcreteFieldMethodCall_ExternalStub(t *testing.T) {
	// Setup: A struct has a field of type DB (from external package, not indexed).
	// The resolver should generate a synthetic stub for DB.Query.

	files := []FileEntity{
		{ID: "file:repo.go", Path: "pkg/repo.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Repo.Save", Name: "Repo.Save", FilePath: "pkg/repo.go"},
		// Note: NO DB.Query function in the index (it's external)
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"pkg/repo.go": "pkg",
	}

	fields := []FieldEntity{
		{StructName: "Repo", FieldName: "db", FieldType: "DB", FilePath: "pkg/repo.go"},
	}
	implements := []ImplementsEdge{} // No implements for DB

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Repo.Save",
			CalleeName: "r.db.Query",
			FilePath:   "pkg/repo.go",
			Line:       15,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	// Should produce 1 edge to a synthetic stub
	if len(resolvedCalls) != 1 {
		t.Fatalf("expected 1 resolved call via external stub, got %d", len(resolvedCalls))
	}
	if resolvedCalls[0].CallerID != "fn:Repo.Save" {
		t.Errorf("expected caller fn:Repo.Save, got %s", resolvedCalls[0].CallerID)
	}

	// Verify stub was generated
	stubs := resolver.StubFunctions()
	if len(stubs) != 1 {
		t.Fatalf("expected 1 stub function, got %d", len(stubs))
	}
	if stubs[0].Name != "DB.Query" {
		t.Errorf("expected stub name DB.Query, got %s", stubs[0].Name)
	}
	if stubs[0].FilePath != "<external>" {
		t.Errorf("expected stub file_path <external>, got %s", stubs[0].FilePath)
	}
	// Stub ID should match the callee ID in the edge
	if resolvedCalls[0].CalleeID != stubs[0].ID {
		t.Errorf("edge calleeID %s should match stub ID %s", resolvedCalls[0].CalleeID, stubs[0].ID)
	}
}

func TestCallResolver_ResolveConcreteFieldMethodCall_InterfaceTakesPrecedence(t *testing.T) {
	// When a field type IS an interface, interface dispatch should work (no regression).
	// Writer is an interface with implementors CozoDB and FileStore.

	files := []FileEntity{
		{ID: "file:store.go", Path: "internal/store/store.go", Language: "go"},
	}
	functions := []FunctionEntity{
		{ID: "fn:Builder.Build", Name: "Builder.Build", FilePath: "internal/store/store.go"},
		{ID: "fn:CozoDB.Write", Name: "CozoDB.Write", FilePath: "internal/store/store.go"},
		{ID: "fn:FileStore.Write", Name: "FileStore.Write", FilePath: "internal/store/store.go"},
	}
	imports := []ImportEntity{}
	packageNames := map[string]string{
		"internal/store/store.go": "store",
	}

	fields := []FieldEntity{
		{StructName: "Builder", FieldName: "writer", FieldType: "Writer"},
	}
	implements := []ImplementsEdge{
		{TypeName: "CozoDB", InterfaceName: "Writer"},
		{TypeName: "FileStore", InterfaceName: "Writer"},
	}

	unresolvedCalls := []UnresolvedCall{
		{
			CallerID:   "fn:Builder.Build",
			CalleeName: "writer.Write",
			FilePath:   "internal/store/store.go",
			Line:       10,
		},
	}

	resolver := NewCallResolver()
	resolver.BuildIndex(files, functions, imports, packageNames)
	resolver.SetInterfaceIndex(fields, implements)

	resolvedCalls := resolver.ResolveCalls(unresolvedCalls)

	// Interface dispatch should produce 2 edges (CozoDB + FileStore), not 1
	if len(resolvedCalls) != 2 {
		t.Fatalf("expected 2 resolved calls (interface dispatch), got %d", len(resolvedCalls))
	}

	calleeIDs := map[string]bool{}
	for _, call := range resolvedCalls {
		calleeIDs[call.CalleeID] = true
	}
	if !calleeIDs["fn:CozoDB.Write"] {
		t.Error("expected callee fn:CozoDB.Write")
	}
	if !calleeIDs["fn:FileStore.Write"] {
		t.Error("expected callee fn:FileStore.Write")
	}

	// No stubs should be generated
	stubs := resolver.StubFunctions()
	if len(stubs) != 0 {
		t.Errorf("expected 0 stubs when interface dispatch succeeds, got %d", len(stubs))
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
