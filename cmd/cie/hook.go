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
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/kraklabs/cie/internal/errors"
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
func runInstallHook(args []string, configPath string, globals GlobalFlags) {
	fs := flag.NewFlagSet("install-hook", flag.ExitOnError)
	force := fs.Bool("force", false, "Overwrite existing hook")
	remove := fs.Bool("remove", false, "Remove the hook instead of installing")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: cie install-hook [options]

Description:
  Install a git post-commit hook that automatically triggers incremental
  indexing after each commit.

  This ensures your CIE database stays up-to-date as you write code,
  making AI-powered code intelligence always current.

  The hook installs to .git/hooks/post-commit. If a hook already exists,
  use --force to overwrite.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Install the post-commit hook
  cie install-hook

  # Force overwrite existing hook
  cie install-hook --force

  # Remove the installed hook
  cie install-hook --remove

Notes:
  The hook runs 'cie index' in the background after each commit.
  You can also install the hook during 'cie init' with --hook flag.

`)
	}

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Find git directory
	gitDir, err := findGitDir()
	if err != nil {
		errors.FatalError(err, false) // findGitDir returns UserError
	}

	hookPath := filepath.Join(gitDir, "hooks", "post-commit")

	if *remove {
		if err := removeHook(hookPath); err != nil {
			errors.FatalError(err, false) // removeHook returns UserError
		}
		fmt.Println("Git hook removed successfully.")
		return
	}

	if err := installHook(hookPath, *force); err != nil {
		errors.FatalError(err, false) // installHook returns UserError
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
		return "", errors.NewInternalError(
			"Cannot access working directory",
			"Failed to determine current directory path",
			"Check system permissions and try again",
			err,
		)
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
			content, err := os.ReadFile(gitPath) //nolint:gosec // G304: gitPath is constructed from CWD
			if err != nil {
				return "", errors.NewPermissionError(
					"Cannot read git worktree file",
					fmt.Sprintf("Permission denied reading %s", gitPath),
					"Check file permissions in your git repository",
					err,
				)
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

	return "", errors.NewNotFoundError(
		"Git repository not found",
		fmt.Sprintf("Searched from %s to filesystem root, no .git directory found", cwd),
		"Initialize a git repository with 'git init' or run this command in a git-managed directory",
	)
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
	if err := os.MkdirAll(hookDir, 0750); err != nil {
		return errors.NewPermissionError(
			"Cannot create hooks directory",
			fmt.Sprintf("Permission denied creating %s", hookDir),
			"Check .git/hooks/ directory permissions",
			err,
		)
	}

	// Check if hook already exists
	if _, err := os.Stat(hookPath); err == nil {
		if !force {
			// Check if it's our hook
			content, err := os.ReadFile(hookPath) //nolint:gosec // G304: hookPath from user's git repo
			if err == nil && containsCIEMarker(string(content)) {
				fmt.Println("CIE hook already installed. Use --force to reinstall.")
				return nil
			}
			return errors.NewInputError(
				"Hook already exists",
				fmt.Sprintf("A post-commit hook already exists at %s", hookPath),
				"Use 'cie install-hook --force' to overwrite the existing hook, or manually edit it",
			)
		}
	}

	// Write the hook (needs exec permission)
	if err := os.WriteFile(hookPath, []byte(postCommitHookContent), 0750); err != nil { //nolint:gosec // G306: Hook needs exec permission
		return errors.NewPermissionError(
			"Cannot write hook file",
			fmt.Sprintf("Permission denied writing to %s", hookPath),
			"Check file permissions in .git/hooks/ directory",
			err,
		)
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
	content, err := os.ReadFile(hookPath) //nolint:gosec // G304: hookPath from user's git repo
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NewNotFoundError(
				"Hook not found",
				fmt.Sprintf("No post-commit hook exists at %s", hookPath),
				"Run 'cie install-hook' to install the CIE hook first",
			)
		}
		return errors.NewPermissionError(
			"Cannot read hook file",
			fmt.Sprintf("Permission denied reading %s", hookPath),
			"Check file permissions in .git/hooks/ directory",
			err,
		)
	}

	// Check if it's our hook
	if !containsCIEMarker(string(content)) {
		return errors.NewInputError(
			"Hook not installed by CIE",
			fmt.Sprintf("The hook at %s was not installed by CIE", hookPath),
			"Manually remove the hook file if you want to delete it, or use --force when installing",
		)
	}

	// Remove the hook
	if err := os.Remove(hookPath); err != nil {
		return errors.NewPermissionError(
			"Cannot remove hook file",
			fmt.Sprintf("Permission denied deleting %s", hookPath),
			"Check file permissions in .git/hooks/ directory",
			err,
		)
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
	content, err := os.ReadFile(hookPath) //nolint:gosec // G304: hookPath from git dir discovery
	if err != nil {
		return false
	}

	return containsCIEMarker(string(content))
}
