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
	"strings"
)

// FunctionHistoryArgs holds arguments for the cie_function_history tool.
type FunctionHistoryArgs struct {
	FunctionName string // Required: name of the function to get history for
	Limit        int    // Maximum commits to show (default: 10)
	Since        string // Only show commits after this date (e.g., "2024-01-01")
	PathPattern  string // Optional: disambiguate multiple matches
}

// FunctionHistory retrieves git commit history for a specific function.
// It uses git log -L to track line-based history.
func FunctionHistory(ctx context.Context, client Querier, git GitRunner, args FunctionHistoryArgs) (*ToolResult, error) {
	if args.FunctionName == "" {
		return NewError("Error: 'function_name' is required"), nil
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}

	// Find function location using CIE index
	locations, err := FindFunctionsWithLocation(ctx, client, args.FunctionName, args.PathPattern)
	if err != nil {
		return nil, fmt.Errorf("find function: %w", err)
	}

	if len(locations) == 0 {
		return NewResult(fmt.Sprintf("Function '%s' not found in the index.\n\n**Suggestion:** Use `cie_find_function` to search for the function.", args.FunctionName)), nil
	}

	if len(locations) > 1 {
		return NewResult(formatAmbiguousFunctions(locations)), nil
	}

	loc := locations[0]

	// Build git log -L command for line-based history
	// Format: git log -L start,end:file --oneline --no-patch --format="%h|%ad|%an|%s"
	gitArgs := []string{
		"log",
		fmt.Sprintf("-L%d,%d:%s", loc.StartLine, loc.EndLine, loc.FilePath),
		"--no-patch",
		"--format=%h|%ad|%an|%s",
		"--date=short",
		fmt.Sprintf("-n%d", args.Limit),
	}

	if args.Since != "" {
		gitArgs = append(gitArgs, "--since="+args.Since)
	}

	output, err := git.Run(ctx, gitArgs...)
	if err != nil {
		// git log -L can fail if the file was renamed or lines changed drastically
		// Fall back to file-based history
		return fallbackToFileHistory(ctx, git, loc, args)
	}

	return NewResult(formatFunctionHistory(loc, output, args)), nil
}

// fallbackToFileHistory uses regular git log when -L fails.
func fallbackToFileHistory(ctx context.Context, git GitRunner, loc FunctionLocation, args FunctionHistoryArgs) (*ToolResult, error) {
	gitArgs := []string{
		"log",
		"--oneline",
		"--format=%h|%ad|%an|%s",
		"--date=short",
		fmt.Sprintf("-n%d", args.Limit),
		"--",
		loc.FilePath,
	}

	if args.Since != "" {
		gitArgs = append(gitArgs, "--since="+args.Since)
	}

	output, err := git.Run(ctx, gitArgs...)
	if err != nil {
		return NewError(fmt.Sprintf("Failed to get git history: %s", err)), nil
	}

	result := formatFunctionHistory(loc, output, args)
	result += "\n\n⚠️ **Note:** Used file-level history because line-based tracking failed (function may have been moved/renamed)."
	return NewResult(result), nil
}

// formatFunctionHistory formats git log output as markdown.
func formatFunctionHistory(loc FunctionLocation, gitOutput string, args FunctionHistoryArgs) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Commit History for `%s`\n", loc.Name))
	sb.WriteString(fmt.Sprintf("**File:** `%s:%d-%d`\n\n", loc.FilePath, loc.StartLine, loc.EndLine))

	lines := strings.Split(strings.TrimSpace(gitOutput), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		sb.WriteString("_No commits found._\n")
		return sb.String()
	}

	sb.WriteString("| Commit | Date | Author | Message |\n")
	sb.WriteString("|--------|------|--------|--------|\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commit, date, author, message := parts[0], parts[1], parts[2], parts[3]
		// Truncate long messages
		if len(message) > 60 {
			message = message[:57] + "..."
		}
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", commit, date, author, message))
	}

	return sb.String()
}

