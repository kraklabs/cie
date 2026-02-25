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

package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// IndexingState represents the current state of indexing operations.
// Uses a proper state machine to prevent race conditions.
type IndexingState int

const (
	// IndexingStateReady indicates normal operation, queries allowed.
	IndexingStateReady IndexingState = iota
	
	// IndexingStateDraining indicates queries should finish but new ones blocked.
	// Transitioning to this state waits for active queries to complete.
	IndexingStateDraining
	
	// IndexingStateIndexing indicates reindex is in progress, DB lock released.
	// No queries can run during this state.
	IndexingStateIndexing
	
	// IndexingStateReopening indicates reindex done, reopening DB connection.
	IndexingStateReopening
)

// String returns the string representation of the indexing state.
func (s IndexingState) String() string {
	switch s {
	case IndexingStateReady:
		return "ready"
	case IndexingStateDraining:
		return "draining"
	case IndexingStateIndexing:
		return "indexing"
	case IndexingStateReopening:
		return "reopening"
	default:
		return "unknown"
	}
}

// CanQuery returns true if queries are allowed in this state.
func (s IndexingState) CanQuery() bool {
	return s == IndexingStateReady
}

// IsTransitioning returns true if state is transitioning (draining/reopening).
func (s IndexingState) IsTransitioning() bool {
	return s == IndexingStateDraining || s == IndexingStateReopening
}

// ReindexJob represents a pending or active reindex operation.
type ReindexJob struct {
	ID          string
	Force       bool
	Paths       []string
	CancelFunc  context.CancelFunc
	StartTime   time.Time
	Context     context.Context
}

// ReindexConfig holds configuration for auto-reindex behavior.
type ReindexConfig struct {
	// AutoReindex enables file watching and automatic reindexing.
	AutoReindex bool `yaml:"auto_reindex"`

	// DebounceMs is the delay before triggering reindex after file change.
	DebounceMs int `yaml:"debounce_ms"`

	// ExcludePatterns are glob patterns to ignore during file watching.
	ExcludePatterns []string `yaml:"exclude_patterns"`

	// WatchExtensions are file extensions to watch for changes.
	// If empty, watches common code file extensions.
	WatchExtensions []string `yaml:"watch_extensions"`
}

// DefaultReindexConfig returns default auto-reindex configuration.
func DefaultReindexConfig() ReindexConfig {
	return ReindexConfig{
		AutoReindex:     false,
		DebounceMs:      2000,
		ExcludePatterns: []string{},
		WatchExtensions: []string{
			".go", ".js", ".ts", ".jsx", ".tsx",
			".py", ".rs", ".java", ".kt", ".scala",
			".c", ".cpp", ".h", ".hpp",
			".rb", ".php", ".swift", ".m", ".mm",
		},
	}
}

// ReindexCoordinator manages cooperative locking for reindex operations.
// It uses a state machine to ensure safe transitions between operational modes.
type ReindexCoordinator struct {
	mu     sync.RWMutex
	state  IndexingState
	config ReindexConfig

	// Active query tracking
	activeQueries sync.WaitGroup
	queryCond     *sync.Cond // Signaled when state changes

	// Job tracking
	currentJob    *ReindexJob
	lastJob       *ReindexJob
	jobHistory    []*ReindexJob
	jobQueue      chan *ReindexJob
	
	// Pending changes for auto-reindex
	pendingChanges   map[string]bool
	pendingMu        sync.Mutex
	
	// Statistics
	lastReindexTime *time.Time
	lastError       string
	
	// Callbacks
	onReindexStart func(job *ReindexJob) error
	onReindexDone  func(job *ReindexJob, err error)
	
	// Context for coordinator lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// NewReindexCoordinator creates a new reindex coordinator.
func NewReindexCoordinator() *ReindexCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	rc := &ReindexCoordinator{
		state:          IndexingStateReady,
		config:         DefaultReindexConfig(),
		pendingChanges: make(map[string]bool),
		jobQueue:       make(chan *ReindexJob, 10),
		ctx:            ctx,
		cancel:         cancel,
	}
	rc.queryCond = sync.NewCond(&rc.mu)
	
	// Start job processor
	go rc.jobProcessor()
	
	return rc
}

// Stop shuts down the coordinator.
func (c *ReindexCoordinator) Stop() {
	c.cancel()
	close(c.jobQueue)
}

// GetConfig returns the current reindex configuration.
func (c *ReindexCoordinator) GetConfig() ReindexConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}

// SetConfig updates the reindex configuration.
func (c *ReindexCoordinator) SetConfig(config ReindexConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetState returns the current indexing state.
func (c *ReindexCoordinator) GetState() IndexingState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// CanQuery returns true if queries are currently allowed.
func (c *ReindexCoordinator) CanQuery() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.CanQuery()
}

