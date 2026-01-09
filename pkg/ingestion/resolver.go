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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// CallResolver resolves cross-package function calls.
// It builds an index of all functions and imports, then resolves
// unresolved calls from the parsing phase.
type CallResolver struct {
	// packageIndex: directory path → PackageInfo
	packageIndex map[string]*PackageInfo

	// globalFunctions: package_path → function_name → function_id
	// Stores exported functions (capitalized) from each package
	globalFunctions map[string]map[string]string

	// fileImports: file_path → alias → import_path
	// Maps what each file has imported
	fileImports map[string]map[string]string

	// importPathToPackagePath: import_path → local package path
	// Maps Go import paths to local directory paths
	importPathToPackagePath map[string]string
}

// NewCallResolver creates a new call resolver.
func NewCallResolver() *CallResolver {
	return &CallResolver{
		packageIndex:            make(map[string]*PackageInfo),
		globalFunctions:         make(map[string]map[string]string),
		fileImports:             make(map[string]map[string]string),
		importPathToPackagePath: make(map[string]string),
	}
}

// BuildIndex constructs the global function registry from parsed results.
// This should be called after all files have been parsed.
func (r *CallResolver) BuildIndex(
	files []FileEntity,
	functions []FunctionEntity,
	imports []ImportEntity,
	packageNames map[string]string, // file_path → package_name
) {
	// 1. Build package index from file paths
	for _, f := range files {
		if f.Language != "go" {
			continue
		}
		pkgPath := filepath.Dir(f.Path)
		pkgName := packageNames[f.Path]

		if _, exists := r.packageIndex[pkgPath]; !exists {
			r.packageIndex[pkgPath] = &PackageInfo{
				PackagePath: pkgPath,
				PackageName: pkgName,
				Files:       []string{},
			}
		}
		r.packageIndex[pkgPath].Files = append(r.packageIndex[pkgPath].Files, f.Path)
	}

	// 2. Build global function registry
	for _, fn := range functions {
		if !strings.HasSuffix(fn.FilePath, ".go") {
			continue
		}

		pkgPath := filepath.Dir(fn.FilePath)
		if _, exists := r.globalFunctions[pkgPath]; !exists {
			r.globalFunctions[pkgPath] = make(map[string]string)
		}

		// Store by simple name (without receiver prefix)
		simpleName := extractSimpleName(fn.Name)

		// Only store exported functions (starts with uppercase)
		// Also store all functions for same-package resolution
		r.globalFunctions[pkgPath][simpleName] = fn.ID
	}

	// 3. Build file imports index
	for _, imp := range imports {
		if _, exists := r.fileImports[imp.FilePath]; !exists {
			r.fileImports[imp.FilePath] = make(map[string]string)
		}

		// Determine the alias used for this import
		alias := imp.Alias
		if alias == "" || alias == "_" {
			// Default alias is the last path component
			alias = filepath.Base(imp.ImportPath)
		}

		// Skip blank imports
		if alias == "_" {
			continue
		}

		r.fileImports[imp.FilePath][alias] = imp.ImportPath
	}

	// 4. Build import path to package path mapping
	// This maps import paths to our local package directories
	r.buildImportPathMapping()
}

// buildImportPathMapping creates a mapping from Go import paths to local package paths.
func (r *CallResolver) buildImportPathMapping() {
	// For each package we have, try to infer the import path
	// This works for relative paths within the same module
	for pkgPath, pkgInfo := range r.packageIndex {
		// The import path suffix should match the package path
		// e.g., if pkgPath is "internal/http/handlers", the import would end with that
		r.importPathToPackagePath[pkgPath] = pkgPath

		// Also try to match by package name as a fallback
		if pkgInfo.PackageName != "" {
			// For local packages, the package name often matches the directory name
			r.importPathToPackagePath[pkgInfo.PackageName] = pkgPath
		}
	}
}

// ResolveCalls resolves unresolved calls to their target functions.
// Returns the resolved call edges.
// Uses parallel processing for large call sets (>1000 calls).
func (r *CallResolver) ResolveCalls(unresolvedCalls []UnresolvedCall) []CallsEdge {
	// For small sets, use sequential processing (avoid goroutine overhead)
	if len(unresolvedCalls) < 1000 {
		return r.resolveCallsSequential(unresolvedCalls)
	}
	return r.resolveCallsParallel(unresolvedCalls)
}

// resolveCallsSequential processes calls sequentially (for small sets).
func (r *CallResolver) resolveCallsSequential(unresolvedCalls []UnresolvedCall) []CallsEdge {
	var resolved []CallsEdge
	seen := make(map[string]bool)

	for _, call := range unresolvedCalls {
		calleeID := r.resolveCall(call)
		if calleeID != "" {
			edgeKey := call.CallerID + "->" + calleeID
			if !seen[edgeKey] {
				seen[edgeKey] = true
				resolved = append(resolved, CallsEdge{
					CallerID: call.CallerID,
					CalleeID: calleeID,
				})
			}
		}
	}

	return resolved
}