// FindIntroductionArgs holds arguments for the cie_find_introduction tool.
type FindIntroductionArgs struct {
	CodeSnippet  string // Required: code pattern to find introduction of
	FunctionName string // Optional: limit search to function's file
	PathPattern  string // Optional: limit scope to specific paths
}

// FindIntroduction finds the commit that first introduced a code pattern.
// Uses git log -S (pickaxe) to find when the pattern was first added.
func FindIntroduction(ctx context.Context, client Querier, git GitRunner, args FindIntroductionArgs) (*ToolResult, error) {
	if args.CodeSnippet == "" {
		return NewError("Error: 'code_snippet' is required"), nil
	}

	// Build git log -S command
	gitArgs := []string{
		"log",
		"-S", args.CodeSnippet,
		"--reverse", // Oldest first
		"--format=%H|%ad|%an|%s",
		"--date=short",
		"-n1", // Only first (introduction) commit
	}

	// If function name provided, get its file path to narrow scope
	if args.FunctionName != "" {
		locations, err := FindFunctionsWithLocation(ctx, client, args.FunctionName, args.PathPattern)
		if err != nil {
			return nil, fmt.Errorf("find function: %w", err)
		}
		if len(locations) > 0 {
			gitArgs = append(gitArgs, "--", locations[0].FilePath)
		}
	} else if args.PathPattern != "" {
		gitArgs = append(gitArgs, "--", args.PathPattern)
	}

	output, err := git.Run(ctx, gitArgs...)
	if err != nil {
		return NewError(fmt.Sprintf("Failed to search git history: %s", err)), nil
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return NewResult(fmt.Sprintf("Pattern `%s` not found in git history.\n\n**Possible reasons:**\n- Pattern may have never been added as a single change\n- Pattern may predate the repository history\n- Check spelling and try a simpler pattern", args.CodeSnippet)), nil
	}

	// Parse the output
	parts := strings.SplitN(output, "|", 4)
	if len(parts) < 4 {
		return NewResult("Could not parse git output: " + output), nil
	}

	commit, date, author, message := parts[0], parts[1], parts[2], parts[3]

	// Get diff context for the commit
	diffOutput, _ := git.Run(ctx, "show", commit, "-S", args.CodeSnippet, "--format=", "--stat")

	var sb strings.Builder
	sb.WriteString("## Introduction of Pattern\n")
	sb.WriteString(fmt.Sprintf("**Pattern:** `%s`\n", args.CodeSnippet))
	sb.WriteString(fmt.Sprintf("**Introduced in:** `%s` on %s\n", commit[:7], date))
	sb.WriteString(fmt.Sprintf("**Author:** %s\n", author))
	sb.WriteString(fmt.Sprintf("**Message:** %s\n", message))

	if diffOutput != "" {
		sb.WriteString("\n**Files changed:**\n```\n")
		sb.WriteString(strings.TrimSpace(diffOutput))
		sb.WriteString("\n```\n")
	}

	return NewResult(sb.String()), nil
}

// BlameFunctionArgs holds arguments for the cie_blame_function tool.
type BlameFunctionArgs struct {
	FunctionName string // Required: name of the function to analyze
	PathPattern  string // Optional: disambiguate multiple matches
	ShowLines    bool   // Include line-by-line breakdown
}

// BlameAuthor represents blame statistics for a single author.
type BlameAuthor struct {
	Name       string
	Lines      int
	Percentage float64
	LastCommit string
}

