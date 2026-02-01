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
	"context"
	"fmt"
	"testing"
)

// MockGitRunner is a mock implementation of GitRunner for testing.
type MockGitRunner struct {
	RunFunc  func(ctx context.Context, args ...string) (string, error)
	repoPath string
}

func (m *MockGitRunner) Run(ctx context.Context, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, args...)
	}
	return "", nil
}

func (m *MockGitRunner) RepoPath() string {
	return m.repoPath
}

// newMockGitRunner creates a mock git runner for testing.
func newMockGitRunner(repoPath string) *MockGitRunner {
	return &MockGitRunner{repoPath: repoPath}
}

// --- FunctionHistory Tests ---

func TestFunctionHistory_Success(t *testing.T) {
	t.Parallel()

	// Mock CIE client that returns a single function
	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows: [][]any{
					{"HandleAuth", "internal/auth/handler.go", float64(42), float64(65)},
				},
			}, nil
		},
	}

	// Mock git runner that returns commit history
	mockGit := newMockGitRunner("/repo")
	mockGit.RunFunc = func(ctx context.Context, args ...string) (string, error) {
		return `abc1234|2024-01-15|John Doe|Fix edge case in auth
def5678|2024-01-10|Jane Smith|Add auth handler
ghi9012|2024-01-05|John Doe|Initial implementation`, nil
	}

	result, err := FunctionHistory(context.Background(), mockClient, mockGit, FunctionHistoryArgs{
		FunctionName: "HandleAuth",
		Limit:        10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}

	// Check output contains expected elements
	if !ContainsStr(result.Text, "HandleAuth") {
		t.Errorf("expected function name in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "abc1234") {
		t.Errorf("expected commit hash in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "John Doe") {
		t.Errorf("expected author in output, got: %s", result.Text)
	}
}

func TestFunctionHistory_FunctionNotFound(t *testing.T) {
	t.Parallel()

	// Mock CIE client that returns no results
	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows:    [][]any{},
			}, nil
		},
	}

	mockGit := newMockGitRunner("/repo")

	result, err := FunctionHistory(context.Background(), mockClient, mockGit, FunctionHistoryArgs{
		FunctionName: "NonExistent",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected non-error result with guidance")
	}

	if !ContainsStr(result.Text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", result.Text)
	}
}

