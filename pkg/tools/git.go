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

package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRunner is the interface for executing git commands.
// This allows mocking in tests.
type GitRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
	RepoPath() string
}

// GitExecutor handles git command execution with proper error handling.
type GitExecutor struct {
	repoPath string // Absolute path to git repo root
}

// NewGitExecutor creates a GitExecutor by discovering the repo root from startPath.
// Returns an error if startPath is not inside a git repository.
func NewGitExecutor(startPath string) (*GitExecutor, error) {
	if startPath == "" {
		return nil, fmt.Errorf("startPath cannot be empty")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path: %w", err)
	}

	// Run git rev-parse to find repo root
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("not a git repository: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git not found or not installed: %w", err)
	}

	repoPath := strings.TrimSpace(string(output))
	if repoPath == "" {
		return nil, fmt.Errorf("could not determine git repository root")
	}

	return &GitExecutor{repoPath: repoPath}, nil
}

// RepoPath returns the absolute path to the git repository root.
func (g *GitExecutor) RepoPath() string {
	return g.repoPath
}

// Run executes a git command with the given arguments and returns the output.
// The command is run in the repository root directory.
// Context is used for timeout/cancellation support.
func (g *GitExecutor) Run(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no git command specified")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if context was canceled
		if ctx.Err() != nil {
			return "", fmt.Errorf("git command timed out or canceled: %w", ctx.Err())
		}

		// Return stderr for better error messages
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("git %s failed: %s", args[0], stderrStr)
		}
		return "", fmt.Errorf("git %s failed: %w", args[0], err)
	}

	return stdout.String(), nil
}

// FunctionLocation represents a function's location in the codebase.
type FunctionLocation struct {
	FilePath  string
	StartLine int
	EndLine   int
	Name      string
}

// FindFunctionsWithLocation queries the CIE index to locate functions by name.
// Returns multiple matches if the name is ambiguous.
// Unlike findFunctionsByName in trace.go, this also returns end_line for git history operations.
func FindFunctionsWithLocation(ctx context.Context, client Querier, name, pathPattern string) ([]FunctionLocation, error) {
	// Build condition for function name matching
	condition := fmt.Sprintf("(name = %q or ends_with(name, %q))", name, "."+name)

	// Add path filter if specified
	if pathPattern != "" {
		condition += fmt.Sprintf(" and regex_matches(file_path, %s)", QuoteCozoPattern(EscapeRegex(pathPattern)))
	}

	script := fmt.Sprintf(
		"?[name, file_path, start_line, end_line] := *cie_function { name, file_path, start_line, end_line }, %s :limit 10",
		condition,
	)

	result, err := client.Query(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("query functions: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	var locations []FunctionLocation
	for _, row := range result.Rows {
		loc := FunctionLocation{
			Name:     AnyToString(row[0]),
			FilePath: AnyToString(row[1]),
		}

		// Parse line numbers
		if startLine, ok := row[2].(float64); ok {
			loc.StartLine = int(startLine)
		}
		if endLine, ok := row[3].(float64); ok {
			loc.EndLine = int(endLine)
		}

		locations = append(locations, loc)
	}

	return locations, nil
}

// formatAmbiguousFunctions formats a list of functions for disambiguation prompts.
func formatAmbiguousFunctions(locations []FunctionLocation) string {
	var sb strings.Builder
	sb.WriteString("Multiple functions match. Please specify using `path_pattern`:\n\n")
	for i, loc := range locations {
		sb.WriteString(fmt.Sprintf("%d. **%s** in `%s:%d-%d`\n", i+1, loc.Name, loc.FilePath, loc.StartLine, loc.EndLine))
	}
	sb.WriteString("\nExample: `path_pattern=\"")
	if len(locations) > 0 {
		// Extract directory from first match as example
		dir := ExtractDir(locations[0].FilePath)
		sb.WriteString(dir)
	}
	sb.WriteString("\"`")
	return sb.String()
}