// AcquireRead registers an active query and blocks if indexing is in progress.
// Returns an error if the context is cancelled or indexing starts.
// Must be paired with ReleaseRead().
func (c *ReindexCoordinator) AcquireRead(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Fast path: if ready, acquire immediately
	if c.state == IndexingStateReady {
		c.activeQueries.Add(1)
		return nil
	}

	// If already indexing, fail fast
	if c.state == IndexingStateIndexing {
		return fmt.Errorf("indexing in progress")
	}

	// Wait for state to become ready or context to cancel
	for c.state != IndexingStateReady {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// If transitioning, wait
		if c.state.IsTransitioning() || c.state == IndexingStateIndexing {
			// Release lock and wait for state change
			c.queryCond.Wait()
			
			// Check if now indexing
			if c.state == IndexingStateIndexing {
				return fmt.Errorf("indexing in progress")
			}
		}
	}

	c.activeQueries.Add(1)
	return nil
}

// ReleaseRead decrements the active query counter.
// Must be called after AcquireRead(), ideally with defer.
func (c *ReindexCoordinator) ReleaseRead() {
	c.activeQueries.Done()
	c.queryCond.Broadcast() // Signal any waiting drain operations
}

// StartReindex initiates a reindex operation with proper state transitions.
// Waits for all active queries to complete before returning.
// Returns the job ID or an error if another reindex is in progress.
func (c *ReindexCoordinator) StartReindex(ctx context.Context, force bool, paths []string) (string, error) {
	c.mu.Lock()

	// Check current state
	switch c.state {
	case IndexingStateDraining, IndexingStateIndexing, IndexingStateReopening:
		jobID := ""
		if c.currentJob != nil {
			jobID = c.currentJob.ID
		}
		c.mu.Unlock()
		return "", fmt.Errorf("reindex already in progress (job: %s, state: %s)", jobID, c.state.String())
	}

	// Generate job ID
	jobID := fmt.Sprintf("reindex-%d", time.Now().Unix())
	jobCtx, cancel := context.WithCancel(c.ctx)
	
	job := &ReindexJob{
		ID:        jobID,
		Force:     force,
		Paths:     paths,
		StartTime: time.Now(),
		Context:   jobCtx,
		CancelFunc: cancel,
	}

	// Transition to draining state
	c.state = IndexingStateDraining
	c.currentJob = job
	c.queryCond.Broadcast() // Wake up waiting queries so they can fail fast

	// Release lock before waiting for queries
	c.mu.Unlock()

	// Wait for active queries to complete with timeout
	done := make(chan struct{})
	go func() {
		c.activeQueries.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All queries finished
	case <-ctx.Done():
		// Request cancelled, revert state
		c.mu.Lock()
		c.state = IndexingStateReady
		c.currentJob = nil
		c.mu.Unlock()
		cancel()
		return "", ctx.Err()
	case <-time.After(30 * time.Second):
		// Timeout waiting for queries
		c.mu.Lock()
		c.state = IndexingStateReady
		c.currentJob = nil
		c.mu.Unlock()
		cancel()
		return "", fmt.Errorf("timeout waiting for active queries to complete")
	}

	// Transition to indexing state
	c.mu.Lock()
	c.state = IndexingStateIndexing
	c.mu.Unlock()

	return jobID, nil
}

// FinishReindex marks the reindex operation as complete.
func (c *ReindexCoordinator) FinishReindex(jobID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Verify we're finishing the current job
	if c.currentJob == nil || c.currentJob.ID != jobID {
		return // Job was cancelled or already finished
	}

	now := time.Now()
	c.lastReindexTime = &now
	
	if err != nil {
		c.lastError = err.Error()
	} else {
		c.lastError = ""
	}

	// Save to history
	c.lastJob = c.currentJob
	if len(c.jobHistory) >= 10 {
		c.jobHistory = c.jobHistory[1:]
	}
	c.jobHistory = append(c.jobHistory, c.currentJob)
	
	// Clear pending changes
	c.pendingMu.Lock()
	c.pendingChanges = make(map[string]bool)
	c.pendingMu.Unlock()

	// Transition back to ready
	c.state = IndexingStateReady
	c.currentJob = nil
	c.queryCond.Broadcast() // Wake up waiting queries
}

// CancelReindex cancels the current reindex operation if one is running.
func (c *ReindexCoordinator) CancelReindex(jobID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentJob == nil || c.currentJob.ID != jobID {
		return false
	}

	if c.currentJob.CancelFunc != nil {
		c.currentJob.CancelFunc()
	}

	c.lastError = "cancelled by user"
	c.state = IndexingStateReady
	c.currentJob = nil
	c.queryCond.Broadcast()
	
	return true
}

