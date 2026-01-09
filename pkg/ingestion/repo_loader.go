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
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"log/slog"
)

var (
	// validGitURLPattern matches valid git URLs (https, ssh, file)
	// Allows: https://github.com/user/repo.git, git@github.com:user/repo.git, file:///path/to/repo
	validGitURLPattern = regexp.MustCompile(`^(https?://|git@|ssh://|file://)[\w.\-@:/%]+$`)

	// dangerousCharsPattern matches characters that could be used for command injection
	dangerousCharsPattern = regexp.MustCompile(`[;&|$` + "`" + `\n\r\\]`)
)

// RepoLoader loads repository contents from git URL or local path.
type RepoLoader struct {
	logger     *slog.Logger
	tempDirs   []string // Track temporary directories for cleanup
	tempDirsMu sync.Mutex
}

// NewRepoLoader creates a new repository loader.
func NewRepoLoader(logger *slog.Logger) *RepoLoader {
	if logger == nil {
		logger = slog.Default()
	}
	return &RepoLoader{
		logger:   logger,
		tempDirs: make([]string, 0),
	}
}

// Close cleans up temporary directories created by git clones.
func (rl *RepoLoader) Close() error {
	rl.tempDirsMu.Lock()
	defer rl.tempDirsMu.Unlock()

	var lastErr error
	for _, dir := range rl.tempDirs {
		if err := os.RemoveAll(dir); err != nil {
			rl.logger.Warn("repo.cleanup.error", "dir", dir, "err", err)
			lastErr = err
		}
	}
	rl.tempDirs = nil
	return lastErr
}

// LoadResult contains the loaded repository information.
type LoadResult struct {
	RootPath    string // Absolute path to repository root
	Files       []FileInfo
	FileCount   int
	TotalSize   int64
	Languages   map[string]int // Language -> file count
	SkipReasons map[string]int // Reason -> count (e.g., "excluded", "too_large", "unsupported_language")
}

// FileInfo represents a file in the repository.
type FileInfo struct {
	Path     string // Relative path from repo root
	FullPath string // Absolute path
	Size     int64
	Language string // Detected from extension
}