// BlameFunction provides aggregated blame analysis showing who owns what percentage of a function.
func BlameFunction(ctx context.Context, client Querier, git GitRunner, args BlameFunctionArgs) (*ToolResult, error) {
	if args.FunctionName == "" {
		return NewError("Error: 'function_name' is required"), nil
	}

	// Find function location
	locations, err := FindFunctionsWithLocation(ctx, client, args.FunctionName, args.PathPattern)
	if err != nil {
		return nil, fmt.Errorf("find function: %w", err)
	}

	if len(locations) == 0 {
		return NewResult(fmt.Sprintf("Function '%s' not found in the index.\n\n**Suggestion:** Use `cie_find_function` to search for the function.", args.FunctionName)), nil
	}

	if len(locations) > 1 {
		return NewResult(formatAmbiguousFunctions(locations)), nil
	}

	loc := locations[0]
	totalLines := loc.EndLine - loc.StartLine + 1

	// Run git blame with porcelain format for parsing
	gitArgs := []string{
		"blame",
		fmt.Sprintf("-L%d,%d", loc.StartLine, loc.EndLine),
		"--line-porcelain",
		loc.FilePath,
	}

	output, err := git.Run(ctx, gitArgs...)
	if err != nil {
		return NewError(fmt.Sprintf("Failed to get blame info: %s", err)), nil
	}

	// Parse porcelain blame output
	authors := parseBlameOutput(output)

	return NewResult(formatBlameResult(loc, authors, totalLines, args.ShowLines)), nil
}

// parseBlameOutput parses git blame --line-porcelain output and aggregates by author.
func parseBlameOutput(output string) map[string]*BlameAuthor {
	authors := make(map[string]*BlameAuthor)
	lines := strings.Split(output, "\n")

	var currentCommit, currentAuthor string

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Lines starting with a 40-char hash are commit headers
		if len(line) >= 40 && isHexString(line[:40]) {
			currentCommit = line[:7] // Short hash
			continue
		}

		// Author line
		if strings.HasPrefix(line, "author ") {
			currentAuthor = strings.TrimPrefix(line, "author ")
			continue
		}

		// Code line (starts with tab)
		if strings.HasPrefix(line, "\t") && currentAuthor != "" {
			if authors[currentAuthor] == nil {
				authors[currentAuthor] = &BlameAuthor{
					Name:       currentAuthor,
					LastCommit: currentCommit,
				}
			}
			authors[currentAuthor].Lines++
			// Keep the most recent commit (first one seen in blame output is most recent)
			if authors[currentAuthor].LastCommit == "" {
				authors[currentAuthor].LastCommit = currentCommit
			}
		}
	}

	return authors
}

// isHexString checks if a string contains only hex characters.
func isHexString(s string) bool {
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// formatBlameResult formats the blame analysis as markdown.
func formatBlameResult(loc FunctionLocation, authors map[string]*BlameAuthor, totalLines int, showLines bool) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Blame Analysis for `%s`\n", loc.Name))
	sb.WriteString(fmt.Sprintf("**File:** `%s:%d-%d` (%d lines)\n\n", loc.FilePath, loc.StartLine, loc.EndLine, totalLines))

	if len(authors) == 0 {
		sb.WriteString("_No blame data available._\n")
		return sb.String()
	}

	// Calculate percentages and sort by lines
	sorted := sortAuthorsByLines(authors, totalLines)

	sb.WriteString("| Author | Lines | % | Last Commit |\n")
	sb.WriteString("|--------|------:|--:|-------------|\n")

	for _, author := range sorted {
		sb.WriteString(fmt.Sprintf("| %s | %d | %.0f%% | `%s` |\n",
			author.Name, author.Lines, author.Percentage, author.LastCommit))
	}

	return sb.String()
}

// sortAuthorsByLines sorts authors by line count (descending) and calculates percentages.
func sortAuthorsByLines(authors map[string]*BlameAuthor, totalLines int) []*BlameAuthor {
	// Convert map to slice
	var sorted []*BlameAuthor
	for _, author := range authors {
		if totalLines > 0 {
			author.Percentage = float64(author.Lines) / float64(totalLines) * 100
		}
		sorted = append(sorted, author)
	}

	// Simple bubble sort by lines descending
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Lines > sorted[i].Lines {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
