// Copyright 2025 KrakLabs
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kraklabs/cie/pkg/ingestion"
)

// Папки, которые не следим (экономия дескрипторов и шум).
var watchSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".cie": true, "bin": true,
}

const watchDebounce = 2 * time.Second

// runWatchAndReindex следит за изменениями в репозитории и запускает инкрементальный реиндекс
// с дебаунсом. Работает только в embedded MCP; портабельно для macOS и Linux (fsnotify).
func runWatchAndReindex(s *mcpServer) {
	if s.backend == nil || s.repoPath == "" {
		fmt.Fprintf(os.Stderr, "[CIE watch] skip: backend=%v repoPath=%q\n", s.backend != nil, s.repoPath)
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CIE watch] fsnotify failed: %v\n", err)
		return
	}
	defer watcher.Close()

	// Добавляем директории репо (рекурсивно), пропуская .git, node_modules и т.д.
	watchCount := 0
	skippedDirs := []string{}
	addDirs := func(root string) {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsPermission(err) {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.IsDir() {
				return nil
			}
			base := filepath.Base(path)
			// Пропускаем явно указанные + скрытые директории (кроме корня)
			if watchSkipDirs[base] || (strings.HasPrefix(base, ".") && base != ".") {
				skippedDirs = append(skippedDirs, path)
				return filepath.SkipDir
			}
			if err := watcher.Add(path); err != nil {
				fmt.Fprintf(os.Stderr, "[CIE watch] add %s: %v\n", path, err)
				if os.IsPermission(err) {
					return filepath.SkipDir
				}
			} else {
				watchCount++
			}
			return nil
		})
	}
	addDirs(s.repoPath)
	fmt.Fprintf(os.Stderr, "[CIE watch] watching %d dirs, skipped %d hidden/system dirs\n", watchCount, len(skippedDirs))

	var debounceTimer *time.Timer
	var timerCh <-chan time.Time // nil = не ждём срабатывания
	eventCount := 0

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			eventCount++
			fmt.Fprintf(os.Stderr, "[CIE watch] event #%d: %s op=%s\n", eventCount, event.Name, event.Op)
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.NewTimer(watchDebounce)
			timerCh = debounceTimer.C
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "[CIE watch] fsnotify error: %v\n", err)
		case <-timerCh:
			timerCh = nil
			fmt.Fprintf(os.Stderr, "[CIE watch] debounce fired, events=%d, calling reindex...\n", eventCount)
			if tryStartReindex(s, false) {
				ingestion.AppendIndexLog(filepath.Join(s.repoPath, ".cie"), "reindex triggered (watch)")
				fmt.Fprintf(os.Stderr, "[CIE watch] reindex started after file change\n")
			} else {
				fmt.Fprintf(os.Stderr, "[CIE watch] reindex already in progress\n")
			}
		}
	}
}

// tryStartReindex запускает реиндекс, если он ещё не идёт. Возвращает true, если запуск выполнен.
func tryStartReindex(s *mcpServer, forceFull bool) bool {
	s.reindex.mu.Lock()
	if s.reindex.inProgress {
		s.reindex.mu.Unlock()
		return false
	}
	s.reindex.inProgress = true
	s.reindex.startedAt = time.Now()
	s.reindex.phase = "starting"
	s.reindex.current = 0
	s.reindex.total = 0
	s.reindex.lastErr = nil
	s.reindex.lastResult = nil
	s.reindex.mu.Unlock()
	go runReindexGoroutine(s, forceFull)
	return true
}
