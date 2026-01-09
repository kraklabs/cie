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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kraklabs/cie/pkg/tools"
)

// ProjectMeta stores per-project indexing state in CozoDB.
// This replaces the 250MB manifest file with lightweight database state.
type ProjectMeta struct {
	ProjectID          string
	LastIndexedSHA     string
	LastCommittedIndex uint64
	UpdatedAt          time.Time
}

// GetProjectMeta retrieves the project metadata from CozoDB via Edge Cache.
// Returns nil (not an error) if the project has no metadata yet.
func GetProjectMeta(ctx context.Context, client tools.Querier, projectID string) (*ProjectMeta, error) {
	script := fmt.Sprintf(
		`?[last_indexed_sha, last_committed_index, updated_at] :=
		  *cie_project_meta { project_id, last_indexed_sha, last_committed_index, updated_at },
		  project_id = %q`,
		projectID,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		// Table might not exist yet - return nil, not error
		if strings.Contains(err.Error(), "Cannot find") {
			return nil, nil
		}
		return nil, fmt.Errorf("query project meta: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil // No metadata yet
	}

	row := result.Rows[0]
	if len(row) < 3 {
		return nil, fmt.Errorf("unexpected row format: %v", row)
	}

	meta := &ProjectMeta{
		ProjectID:      projectID,
		LastIndexedSHA: tools.AnyToString(row[0]),
	}

	// Parse last_committed_index
	switch v := row[1].(type) {
	case float64:
		meta.LastCommittedIndex = uint64(v)
	case int64:
		meta.LastCommittedIndex = uint64(v)
	case int:
		meta.LastCommittedIndex = uint64(v)
	case string:
		n, _ := strconv.ParseUint(v, 10, 64)
		meta.LastCommittedIndex = n
	}

	// Parse updated_at (stored as Unix timestamp)
	switch v := row[2].(type) {
	case float64:
		meta.UpdatedAt = time.Unix(int64(v), 0)
	case int64:
		meta.UpdatedAt = time.Unix(v, 0)
	case int:
		meta.UpdatedAt = time.Unix(int64(v), 0)
	}

	return meta, nil
}

// BuildSetProjectMetaScript builds the CozoScript to upsert project metadata.
// The script should be executed via Primary Hub's ExecuteWrite.
func BuildSetProjectMetaScript(meta *ProjectMeta) string {
	return fmt.Sprintf(
		`?[project_id, last_indexed_sha, last_committed_index, updated_at] <- [[%q, %q, %d, %d]]
:put cie_project_meta { project_id => last_indexed_sha, last_committed_index, updated_at }`,
		meta.ProjectID,
		meta.LastIndexedSHA,
		meta.LastCommittedIndex,
		meta.UpdatedAt.Unix(),
	)
}

// GetFunctionIDsForFiles retrieves function IDs for the given file paths.
// Returns a map of file_path -> []function_id.
func GetFunctionIDsForFiles(ctx context.Context, client tools.Querier, filePaths []string) (map[string][]string, error) {
	if len(filePaths) == 0 {
		return make(map[string][]string), nil
	}

	// Build OR condition for file paths
	// Use ends_with for efficiency (avoids full regex matching)
	conditions := make([]string, len(filePaths))
	for i, path := range filePaths {
		conditions[i] = fmt.Sprintf("file_path = %q", path)
	}

	script := fmt.Sprintf(
		`?[id, file_path] := *cie_function { id, file_path }, (%s)`,
		strings.Join(conditions, " or "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query function IDs: %w", err)
	}

	// Group by file_path
	byFile := make(map[string][]string)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		id := tools.AnyToString(row[0])
		filePath := tools.AnyToString(row[1])
		byFile[filePath] = append(byFile[filePath], id)
	}

	return byFile, nil
}

// GetFileIDsForPaths retrieves file IDs for the given file paths.
// Returns a map of file_path -> file_id.
func GetFileIDsForPaths(ctx context.Context, client tools.Querier, filePaths []string) (map[string]string, error) {
	if len(filePaths) == 0 {
		return make(map[string]string), nil
	}

	// Build OR condition for file paths
	conditions := make([]string, len(filePaths))
	for i, path := range filePaths {
		conditions[i] = fmt.Sprintf("path = %q", path)
	}

	script := fmt.Sprintf(
		`?[id, path] := *cie_file { id, path }, (%s)`,
		strings.Join(conditions, " or "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query file IDs: %w", err)
	}

	// Map path -> id
	byPath := make(map[string]string)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		id := tools.AnyToString(row[0])
		path := tools.AnyToString(row[1])
		byPath[path] = id
	}

	return byPath, nil
}

// StoredCallsEdge represents a caller->callee relationship stored in CozoDB.
// Different from CallsEdge in schema.go which lacks the ID field.
type StoredCallsEdge struct {
	ID       string
	CallerID string
	CalleeID string
}

// GetCallsEdgesForFiles retrieves calls edges where the caller is in one of the given files.
// Used for cleaning up stale edges when files are deleted or modified.
func GetCallsEdgesForFiles(ctx context.Context, client tools.Querier, filePaths []string) ([]StoredCallsEdge, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	// Build OR condition for file paths
	conditions := make([]string, len(filePaths))
	for i, path := range filePaths {
		conditions[i] = fmt.Sprintf("file_path = %q", path)
	}

	// Join cie_calls with cie_function to find edges where caller is in the given files
	script := fmt.Sprintf(
		`?[call_id, caller_id, callee_id] :=
		  *cie_calls { id: call_id, caller_id, callee_id },
		  *cie_function { id: caller_id, file_path },
		  (%s)`,
		strings.Join(conditions, " or "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query calls edges: %w", err)
	}

	edges := make([]StoredCallsEdge, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}
		edges = append(edges, StoredCallsEdge{
			ID:       tools.AnyToString(row[0]),
			CallerID: tools.AnyToString(row[1]),
			CalleeID: tools.AnyToString(row[2]),
		})
	}

	return edges, nil
}

// GetDefinesEdgesForFiles retrieves defines edges (file->function) for the given file paths.
func GetDefinesEdgesForFiles(ctx context.Context, client tools.Querier, filePaths []string) (map[string][]string, error) {
	if len(filePaths) == 0 {
		return make(map[string][]string), nil
	}

	// First get file IDs
	fileIDs, err := GetFileIDsForPaths(ctx, client, filePaths)
	if err != nil {
		return nil, err
	}

	if len(fileIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build OR condition for file IDs
	conditions := make([]string, 0, len(fileIDs))
	for _, fileID := range fileIDs {
		conditions = append(conditions, fmt.Sprintf("file_id = %q", fileID))
	}

	script := fmt.Sprintf(
		`?[id, file_id, function_id] := *cie_defines { id, file_id, function_id }, (%s)`,
		strings.Join(conditions, " or "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query defines edges: %w", err)
	}

	// Group by file_id and return defines IDs
	// We actually want to return the defines edge IDs for deletion
	byFileID := make(map[string][]string)
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}
		definesID := tools.AnyToString(row[0])
		fileID := tools.AnyToString(row[1])
		byFileID[fileID] = append(byFileID[fileID], definesID)
	}

	return byFileID, nil
}

// GetTypeIDsForFiles retrieves type IDs for the given file paths.
// Returns a map of file_path -> []type_id.
func GetTypeIDsForFiles(ctx context.Context, client tools.Querier, filePaths []string) (map[string][]string, error) {
	if len(filePaths) == 0 {
		return make(map[string][]string), nil
	}

	// Build OR condition for file paths
	conditions := make([]string, len(filePaths))
	for i, path := range filePaths {
		conditions[i] = fmt.Sprintf("file_path = %q", path)
	}

	script := fmt.Sprintf(
		`?[id, file_path] := *cie_type { id, file_path }, (%s)`,
		strings.Join(conditions, " or "),
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query type IDs: %w", err)
	}

	// Group by file_path
	byFile := make(map[string][]string)
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}
		id := tools.AnyToString(row[0])
		filePath := tools.AnyToString(row[1])
		byFile[filePath] = append(byFile[filePath], id)
	}

	return byFile, nil
}
