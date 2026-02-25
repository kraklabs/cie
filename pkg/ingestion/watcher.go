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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kraklabs/cie/pkg/storage"
)

// FileWatcher monitors the filesystem for changes and queues reindex requests.
// It does NOT access the database directly - it only detects changes and
// notifies the ReindexCoordinator, which manages the actual reindex scheduling.
type FileWatcher struct {
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	logger   *slog.Logger
	rootPath string
	config   storage.ReindexConfig

	// coordinator manages indexing state
	coordinator *storage.ReindexCoordinator

	// debounceTimer is used to batch rapid file changes
	debounceTimer *time.Timer
	debounceMu    sync.Mutex

	// isRunning indicates if the watcher is active
	isRunning bool
	
	// stopChan signals the watcher to stop
	stopChan chan struct{}
}

// NewFileWatcher creates a new filesystem watcher.
//
// The watcher only DETECTS file changes and RECORDS them via the coordinator.
// It NEVER creates database connections or runs reindex operations directly.
//
// Parameters:
//   - rootPath: The directory to watch
//   - config: Auto-reindex configuration
//   - logger: Optional logger
func NewFileWatcher(
	rootPath string,
	config storage.ReindexConfig,
	logger *slog.Logger,
) (*FileWatcher, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Verify root path exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access watch root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("watch root is not a directory: %s", rootPath)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	return &FileWatcher{
		watcher:     watcher,
		logger:      logger,
		rootPath:    rootPath,
		config:      config,
		coordinator: storage.GetReindexCoordinator(),
		stopChan:    make(chan struct{}),
	}, nil
}

// Start begins watching for file changes.
func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.isRunning {
		return nil // Already running
	}

	// Add root path and all subdirectories
	if err := fw.addWatchRecursive(fw.rootPath); err != nil {
		return fmt.Errorf("add watch recursive: %w", err)
	}

	fw.isRunning = true
	go fw.watchLoop()

	fw.logger.Info("file_watcher.started",
		"root", fw.rootPath,
		"debounce_ms", fw.config.DebounceMs,
		"auto_reindex", fw.config.AutoReindex,
	)

	return nil
}

// Stop halts the file watcher.
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	if !fw.isRunning {
		fw.mu.Unlock()
		return nil
	}

	fw.isRunning = false
	close(fw.stopChan)
	
	// Stop debounce timer
	fw.debounceMu.Lock()
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}
	fw.debounceMu.Unlock()
	
	fw.mu.Unlock()

	if err := fw.watcher.Close(); err != nil {
		fw.logger.Warn("file_watcher.close_error", "err", err)
	}

	fw.logger.Info("file_watcher.stopped")
	return nil
}

// IsRunning returns true if the watcher is active.
func (fw *FileWatcher) IsRunning() bool {
	fw.mu.RLock()
	defer fw.mu.RUnlock()
	return fw.isRunning
}

// UpdateConfig updates the watcher configuration.
// Changes take effect immediately for new file events.
func (fw *FileWatcher) UpdateConfig(config storage.ReindexConfig) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.config = config
	fw.coordinator.SetConfig(config)
}

// addWatchRecursive adds all subdirectories under root to the watcher.
func (fw *FileWatcher) addWatchRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip paths we can't access
		}

		if !info.IsDir() {
			return nil // Only watch directories
		}

		// Skip excluded directories
		if fw.shouldExcludeDir(path) {
			return filepath.SkipDir
		}

		if err := fw.watcher.Add(path); err != nil {
			fw.logger.Warn("file_watcher.add_failed",
				"path", path,
				"err", err,
			)
		}

		return nil
	})
}

// shouldExcludeDir checks if a directory should be excluded from watching.
func (fw *FileWatcher) shouldExcludeDir(path string) bool {
	base := filepath.Base(path)

	// Common directories to always exclude
	alwaysExclude := []string{
		".git", ".hg", ".svn",
		"node_modules", "vendor",
		".idea", ".vscode",
		"dist", "build", "out", "bin",
		".next", ".nuxt",
		"__pycache__", ".pytest_cache",
		"target", // Rust
		".cargo",
		".gradle",
		".cie",
	}

	for _, ex := range alwaysExclude {
		if base == ex {
			return true
		}
	}

	// Check user-defined exclude patterns
	for _, pattern := range fw.config.ExcludePatterns {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return true
		}
		// Also try matching against the full path relative to root
		relPath, _ := filepath.Rel(fw.rootPath, path)
		matched, _ = filepath.Match(pattern, relPath)
		if matched {
			return true
		}
	}

	return false
}

// shouldProcessFile checks if a file change should trigger reindexing.
func (fw *FileWatcher) shouldProcessFile(path string) bool {
	// Check extension
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}

	// Check if extension is in watch list
	found := false
	for _, watchExt := range fw.config.WatchExtensions {
		if ext == watchExt {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	// Check exclude patterns against the path
	relPath, _ := filepath.Rel(fw.rootPath, path)
	for _, pattern := range fw.config.ExcludePatterns {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return false
		}
		matched, _ = filepath.Match(pattern, filepath.Base(path))
		if matched {
			return false
		}
	}

	return true
}

// watchLoop processes file system events.
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case <-fw.stopChan:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Warn("file_watcher.error", "err", err)
		}
	}
}

// handleEvent processes a single fsnotify event.
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Handle new directories
	if event.Op&fsnotify.Create == fsnotify.Create {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			if !fw.shouldExcludeDir(event.Name) {
				if err := fw.addWatchRecursive(event.Name); err != nil {
					fw.logger.Warn("file_watcher.watch_new_dir_failed",
						"path", event.Name,
						"err", err,
					)
				}
			}
			return // Don't process directory creation as a file change
		}
	}

	// Only process relevant file changes
	if !fw.shouldProcessFile(event.Name) {
		return
	}

	// Record the change with the coordinator
	fw.coordinator.RecordPendingChange(event.Name)

	fw.logger.Debug("file_watcher.change_detected",
		"path", event.Name,
		"op", event.Op.String(),
	)

	// Schedule debounced reindex if auto-reindex enabled
	if fw.config.AutoReindex {
		fw.scheduleReindex()
	}
}

// scheduleReindex schedules a reindex operation with debouncing.
func (fw *FileWatcher) scheduleReindex() {
	fw.debounceMu.Lock()
	defer fw.debounceMu.Unlock()

	// Cancel existing timer if any
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}

	debounceDuration := time.Duration(fw.config.DebounceMs) * time.Millisecond
	if debounceDuration < 100*time.Millisecond {
		debounceDuration = 100 * time.Millisecond
	}

	fw.debounceTimer = time.AfterFunc(debounceDuration, func() {
		fw.triggerReindex()
	})
}

// triggerReindex queues the actual reindex operation.
// This only QUEUES the reindex - the coordinator decides when to run it.
func (fw *FileWatcher) triggerReindex() {
	// Get pending changes
	paths := fw.coordinator.GetPendingChanges()
	if len(paths) == 0 {
		return
	}

	fw.logger.Info("file_watcher.queueing_reindex",
		"changed_files", len(paths),
	)

	// Queue the reindex job with the coordinator
	// The coordinator will run it when safe (no active queries, etc.)
	jobID := fw.coordinator.QueueReindex(paths)
	
	if jobID == "" {
		fw.logger.Warn("file_watcher.reindex_queue_full",
			"paths", len(paths),
		)
	} else {
		fw.logger.Info("file_watcher.reindex_queued",
			"job_id", jobID,
			"paths", len(paths),
		)
	}
}
