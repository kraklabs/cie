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

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const postCommitHookContent = `#!/bin/sh
# CIE auto-index hook - queues incremental indexing for this commit
# Installed by: cie install-hook
# Remove with: cie install-hook --remove

COMMIT=$(git rev-parse HEAD)
cie index --incremental --until="$COMMIT" --skip-checks --queue 2>/dev/null &
`

// runInstallHook executes the 'install-hook' CLI command, managing git post-commit hooks.
//
// It installs or removes a git post-commit hook that automatically triggers incremental
// indexing after each commit. The hook runs in the background using the queue system
// to handle concurrent commits gracefully.
//
// Flags:
//   - --force: Overwrite existing hook (default: false)
//   - --remove: Remove the hook instead of installing (default: false)
//
// Examples:
//
//	cie install-hook           Install the post-commit hook
//	cie install-hook --force   Overwrite existing hook
//	cie install-hook --remove  Remove the hook
func runInstallHook(args []string, configPath string) {
	fs := flag.NewFlagSet("install-hook", flag.ExitOnError)
	force := fs.Bool("force", false, "Overwrite existing hook")
	remove := fs.Bool("remove", false, "Remove the hook instead of installing")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie install-hook [options]

Installs a git post-commit hook that automatically queues incremental indexing
after each commit. The hook runs in the background and uses the queue system
to handle concurrent commits.

Hook behavior:
  1. On each commit, captures the commit hash
  2. Runs 'cie index --incremental --until=COMMIT --queue' in background
  3. If another index is running, queues the commit for later processing

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Find git directory
	gitDir, err := findGitDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	hookPath := filepath.Join(gitDir, "hooks", "post-commit")

	if *remove {
		if err := removeHook(hookPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Git hook removed successfully.")
		return
	}

	if err := installHook(hookPath, *force); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Git hook installed: %s\n", hookPath)
}

// findGitDir finds the .git directory by walking up the directory tree.
//
// Starting from the current working directory, it searches parent directories
// until it finds a .git directory or reaches the filesystem root.
//
// Returns the absolute path to the .git directory, or an error if not found.
func findGitDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree looking for .git
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() {
				return gitPath, nil
			}
			// .git is a file (worktree), read its contents
			content, err := os.ReadFile(gitPath)
			if err != nil {
				return "", fmt.Errorf("cannot read .git file: %w", err)
			}
			// Parse "gitdir: <path>"
			var gitdir string
			if _, err := fmt.Sscanf(string(content), "gitdir: %s", &gitdir); err == nil {
				if filepath.IsAbs(gitdir) {
					return gitdir, nil
				}
				return filepath.Join(dir, gitdir), nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not a git repository (or any of the parent directories)")
}

// installHook writes the CIE post-commit hook to the specified path.
//
// If the hook file already exists and force is false, it checks whether the existing
// hook is a CIE hook. If force is true, it overwrites any existing hook.
//
// Parameters:
//   - hookPath: Absolute path to the hook file (.git/hooks/post-commit)
//   - force: Whether to overwrite existing hooks
//
// Returns an error if the file cannot be written or if an existing non-CIE hook
// would be overwritten without force=true.
func installHook(hookPath string, force bool) error {
	// Check if hooks directory exists
	hookDir := filepath.Dir(hookPath)
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("cannot create hooks directory: %w", err)
	}

	// Check if hook already exists
	if _, err := os.Stat(hookPath); err == nil {
		if !force {
			// Check if it's our hook
			content, err := os.ReadFile(hookPath)
			if err == nil && containsCIEMarker(string(content)) {
				fmt.Println("CIE hook already installed. Use --force to reinstall.")
				return nil
			}
			return fmt.Errorf("hook already exists at %s\nUse --force to overwrite", hookPath)
		}
	}

	// Write the hook
	if err := os.WriteFile(hookPath, []byte(postCommitHookContent), 0755); err != nil {
		return fmt.Errorf("cannot write hook: %w", err)
	}

	return nil
}

// removeHook removes the CIE post-commit hook if it exists and is a CIE hook.
//
// It only removes the hook if it contains the CIE marker comment, preventing
// accidental removal of user-created hooks.
//
// Parameters:
//   - hookPath: Absolute path to the hook file (.git/hooks/post-commit)
//
// Returns an error if the file cannot be read or deleted, or if the hook
// is not a CIE hook (protection against accidental removal).
func removeHook(hookPath string) error {
	// Check if hook exists
	content, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no hook found at %s", hookPath)
		}
		return fmt.Errorf("cannot read hook: %w", err)
	}

	// Check if it's our hook
	if !containsCIEMarker(string(content)) {
		return fmt.Errorf("hook at %s was not installed by CIE\nManually remove it if needed", hookPath)
	}

	// Remove the hook
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("cannot remove hook: %w", err)
	}

	return nil
}

// containsCIEMarker checks if the hook content contains the CIE marker comment.
//
// The marker "# CIE auto-index hook" identifies hooks installed by CIE, allowing
// safe detection and removal without affecting user-created hooks.
//
// Returns true if the marker is found, false otherwise.
func containsCIEMarker(content string) bool {
	// Check for our marker comment
	for i := 0; i < len(content)-20; i++ {
		if content[i:i+20] == "# CIE auto-index hoo" {
			return true
		}
	}
	return false
}

// IsHookInstalled checks if the CIE git hook is currently installed.
//
// This is an exported function that can be called by other packages to check
// hook installation status without attempting to install or remove hooks.
//
// Returns true if the hook exists and contains the CIE marker, false otherwise.
func IsHookInstalled() bool {
	gitDir, err := findGitDir()
	if err != nil {
		return false
	}

	hookPath := filepath.Join(gitDir, "hooks", "post-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}

	return containsCIEMarker(string(content))
}
