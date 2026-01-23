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
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// CallResolver resolves cross-package function calls.
// It builds an index of all functions and imports, then resolves
// unresolved calls from the parsing phase.
type CallResolver struct {
	mu sync.RWMutex

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
	if strings.Contains(call.CalleeName, ".") {
		if id := r.resolveQualifiedCall(call); id != "" {
			return id
		}
	}
	return r.resolveDotImportCall(call)
}

// resolveQualifiedCall resolves calls like "pkg.Foo()" or "obj.Method()".
func (r *CallResolver) resolveQualifiedCall(call UnresolvedCall) string {
	parts := strings.SplitN(call.CalleeName, ".", 2)
	funcName := extractLastComponent(call.CalleeName, parts[1])

	if !isExportedName(funcName) {
		return ""
	}

	imports, ok := r.fileImports[call.FilePath]
	if !ok {
		return ""
	}
	importPath, ok := imports[parts[0]]
	if !ok {
		return ""
	}
	return r.lookupFunctionInPackage(importPath, funcName)
}

// resolveDotImportCall resolves calls from dot imports.
func (r *CallResolver) resolveDotImportCall(call UnresolvedCall) string {
	imports, ok := r.fileImports[call.FilePath]
	if !ok {
		return ""
	}
	for alias, importPath := range imports {
		if alias == "." {
			if id := r.lookupFunctionInPackage(importPath, call.CalleeName); id != "" {
				return id
			}
		}
	}
	return ""
}

// extractLastComponent extracts the final function name from a chain like "obj.method.Func".
func extractLastComponent(fullName, funcName string) string {
	if strings.Contains(funcName, ".") {
		lastDot := strings.LastIndex(fullName, ".")
		return fullName[lastDot+1:]
	}
	return funcName
}

// isExportedName checks if a function name is exported (starts with uppercase).
func isExportedName(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

// lookupFunctionInPackage finds a function by import path and function name.
func (r *CallResolver) lookupFunctionInPackage(importPath, funcName string) string {
	pkgPath := r.findPackageByImportPath(importPath)
	if pkgPath == "" {
		return ""
	}
	if funcs, ok := r.globalFunctions[pkgPath]; ok {
		if funcID, ok := funcs[funcName]; ok {
			return funcID
		}
	}
	return ""
}

// findPackageByImportPath finds our internal package path from an import path.
func (r *CallResolver) findPackageByImportPath(importPath string) string {
	r.mu.RLock()
	// Direct match
	if pkgPath, exists := r.importPathToPackagePath[importPath]; exists {
		r.mu.RUnlock()
		return pkgPath
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check after acquiring lock
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
