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
	"strconv"
	"syscall"
	"time"
)

// IndexQueue manages the lock file and commit queue for index operations.
// It prevents concurrent indexing and queues commits for sequential processing.
type IndexQueue struct {
	projectID string
	baseDir   string // ~/.cie/<project>/
	lockPath  string // ~/.cie/<project>/index.lock
	queuePath string // ~/.cie/<project>/index.queue
	lockFile  *os.File
}

// LockInfo contains information about the current lock holder.
type LockInfo struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

// NewIndexQueue creates a new IndexQueue for the given project.
func NewIndexQueue(projectID string) (*IndexQueue, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".cie", projectID)
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("create queue dir: %w", err)
	}

	return &IndexQueue{
		projectID: projectID,
		baseDir:   baseDir,
		lockPath:  filepath.Join(baseDir, "index.lock"),
		queuePath: filepath.Join(baseDir, "index.queue"),
	}, nil
}

// TryAcquireLock attempts to acquire the index lock.
// Returns true if lock was acquired, false if another process holds it.
func (q *IndexQueue) TryAcquireLock() (bool, error) {
	f, err := os.OpenFile(q.lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, fmt.Errorf("open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil // Lock is held by another process
		}
		return false, fmt.Errorf("flock: %w", err)
	}

	// Write our PID and start time to the lock file
	if err := f.Truncate(0); err != nil {
		_ = f.Close()
		return false, fmt.Errorf("truncate lock file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		_ = f.Close()
		return false, fmt.Errorf("seek lock file: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%d %d\n", os.Getpid(), time.Now().Unix()); err != nil {
		_ = f.Close()
		return false, fmt.Errorf("write lock file: %w", err)
	}

	q.lockFile = f
	return true, nil
}

// WaitForLock waits up to timeout for the lock to become available.
// Returns true if lock was acquired, false if timeout.
func (q *IndexQueue) WaitForLock(timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		acquired, err := q.TryAcquireLock()
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}

		// Wait a bit before retrying
		time.Sleep(500 * time.Millisecond)
	}

	return false, nil
}

// ReleaseLock releases the index lock.
func (q *IndexQueue) ReleaseLock() {
	if q.lockFile != nil {
		_ = syscall.Flock(int(q.lockFile.Fd()), syscall.LOCK_UN)
		_ = q.lockFile.Close()
		q.lockFile = nil
	}
}

// GetLockInfo returns information about the current lock holder, if any.
func (q *IndexQueue) GetLockInfo() (*LockInfo, error) {
	data, err := os.ReadFile(q.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pid int
	var timestamp int64
	if _, err := fmt.Sscanf(string(data), "%d %d", &pid, &timestamp); err != nil {
		return nil, fmt.Errorf("parse lock info: %w", err)
	}

	return &LockInfo{
		PID:       pid,
		StartedAt: time.Unix(timestamp, 0),
	}, nil
}

// IsLockStale checks if the lock is stale (process no longer exists).
func (q *IndexQueue) IsLockStale() bool {
	info, err := q.GetLockInfo()
	if err != nil || info == nil {
		return false
	}

	// Check if process is still running
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return true // Process not found
	}

	// On Unix, FindProcess always succeeds; use signal 0 to check if process exists
	err = proc.Signal(syscall.Signal(0))
	return err != nil
}

// AddToQueue adds a commit hash to the pending queue.
func (q *IndexQueue) AddToQueue(commitHash string) error {
	f, err := os.OpenFile(q.queuePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open queue file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintln(f, commitHash); err != nil {
		return fmt.Errorf("write to queue: %w", err)
	}

	return nil
}

// DrainQueue reads and clears all commits from the queue.
// Returns the list of commit hashes in order.
func (q *IndexQueue) DrainQueue() ([]string, error) {
	data, err := os.ReadFile(q.queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read queue: %w", err)
	}

	// Parse commits
	var commits []string
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}

	// Clear the queue
	if err := os.Remove(q.queuePath); err != nil && !os.IsNotExist(err) {
		return commits, fmt.Errorf("clear queue: %w", err)
	}

	return commits, nil
}

// GetQueuedCommits returns the list of queued commits without clearing.
func (q *IndexQueue) GetQueuedCommits() ([]string, error) {
	data, err := os.ReadFile(q.queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var commits []string
	for _, line := range splitLines(string(data)) {
		line = trimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits, nil
}

// QueueStatus contains information about the current queue state.
type QueueStatus struct {
	LockHeld     bool
	LockPID      int
	LockDuration time.Duration
	QueuedCount  int
	QueuedHashes []string
}

// GetStatus returns the current status of the index queue.
func (q *IndexQueue) GetStatus() (*QueueStatus, error) {
	status := &QueueStatus{}

	// Check lock - but verify the process is still alive
	info, _ := q.GetLockInfo()
	if info != nil && !q.IsLockStale() {
		status.LockHeld = true
		status.LockPID = info.PID
		status.LockDuration = time.Since(info.StartedAt)
	}

	// Check queue
	commits, _ := q.GetQueuedCommits()
	status.QueuedCount = len(commits)
	if len(commits) > 5 {
		status.QueuedHashes = commits[:5] // Show first 5
	} else {
		status.QueuedHashes = commits
	}

	return status, nil
}

// Helper functions

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

// FormatDuration formats a duration for human-readable output.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d.Seconds())) + "s"
	}
	if d < time.Hour {
		return strconv.Itoa(int(d.Minutes())) + "m " + strconv.Itoa(int(d.Seconds())%60) + "s"
	}
	return strconv.Itoa(int(d.Hours())) + "h " + strconv.Itoa(int(d.Minutes())%60) + "m"
}