func TestFunctionHistory_MultipleFunctions(t *testing.T) {
	t.Parallel()

	// Mock CIE client that returns multiple functions (ambiguous)
	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows: [][]any{
					{"HandleAuth", "internal/auth/v1/handler.go", float64(42), float64(65)},
					{"HandleAuth", "internal/auth/v2/handler.go", float64(30), float64(50)},
				},
			}, nil
		},
	}

	mockGit := newMockGitRunner("/repo")

	result, err := FunctionHistory(context.Background(), mockClient, mockGit, FunctionHistoryArgs{
		FunctionName: "HandleAuth",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !ContainsStr(result.Text, "Multiple functions match") {
		t.Errorf("expected disambiguation message, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "path_pattern") {
		t.Errorf("expected path_pattern suggestion, got: %s", result.Text)
	}
}

func TestFunctionHistory_RequiresFunctionName(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{}
	mockGit := newMockGitRunner("/repo")

	result, err := FunctionHistory(context.Background(), mockClient, mockGit, FunctionHistoryArgs{
		FunctionName: "",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Errorf("expected error result for empty function name")
	}

	if !ContainsStr(result.Text, "required") {
		t.Errorf("expected 'required' in error message, got: %s", result.Text)
	}
}

// --- FindIntroduction Tests ---

func TestFindIntroduction_PatternFound(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{}
	mockGit := newMockGitRunner("/repo")
	mockGit.RunFunc = func(ctx context.Context, args ...string) (string, error) {
		// Check this is the right command
		if len(args) >= 2 && args[0] == "log" && args[1] == "-S" {
			return `abc123456789|2024-01-15|John Doe|Add JWT authentication`, nil
		}
		if args[0] == "show" {
			return ` internal/auth/jwt.go | 15 +++++++++++++++`, nil
		}
		return "", nil
	}

	result, err := FindIntroduction(context.Background(), mockClient, mockGit, FindIntroductionArgs{
		CodeSnippet: "jwt.Generate()",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}

	if !ContainsStr(result.Text, "jwt.Generate()") {
		t.Errorf("expected pattern in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "abc1234") {
		t.Errorf("expected commit hash in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "John Doe") {
		t.Errorf("expected author in output, got: %s", result.Text)
	}
}

func TestFindIntroduction_PatternNotFound(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{}
	mockGit := newMockGitRunner("/repo")
	mockGit.RunFunc = func(ctx context.Context, args ...string) (string, error) {
		return "", nil // Empty output = not found
	}

	result, err := FindIntroduction(context.Background(), mockClient, mockGit, FindIntroductionArgs{
		CodeSnippet: "nonExistentPattern()",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected non-error result with guidance")
	}

	if !ContainsStr(result.Text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", result.Text)
	}
}

func TestFindIntroduction_RequiresCodeSnippet(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{}
	mockGit := newMockGitRunner("/repo")

	result, err := FindIntroduction(context.Background(), mockClient, mockGit, FindIntroductionArgs{
		CodeSnippet: "",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Errorf("expected error result for empty code snippet")
	}
}

// --- BlameFunction Tests ---

func TestBlameFunction_Success(t *testing.T) {
	t.Parallel()

	// Mock CIE client that returns a function
	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows: [][]any{
					{"RegisterRoutes", "internal/routes/setup.go", float64(15), float64(45)},
				},
			}, nil
		},
	}

	// Mock git blame output in porcelain format
	mockGit := newMockGitRunner("/repo")
	mockGit.RunFunc = func(ctx context.Context, args ...string) (string, error) {
		return `abc1234567890abcdef1234567890abcdef123456 15 15 10
author John Doe
author-mail <john@example.com>
author-time 1705320000
author-tz -0800
committer John Doe
committer-mail <john@example.com>
committer-time 1705320000
committer-tz -0800
summary Fix routing
filename internal/routes/setup.go
	router.GET("/health", healthHandler)
def567890abcdef1234567890abcdef12345678 16 16 5
author Jane Smith
author-mail <jane@example.com>
author-time 1705230000
author-tz -0800
committer Jane Smith
committer-mail <jane@example.com>
committer-time 1705230000
committer-tz -0800
summary Add auth routes
filename internal/routes/setup.go
	router.POST("/auth", authHandler)`, nil
	}

	result, err := BlameFunction(context.Background(), mockClient, mockGit, BlameFunctionArgs{
		FunctionName: "RegisterRoutes",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}

	// Check output contains expected elements
	if !ContainsStr(result.Text, "RegisterRoutes") {
		t.Errorf("expected function name in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "John Doe") {
		t.Errorf("expected author in output, got: %s", result.Text)
	}
	if !ContainsStr(result.Text, "Jane Smith") {
		t.Errorf("expected second author in output, got: %s", result.Text)
	}
}

func TestBlameFunction_FunctionNotFound(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows:    [][]any{},
			}, nil
		},
	}

	mockGit := newMockGitRunner("/repo")

	result, err := BlameFunction(context.Background(), mockClient, mockGit, BlameFunctionArgs{
		FunctionName: "NonExistent",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("expected non-error result with guidance")
	}

	if !ContainsStr(result.Text, "not found") {
		t.Errorf("expected 'not found' message, got: %s", result.Text)
	}
}

func TestBlameFunction_RequiresFunctionName(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{}
	mockGit := newMockGitRunner("/repo")

	result, err := BlameFunction(context.Background(), mockClient, mockGit, BlameFunctionArgs{
		FunctionName: "",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Errorf("expected error result for empty function name")
	}
}

// --- GitExecutor Tests ---

func TestGitExecutor_RepoDiscovery(t *testing.T) {
	t.Parallel()

	// Test with current directory (should work since we're in a git repo)
	exec, err := NewGitExecutor(".")
	if err != nil {
		t.Fatalf("expected to find git repo: %v", err)
	}

	if exec.RepoPath() == "" {
		t.Errorf("expected non-empty repo path")
	}
}

func TestGitExecutor_InvalidPath(t *testing.T) {
	t.Parallel()

	_, err := NewGitExecutor("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Errorf("expected error for nonexistent path")
	}
}

func TestGitExecutor_EmptyPath(t *testing.T) {
	t.Parallel()

	_, err := NewGitExecutor("")
	if err == nil {
		t.Errorf("expected error for empty path")
	}
}

// --- Helper Function Tests ---

func TestParseBlameOutput(t *testing.T) {
	t.Parallel()

	output := `abc1234567890abcdef1234567890abcdef123456 1 1 1
author John Doe
author-mail <john@example.com>
	line content here
def567890abcdef1234567890abcdef12345678 2 2 1
author Jane Smith
author-mail <jane@example.com>
	another line
abc1234567890abcdef1234567890abcdef123456 3 3 1
author John Doe
author-mail <john@example.com>
	third line`

	authors := parseBlameOutput(output)

	if len(authors) != 2 {
		t.Errorf("expected 2 authors, got %d", len(authors))
	}

	johnDoe := authors["John Doe"]
	if johnDoe == nil {
		t.Fatalf("expected John Doe in authors")
	}
	if johnDoe.Lines != 2 {
		t.Errorf("expected John Doe to have 2 lines, got %d", johnDoe.Lines)
	}

	janeSmith := authors["Jane Smith"]
	if janeSmith == nil {
		t.Fatalf("expected Jane Smith in authors")
	}
	if janeSmith.Lines != 1 {
		t.Errorf("expected Jane Smith to have 1 line, got %d", janeSmith.Lines)
	}
}

func TestIsHexString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"0123456789abcdef", true},
		{"ghijkl", false},
		{"abc xyz", false},
		{"", true}, // Empty string is technically valid
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isHexString(tt.input)
			if result != tt.expected {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatAmbiguousFunctions(t *testing.T) {
	t.Parallel()

	locations := []FunctionLocation{
		{Name: "HandleAuth", FilePath: "internal/v1/auth.go", StartLine: 10, EndLine: 30},
		{Name: "HandleAuth", FilePath: "internal/v2/auth.go", StartLine: 15, EndLine: 40},
	}

	output := formatAmbiguousFunctions(locations)

	if !ContainsStr(output, "Multiple functions match") {
		t.Errorf("expected disambiguation header, got: %s", output)
	}
	if !ContainsStr(output, "internal/v1/auth.go") {
		t.Errorf("expected first path, got: %s", output)
	}
	if !ContainsStr(output, "internal/v2/auth.go") {
		t.Errorf("expected second path, got: %s", output)
	}
	if !ContainsStr(output, "path_pattern") {
		t.Errorf("expected path_pattern suggestion, got: %s", output)
	}
}

// --- Integration-style Tests (with real git) ---

func TestFunctionHistory_WithRealGit(t *testing.T) {
	t.Parallel()

	// Skip if not in a git repo
	exec, err := NewGitExecutor(".")
	if err != nil {
		t.Skip("not in a git repository")
	}

	// Use a real function from this codebase
	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			// Simulate finding the FunctionHistory function itself
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows: [][]any{
					{"FunctionHistory", "pkg/tools/git_history.go", float64(37), float64(85)},
				},
			}, nil
		},
	}

	result, err := FunctionHistory(context.Background(), mockClient, exec, FunctionHistoryArgs{
		FunctionName: "FunctionHistory",
		Limit:        5,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just verify it doesn't error - actual content depends on git history
	if result.Text == "" {
		t.Errorf("expected non-empty result")
	}
}

// --- Edge Cases ---

func TestFunctionHistory_GitLogFails_FallsBackToFileHistory(t *testing.T) {
	t.Parallel()

	mockClient := &MockCIEClient{
		QueryFunc: func(ctx context.Context, script string) (*QueryResult, error) {
			return &QueryResult{
				Headers: []string{"name", "file_path", "start_line", "end_line"},
				Rows: [][]any{
					{"HandleAuth", "internal/auth/handler.go", float64(42), float64(65)},
				},
			}, nil
		},
	}

	callCount := 0
	mockGit := newMockGitRunner("/repo")
	mockGit.RunFunc = func(ctx context.Context, args ...string) (string, error) {
		callCount++
		// First call (git log -L) fails
		if callCount == 1 {
			return "", fmt.Errorf("fatal: no such path")
		}
		// Second call (file history fallback) succeeds
		return `abc1234|2024-01-15|John Doe|Fix handler`, nil
	}

	result, err := FunctionHistory(context.Background(), mockClient, mockGit, FunctionHistoryArgs{
		FunctionName: "HandleAuth",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Text)
	}

	// Should contain fallback note
	if !ContainsStr(result.Text, "file-level history") {
		t.Errorf("expected fallback note in output, got: %s", result.Text)
	}
}
