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

package tools

import (
	"context"
	"fmt"
	"testing"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
)

// ============================================================================
// BENCHMARKS for Core CIE Operations
// ============================================================================

// setupBenchmarkClient creates a test client with a realistic dataset for benchmarking.
// The database is populated with 100 files, 500 functions, and 1000 call relationships.
func setupBenchmarkClient(b *testing.B) (*TestCIEClient, context.Context) {
	b.Helper()

	// Create in-memory test database
	db := openTestDB(b)

	// Populate with realistic test data
	populateBenchmarkData(b, db)

	client := NewTestCIEClient(db)
	ctx := context.Background()

	return client, ctx
}

// populateBenchmarkData seeds the database with a realistic dataset representing
// a medium-sized codebase with ~500 functions across 100 files.
func populateBenchmarkData(b *testing.B, db *cozo.CozoDB) {
	b.Helper()

	// Create 100 files across various packages
	packages := []string{"cmd", "internal/auth", "internal/api", "internal/db", "pkg/models", "pkg/utils"}
	filesPerPackage := 100 / len(packages)

	fileID := 0
	for _, pkg := range packages {
		for i := 0; i < filesPerPackage; i++ {
			fileID++
			path := fmt.Sprintf("%s/file%d.go", pkg, i)
			insertTestFile(b, db, fmt.Sprintf("file_%d", fileID), path, "go")
		}
	}

	// Create 500 functions with realistic signatures and code
	functionID := 0
	functionNames := []string{
		"HandleAuth", "HandleLogin", "HandleLogout", "ValidateToken", "RefreshToken",
		"GetUser", "CreateUser", "UpdateUser", "DeleteUser", "ListUsers",
		"GetPost", "CreatePost", "UpdatePost", "DeletePost", "ListPosts",
		"Connect", "Query", "Exec", "Begin", "Commit", "Rollback",
		"Encode", "Decode", "Validate", "Transform", "Process",
		"main", "init", "setup", "teardown", "helper",
	}

	for fileIdx := 1; fileIdx <= 100; fileIdx++ {
		// 5 functions per file = 500 total
		for funcIdx := 0; funcIdx < 5; funcIdx++ {
			functionID++
			name := functionNames[(functionID-1)%len(functionNames)]
			if funcIdx > 0 {
				name = fmt.Sprintf("%s%d", name, funcIdx)
			}

			signature := fmt.Sprintf("func %s(ctx context.Context, param string) error", name)
			code := fmt.Sprintf(`func %s(ctx context.Context, param string) error {
	if param == "" {
		return errors.New("param required")
	}
	result, err := db.Query(ctx, param)
	if err != nil {
		return fmt.Errorf("query failed: %%w", err)
	}
	return processResult(result)
}`, name)

			insertTestFunction(
				b,
				db,
				fmt.Sprintf("func_%d", functionID),
				name,
				fmt.Sprintf("pkg/file%d.go", fileIdx),
				signature,
				code,
				10+funcIdx*15,
			)
		}
	}

	// Create 1000 call relationships forming a realistic call graph
	// Pattern: main → handlers → database → utils
	callID := 0
	for callerID := 1; callerID <= 500; callerID++ {
		// Each function calls 2-3 other functions on average
		numCallees := 2
		if callerID%10 == 0 {
			numCallees = 3
		}

		for i := 0; i < numCallees; i++ {
			callID++
			if callID > 1000 {
				break
			}

			// Call functions deeper in the stack (higher IDs)
			calleeID := callerID + (i+1)*10
			if calleeID > 500 {
				calleeID = calleeID % 500
			}

			insertTestCall(
				b,
				db,
				fmt.Sprintf("call_%d", callID),
				fmt.Sprintf("func_%d", callerID),
				fmt.Sprintf("func_%d", calleeID),
			)
		}
	}
}

// BenchmarkSemanticSearch measures semantic search performance with a small result set.
// This benchmark measures vector similarity search performance.
// Note: Actual embedding generation is not performed in tests, so this measures
// the keyword search fallback path.
func BenchmarkSemanticSearch(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SemanticSearch(ctx, client, SemanticSearchArgs{
			Query: "authentication handler",
			Limit: 10,
		})
	}
}

// BenchmarkSemanticSearch_Large measures semantic search with a larger result set.
// Tests performance when returning 100 results instead of 10.
func BenchmarkSemanticSearch_Large(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SemanticSearch(ctx, client, SemanticSearchArgs{
			Query: "function",
			Limit: 100,
		})
	}
}

// BenchmarkGrep measures literal text search performance for a single pattern.
// This is one of the fastest operations as it uses indexed text search.
func BenchmarkGrep(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Grep(ctx, client, GrepArgs{
			Text: "Query",
		})
	}
}

// BenchmarkGrepMulti measures batch text search performance for multiple patterns.
// Tests the efficiency of searching for 3 patterns simultaneously.
func BenchmarkGrepMulti(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Grep(ctx, client, GrepArgs{
			Texts: []string{"HandleAuth", "Query", "error"},
		})
	}
}

// BenchmarkFindFunction measures function lookup performance by name.
// This tests the index performance for function name queries.
func BenchmarkFindFunction(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindFunction(ctx, client, FindFunctionArgs{
			Name: "HandleAuth",
		})
	}
}

// BenchmarkFindCallers measures caller discovery performance.
// This tests call graph traversal in the reverse direction.
func BenchmarkFindCallers(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindCallers(ctx, client, FindCallersArgs{
			FunctionName: "Query",
		})
	}
}

// BenchmarkGetCallGraph measures full call graph retrieval performance.
// This tests the performance of getting both callers and callees for a function.
func BenchmarkGetCallGraph(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetCallGraph(ctx, client, GetCallGraphArgs{
			FunctionName: "HandleAuth",
		})
	}
}

// BenchmarkTracePath measures path tracing performance from entry points to a target.
// This is one of the most complex operations, performing BFS traversal of the call graph.
func BenchmarkTracePath(b *testing.B) {
	client, ctx := setupBenchmarkClient(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = TracePath(ctx, client, TracePathArgs{
			Target:   "Query",
			MaxDepth: 10,
		})
	}
}