// resolveCallsParallel processes calls in parallel using worker pool.
// The indices are read-only after BuildIndex, so concurrent access is safe.
func (r *CallResolver) resolveCallsParallel(unresolvedCalls []UnresolvedCall) []CallsEdge {
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Cap at 8 workers
	}

	// Channel for jobs (indices into unresolvedCalls)
	jobs := make(chan int, len(unresolvedCalls))

	// Channel for results
	type resolveResult struct {
		callerID string
		calleeID string
	}
	results := make(chan resolveResult, len(unresolvedCalls))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				call := unresolvedCalls[i]
				calleeID := r.resolveCall(call)
				if calleeID != "" {
					results <- resolveResult{
						callerID: call.CallerID,
						calleeID: calleeID,
					}
				}
			}
		}()
	}

	// Send jobs
	for i := range unresolvedCalls {
		jobs <- i
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and deduplicate results
	seen := make(map[string]bool)
	var resolved []CallsEdge
	for result := range results {
		edgeKey := result.callerID + "->" + result.calleeID
		if !seen[edgeKey] {
			seen[edgeKey] = true
			resolved = append(resolved, CallsEdge{
				CallerID: result.callerID,
				CalleeID: result.calleeID,
			})
		}
	}

	return resolved
}

// resolveCall attempts to resolve a single unresolved call.
func (r *CallResolver) resolveCall(call UnresolvedCall) string {
	// Case 1: Qualified call like "pkg.Foo()" or "response.RespondError()"
	if strings.Contains(call.CalleeName, ".") {
		parts := strings.SplitN(call.CalleeName, ".", 2)
		pkgAlias := parts[0]
		funcName := parts[1]

		// Handle method calls on objects (e.g., "s.handler.Run()")
		// We only care about the last component
		if strings.Contains(funcName, ".") {
			lastDot := strings.LastIndex(call.CalleeName, ".")
			funcName = call.CalleeName[lastDot+1:]
		}

		// Skip if the function name doesn't start with uppercase (not exported)
		if len(funcName) == 0 || funcName[0] < 'A' || funcName[0] > 'Z' {
			return ""
		}

		// Look up the import path for this alias
		imports, ok := r.fileImports[call.FilePath]
		if !ok {
			return ""
		}

		importPath, ok := imports[pkgAlias]
		if !ok {
			return ""
		}

		// Find matching package in our index
		pkgPath := r.findPackageByImportPath(importPath)
		if pkgPath == "" {
			return ""
		}

		// Look up the function in that package
		if funcs, ok := r.globalFunctions[pkgPath]; ok {
			if funcID, ok := funcs[funcName]; ok {
				return funcID
			}
		}
	}

	// Case 2: Dot import (function called without package prefix)
	// Check if any dot imports contain this function
	imports, ok := r.fileImports[call.FilePath]
	if ok {
		for alias, importPath := range imports {
			if alias == "." {
				pkgPath := r.findPackageByImportPath(importPath)
				if pkgPath == "" {
					continue
				}
				if funcs, ok := r.globalFunctions[pkgPath]; ok {
					if funcID, ok := funcs[call.CalleeName]; ok {
						return funcID
					}
				}
			}
		}
	}

	return ""
}

// findPackageByImportPath finds our internal package path from an import path.
func (r *CallResolver) findPackageByImportPath(importPath string) string {
	// Direct match
	if pkgPath, exists := r.importPathToPackagePath[importPath]; exists {
		return pkgPath
	}

	// Try suffix matching: "github.com/org/project/internal/handlers" -> "internal/handlers"
	for pkgPath := range r.packageIndex {
		if strings.HasSuffix(importPath, pkgPath) {
			r.importPathToPackagePath[importPath] = pkgPath // Cache for future lookups
			return pkgPath
		}
	}

	// Try matching just the last component
	baseName := filepath.Base(importPath)
	for pkgPath, pkgInfo := range r.packageIndex {
		if pkgInfo.PackageName == baseName {
			r.importPathToPackagePath[importPath] = pkgPath // Cache for future lookups
			return pkgPath
		}
	}

	return ""
}

// Stats returns statistics about the resolver's index.
func (r *CallResolver) Stats() (packages, functions, imports int) {
	packages = len(r.packageIndex)

	for _, funcs := range r.globalFunctions {
		functions += len(funcs)
	}

	for _, imps := range r.fileImports {
		imports += len(imps)
	}

	return
}