// LoadRepository loads a repository from the specified source.
// For git URLs, it clones to a temporary directory.
// For local paths, it reads directly.
func (rl *RepoLoader) LoadRepository(source RepoSource, excludeGlobs []string, maxFileSize int64) (*LoadResult, error) {
	var rootPath string
	var err error

	switch source.Type {
	case "git_url":
		rootPath, err = rl.cloneGitRepo(source.Value)
		if err != nil {
			return nil, fmt.Errorf("clone git repo: %w", err)
		}
		// Cleanup will be handled by caller or via debug flag
	case "local_path":
		rootPath, err = filepath.Abs(source.Value)
		if err != nil {
			return nil, fmt.Errorf("resolve local path: %w", err)
		}
		// Validate path to prevent path traversal attacks
		if err := rl.validateLocalPath(rootPath); err != nil {
			return nil, fmt.Errorf("invalid local path: %w", err)
		}
		// Verify it's a directory
		info, err := os.Stat(rootPath)
		if err != nil {
			return nil, fmt.Errorf("stat local path: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("local path is not a directory: %s", rootPath)
		}
	default:
		return nil, fmt.Errorf("unsupported repo source type: %s", source.Type)
	}

	rl.logger.Info("repo.load.start", "root", rootPath, "type", source.Type)

	// Walk repository and collect files
	files, skipReasons, err := rl.walkRepository(rootPath, excludeGlobs, maxFileSize)
	if err != nil {
		return nil, fmt.Errorf("walk repository: %w", err)
	}

	// Compute statistics
	totalSize := int64(0)
	languages := make(map[string]int)
	for _, f := range files {
		totalSize += f.Size
		if f.Language != "" {
			languages[f.Language]++
		}
	}

	result := &LoadResult{
		RootPath:    rootPath,
		Files:       files,
		FileCount:   len(files),
		TotalSize:   totalSize,
		Languages:   languages,
		SkipReasons: skipReasons,
	}

	rl.logger.Info("repo.load.complete",
		"files", result.FileCount,
		"total_size", totalSize,
		"languages", languages,
	)

	return result, nil
}

// validateGitURL validates a git URL to prevent command injection.
// Returns an error if the URL is invalid or contains dangerous characters.
func validateGitURL(gitURL string) error {
	// Check for empty URL
	if gitURL == "" {
		return fmt.Errorf("git URL is empty")
	}

	// Check for dangerous characters that could enable command injection
	if dangerousCharsPattern.MatchString(gitURL) {
		return fmt.Errorf("git URL contains dangerous characters")
	}

	// For HTTPS URLs, validate using net/url package
	if strings.HasPrefix(gitURL, "http://") || strings.HasPrefix(gitURL, "https://") {
		parsed, err := url.Parse(gitURL)
		if err != nil {
			return fmt.Errorf("invalid URL format: %w", err)
		}
		// Ensure host is present
		if parsed.Host == "" {
			return fmt.Errorf("git URL missing host")
		}
		// Check for username:password@ in URL (credential leak risk)
		if parsed.User != nil {
			_, hasPassword := parsed.User.Password()
			if hasPassword {
				return fmt.Errorf("git URL should not contain embedded password")
			}
		}
		return nil
	}

	// For SSH URLs (git@host:path or ssh://), validate format
	if strings.HasPrefix(gitURL, "git@") || strings.HasPrefix(gitURL, "ssh://") {
		if !validGitURLPattern.MatchString(gitURL) {
			return fmt.Errorf("invalid SSH git URL format")
		}
		return nil
	}

	// For file:// URLs
	if strings.HasPrefix(gitURL, "file://") {
		return nil
	}

	// Unknown protocol
	return fmt.Errorf("unsupported git URL protocol: must be https://, git@, ssh://, or file://")
}

// cloneGitRepo clones a git repository to a temporary directory.
// Uses exec.Command to run git clone with shallow clone for efficiency.
// The URL is validated to prevent command injection attacks.
func (rl *RepoLoader) cloneGitRepo(gitURL string) (string, error) {
	// Validate URL to prevent command injection
	if err := validateGitURL(gitURL); err != nil {
		return "", fmt.Errorf("invalid git URL: %w", err)
	}

	// Create temporary directory for clone
	tmpDir, err := os.MkdirTemp("", "cie-ingestion-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Run git clone with shallow clone (depth 1) for efficiency
	// This clones only the latest commit, not full history
	// #nosec G204 - gitURL is validated above to prevent command injection
	cmd := exec.Command("git", "clone", "--depth", "1", "--quiet", gitURL, tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Sanitize URL for logging (hide potential tokens in query params)
	logURL := gitURL
	if parsed, err := url.Parse(gitURL); err == nil {
		parsed.RawQuery = "" // Remove query params from logs
		if parsed.User != nil {
			parsed.User = url.User("***") // Hide username
		}
		logURL = parsed.String()
	}

	rl.logger.Info("repo.clone.start", "url", logURL, "temp_dir", tmpDir)

	if err := cmd.Run(); err != nil {
		// Cleanup on failure
		_ = os.RemoveAll(tmpDir) // Ignore cleanup error as clone already failed
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	rl.logger.Info("repo.clone.success", "url", logURL, "temp_dir", tmpDir)

	// Track temp directory for cleanup
	rl.tempDirsMu.Lock()
	rl.tempDirs = append(rl.tempDirs, tmpDir)
	rl.tempDirsMu.Unlock()

	return tmpDir, nil
}

// validateLocalPath validates that a local path is safe and doesn't contain path traversal.
// Validates that the path is absolute and doesn't contain suspicious patterns.
// For additional security, consider restricting to a specific base directory in production.
// NOTE: This validation prevents basic path traversal attacks but does not restrict to a
// specific base directory. In production environments with untrusted input, consider
// adding a base directory restriction.
func (rl *RepoLoader) validateLocalPath(path string) error {
	// Check for path traversal attempts
	cleaned := filepath.Clean(path)
	if cleaned != path {
		// Path contained .. or other traversal attempts
		return fmt.Errorf("path contains traversal attempts: %s", path)
	}

	// Resolve to absolute path for validation
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Check for suspicious patterns (basic security check)
	// Ensure no .. remains after cleaning and absolute resolution
	if strings.Contains(absPath, "..") {
		return fmt.Errorf("path contains suspicious patterns after resolution: %s", absPath)
	}

	// Additional validation: ensure path is actually absolute (not relative)
	// This prevents confusion with relative paths that might be interpreted differently
	if !filepath.IsAbs(absPath) {
		return fmt.Errorf("path did not resolve to absolute path: %s", absPath)
	}

	// Check for empty path or root directory
	if absPath == "" || absPath == "/" {
		return fmt.Errorf("path is empty or root directory, which is not allowed")
	}

	// Additional security: prevent access to sensitive system directories
	// This is a basic check - for production, consider whitelisting allowed directories
	sensitiveDirs := []string{"/etc", "/sys", "/proc", "/dev", "/boot", "/root"}
	for _, sensitive := range sensitiveDirs {
		if strings.HasPrefix(absPath, sensitive+"/") || absPath == sensitive {
			return fmt.Errorf("path is in sensitive system directory: %s", absPath)
		}
	}

	return nil
}

// walkRepository walks the repository directory and collects files.
func (rl *RepoLoader) walkRepository(rootPath string, excludeGlobs []string, maxFileSize int64) ([]FileInfo, map[string]int, error) {
	var files []FileInfo
	skipReasons := make(map[string]int)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue on permission errors
			rl.logger.Warn("repo.walk.error", "path", path, "err", err)
			return nil
		}

		if d.IsDir() {
			// Check if directory should be excluded
			relPath, err := filepath.Rel(rootPath, path)
			if err != nil {
				return nil
			}
			if rl.shouldExclude(relPath, excludeGlobs) {
				skipReasons["excluded_dir"]++
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}
		if rl.shouldExclude(relPath, excludeGlobs) {
			skipReasons["excluded"]++
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Check size limit
		if maxFileSize > 0 && info.Size() > maxFileSize {
			skipReasons["too_large"]++
			rl.logger.Warn("repo.walk.skip_large_file",
				"path", relPath,
				"size", info.Size(),
				"limit", maxFileSize,
			)
			return nil
		}

		// Detect language from extension
		language := detectLanguageFromPath(relPath)

		files = append(files, FileInfo{
			Path:     relPath,
			FullPath: path,
			Size:     info.Size(),
			Language: language,
		})

		return nil
	})

	return files, skipReasons, err
}

// shouldExclude checks if a path matches any exclude glob pattern.
func (rl *RepoLoader) shouldExclude(path string, excludeGlobs []string) bool {
	// Normalize path separators
	normalized := filepath.ToSlash(path)

	for _, pattern := range excludeGlobs {
		if matchesGlob(normalized, pattern) {
			return true
		}
	}
	return false
}

// matchesGlob performs full glob matching with support for:
//   - * : matches any sequence of non-separator characters
//   - ** : matches any sequence of characters including separators (any depth)
//   - ? : matches any single non-separator character
//   - [abc] : matches any character in the brackets
//   - [a-z] : matches any character in the range
//   - [!abc] or [^abc] : matches any character NOT in the brackets
//
// Patterns are matched against the full path. If pattern doesn't start with **,
// it can match anywhere in the path (implicit **/ prefix for convenience).
func matchesGlob(path, pattern string) bool {
	// Normalize pattern
	pattern = filepath.ToSlash(pattern)

	// Common patterns optimization
	// Pattern: dir/** or dir/* - match directory and all contents
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
		// Also check if any path suffix matches the prefix (e.g., "apps/catalog/bin" should match "bin/**")
		parts := strings.Split(path, "/")
		for i := range parts {
			subpath := strings.Join(parts[i:], "/")
			if subpath == prefix || strings.HasPrefix(subpath, prefix+"/") {
				return true
			}
		}
	}

	// Pattern: *.ext - match any file with extension
	if strings.HasPrefix(pattern, "*.") && !strings.Contains(pattern, "/") {
		ext := pattern[1:] // Include the dot
		return strings.HasSuffix(path, ext)
	}

	// Pattern: **/name - match name at any depth
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		// Match at root or at any directory level
		if path == suffix || strings.HasSuffix(path, "/"+suffix) {
			return true
		}
		// Also try matching as a prefix pattern for nested globs
		if matchGlobPattern(path, suffix) {
			return true
		}
		// Check each path component
		parts := strings.Split(path, "/")
		for i := range parts {
			subpath := strings.Join(parts[i:], "/")
			if matchGlobPattern(subpath, suffix) {
				return true
			}
		}
		return false
	}

	// Pattern without **: try exact match first, then as suffix
	if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") && !strings.Contains(pattern, "[") {
		// Literal pattern - exact match or path component match
		return path == pattern || strings.HasSuffix(path, "/"+pattern) || strings.HasPrefix(path, pattern+"/")
	}

	// Full glob pattern matching
	// Try matching from root
	if matchGlobPattern(path, pattern) {
		return true
	}

	// Try matching as suffix (implicit **/ prefix)
	parts := strings.Split(path, "/")
	for i := range parts {
		subpath := strings.Join(parts[i:], "/")
		if matchGlobPattern(subpath, pattern) {
			return true
		}
	}

	return false
}

// matchGlobPattern performs glob pattern matching on a single path.
// This is a robust implementation supporting *, **, ?, and character classes.
func matchGlobPattern(path, pattern string) bool {
	return matchGlobRecursive(path, pattern, 0, 0)
}

// matchGlobRecursive is the recursive implementation of glob matching.
func matchGlobRecursive(path, pattern string, pi, pti int) bool {
	for pi < len(path) || pti < len(pattern) {
		if pti >= len(pattern) {
			return false
		}

		// Handle **
		if pti+1 < len(pattern) && pattern[pti] == '*' && pattern[pti+1] == '*' {
			// ** matches any sequence including separators
			// Skip the **
			nextPti := pti + 2
			// Skip trailing / after ** if present
			if nextPti < len(pattern) && pattern[nextPti] == '/' {
				nextPti++
			}

			// If ** is at the end, it matches everything
			if nextPti >= len(pattern) {
				return true
			}

			// Try matching ** against progressively more of the path
			for i := pi; i <= len(path); i++ {
				if matchGlobRecursive(path, pattern, i, nextPti) {
					return true
				}
			}
			return false
		}

		// Handle single *
		if pattern[pti] == '*' {
			// * matches any sequence of non-separator characters
			nextPti := pti + 1

			// If * is at the end of pattern (or before /), match rest of component
			if nextPti >= len(pattern) {
				// Match to end, but stop at /
				for i := pi; i <= len(path); i++ {
					if i == len(path) || path[i] == '/' {
						if nextPti >= len(pattern) && i == len(path) {
							return true
						}
						if nextPti < len(pattern) && matchGlobRecursive(path, pattern, i, nextPti) {
							return true
						}
					}
				}
				// Also try matching nothing
				if matchGlobRecursive(path, pattern, pi, nextPti) {
					return true
				}
				return false
			}

			// Try matching * against progressively more characters (but not /)
			for i := pi; i <= len(path); i++ {
				if i > pi && path[i-1] == '/' {
					break // * doesn't match across /
				}
				if matchGlobRecursive(path, pattern, i, nextPti) {
					return true
				}
			}
			return false
		}

		// Handle ?
		if pattern[pti] == '?' {
			if pi >= len(path) || path[pi] == '/' {
				return false // ? doesn't match / or end of string
			}
			pi++
			pti++
			continue
		}

		// Handle character class [...]
		if pattern[pti] == '[' {
			if pi >= len(path) {
				return false
			}

			// Find the closing ]
			closeIdx := pti + 1
			if closeIdx < len(pattern) && (pattern[closeIdx] == '!' || pattern[closeIdx] == '^') {
				closeIdx++
			}
			if closeIdx < len(pattern) && pattern[closeIdx] == ']' {
				closeIdx++
			}
			for closeIdx < len(pattern) && pattern[closeIdx] != ']' {
				closeIdx++
			}
			if closeIdx >= len(pattern) {
				// Malformed pattern, treat [ as literal
				if path[pi] != '[' {
					return false
				}
				pi++
				pti++
				continue
			}

			// Parse and match character class
			classContent := pattern[pti+1 : closeIdx]
			matched := matchCharClass(path[pi], classContent)
			if !matched {
				return false
			}
			pi++
			pti = closeIdx + 1
			continue
		}

		// Handle literal character
		if pi >= len(path) {
			return false
		}
		if path[pi] != pattern[pti] {
			return false
		}
		pi++
		pti++
	}

	return pi == len(path) && pti == len(pattern)
}

// matchCharClass checks if a character matches a character class.
// Supports: [abc], [a-z], [!abc], [^abc]
func matchCharClass(c byte, class string) bool {
	if len(class) == 0 {
		return false
	}

	negated := false
	idx := 0

	// Check for negation
	if class[0] == '!' || class[0] == '^' {
		negated = true
		idx = 1
	}

	matched := false
	for idx < len(class) {
		// Handle range a-z
		if idx+2 < len(class) && class[idx+1] == '-' {
			low := class[idx]
			high := class[idx+2]
			if c >= low && c <= high {
				matched = true
			}
			idx += 3
			continue
		}

		// Single character
		if c == class[idx] {
			matched = true
		}
		idx++
	}

	if negated {
		return !matched
	}
	return matched
}

// detectLanguageFromPath detects programming language from file extension.
func detectLanguageFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	langMap := map[string]string{
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".jsx":   "javascript",
		".tsx":   "typescript",
		".java":  "java",
		".rs":    "rust",
		".cpp":   "cpp",
		".c":     "c",
		".h":     "c",
		".hpp":   "cpp",
		".cc":    "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".swift": "swift",
		".kt":    "kotlin",
		".scala": "scala",
		".clj":   "clojure",
		".cljs":  "clojure",
		".sh":    "bash",
		".bash":  "bash",
		".zsh":   "bash",
		".fish":  "bash",
		".proto": "protobuf",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return ""
}