// QueueReindex adds a reindex job to the queue (for auto-reindex).
func (c *ReindexCoordinator) QueueReindex(paths []string) string {
	jobID := fmt.Sprintf("auto-%d", time.Now().Unix())
	
	job := &ReindexJob{
		ID:        jobID,
		Force:     false,
		Paths:     paths,
		StartTime: time.Now(),
	}

	select {
	case c.jobQueue <- job:
		// Queued successfully
	default:
		// Queue full, drop job
		return ""
	}

	return jobID
}

// jobProcessor processes queued reindex jobs.
func (c *ReindexCoordinator) jobProcessor() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case job, ok := <-c.jobQueue:
			if !ok {
				return
			}
			
			// Check if auto-reindex is enabled
			config := c.GetConfig()
			if !config.AutoReindex {
				continue
			}
			
			// Check if already indexing
			if c.GetState() != IndexingStateReady {
				continue
			}
			
			// Trigger reindex via callback if set
			if c.onReindexStart != nil {
				// Note: This runs async, actual implementation would need proper coordination
				_ = c.onReindexStart(job)
			}
		}
	}
}

// SetReindexCallbacks sets the callbacks for reindex operations.
func (c *ReindexCoordinator) SetReindexCallbacks(onStart func(job *ReindexJob) error, onDone func(job *ReindexJob, err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onReindexStart = onStart
	c.onReindexDone = onDone
}

// GetCurrentJob returns the currently active job, if any.
func (c *ReindexCoordinator) GetCurrentJob() *ReindexJob {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentJob
}

// GetStatus returns the current indexing status.
func (c *ReindexCoordinator) GetStatus() IndexingStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := IndexingStatus{
		State:              c.state.String(),
		IsIndexing:         c.state == IndexingStateIndexing,
		IsDraining:         c.state == IndexingStateDraining,
		IsReopening:        c.state == IndexingStateReopening,
		AutoReindexEnabled: c.config.AutoReindex,
	}

	if c.currentJob != nil {
		status.JobID = c.currentJob.ID
		status.StartedAt = c.currentJob.StartTime.Format(time.RFC3339)
	}

	if c.lastReindexTime != nil {
		status.LastReindexAt = c.lastReindexTime.Format(time.RFC3339)
	}

	if c.lastError != "" {
		status.LastError = c.lastError
	}

	c.pendingMu.Lock()
	status.PendingChanges = len(c.pendingChanges)
	c.pendingMu.Unlock()

	return status
}

// RecordPendingChange increments the pending change counter.
func (c *ReindexCoordinator) RecordPendingChange(path string) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.pendingChanges[path] = true
}

// GetPendingChanges returns the list of pending file changes.
func (c *ReindexCoordinator) GetPendingChanges() []string {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	
	paths := make([]string, 0, len(c.pendingChanges))
	for path := range c.pendingChanges {
		paths = append(paths, path)
	}
	return paths
}

// ClearPendingChanges resets the pending change counter.
func (c *ReindexCoordinator) ClearPendingChanges() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.pendingChanges = make(map[string]bool)
}

// IndexingStatus provides a snapshot of the current indexing state.
type IndexingStatus struct {
	State              string `json:"state"`
	IsIndexing         bool   `json:"is_indexing"`
	IsDraining         bool   `json:"is_draining"`
	IsReopening        bool   `json:"is_reopening"`
	AutoReindexEnabled bool   `json:"auto_reindex_enabled"`
	PendingChanges     int    `json:"pending_changes"`
	JobID              string `json:"job_id,omitempty"`
	StartedAt          string `json:"started_at,omitempty"`
	LastReindexAt      string `json:"last_reindex_at,omitempty"`
	LastError          string `json:"last_error,omitempty"`
}

// Global coordinator instance.
var globalCoordinator = NewReindexCoordinator()

// GetReindexCoordinator returns the global reindex coordinator.
func GetReindexCoordinator() *ReindexCoordinator {
	return globalCoordinator
}

// CheckIndexingInProgress returns an error if indexing is in progress.
func CheckIndexingInProgress() error {
	if !globalCoordinator.CanQuery() {
		status := globalCoordinator.GetStatus()
		if status.IsIndexing {
			return fmt.Errorf("indexing in progress (job: %s, started: %s) - try again shortly", 
				status.JobID, status.StartedAt)
		}
		return fmt.Errorf("indexing state: %s - try again shortly", status.State)
	}
	return nil
}
