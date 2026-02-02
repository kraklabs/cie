// Copyright 2025 KrakLabs
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	cozo "github.com/kraklabs/cie/pkg/cozodb"
	"github.com/kraklabs/cie/pkg/ingestion"
)

// serveFlags holds configuration for the serve command.
type serveFlags struct {
	port      string
	projectID string
	repoPath  string
}

// indexJob represents an async indexing job.
type indexJob struct {
	ID        string       `json:"job_id"`
	Status    string       `json:"status"` // "running", "completed", "failed"
	Phase     string       `json:"phase,omitempty"`
	Progress  *progress    `json:"progress,omitempty"`
	Result    *indexResult `json:"result,omitempty"`
	Error     string       `json:"error,omitempty"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   *time.Time   `json:"ended_at,omitempty"`
}

type progress struct {
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
}

type indexResult struct {
	FilesProcessed     int    `json:"files_processed"`
	FunctionsExtracted int    `json:"functions_extracted"`
	TypesExtracted     int    `json:"types_extracted"`
	Duration           string `json:"duration"`
}

// cieServer holds the server state.
type cieServer struct {
	projectID string
	dataDir   string
	repoPath  string
	db        cozo.CozoDB
	hasDB     bool
	dbMu      sync.RWMutex
	jobs      map[string]*indexJob
	jobsMu    sync.RWMutex
}

// runServe starts a local HTTP server that exposes the CIE query API.
// This allows the MCP tools to work without requiring the enterprise Edge Cache.
func runServe(args []string, cfg *Config) int {
	f := &serveFlags{}

	// Parse flags
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port", "-p":
			if i+1 < len(args) {
				f.port = args[i+1]
				i++
			}
		case "--project", "--project-id":
			if i+1 < len(args) {
				f.projectID = args[i+1]
				i++
			}
		case "--repo", "--repo-path":
			if i+1 < len(args) {
				f.repoPath = args[i+1]
				i++
			}
		case "--help", "-h":
			printServeUsage()
			return 0
		}
	}

	// Defaults
	if f.port == "" {
		f.port = getEnv("CIE_SERVE_PORT", "8080")
	}
	if f.projectID == "" {
		f.projectID = cfg.ProjectID
	}
	if f.projectID == "" {
		f.projectID = getEnv("CIE_PROJECT_ID", "")
	}
	if f.repoPath == "" {
		f.repoPath = getEnv("CIE_REPO_PATH", "/repo")
	}

	if f.projectID == "" {
		fmt.Fprintln(os.Stderr, "Error: project_id is required. Set CIE_PROJECT_ID, use --project-id, or set it in .cie/project.yaml")
		return 1
	}

	// Determine data directory
	dataDir := getEnv("CIE_DATA_DIR", "")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not get home directory: %v\n", err)
			return 1
		}
		dataDir = filepath.Join(homeDir, ".cie", "data")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not create data directory %s: %v\n", dataDir, err)
		return 1
	}

	dbPath := filepath.Join(dataDir, f.projectID)

	// Create server instance
	srv := &cieServer{
		projectID: f.projectID,
		dataDir:   dataDir,
		repoPath:  f.repoPath,
		jobs:      make(map[string]*indexJob),
	}

	// Try to open existing database (don't fail if it doesn't exist)
	if _, err := os.Stat(dbPath); err == nil {
		log.Printf("[COZO] Opening existing DB: engine=rocksdb path=%s", dbPath)
		db, err := cozo.New("rocksdb", dbPath, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
			return 1
		}
		srv.db = db
		srv.hasDB = true
		defer db.Close()
	} else {
		log.Printf("[INFO] Database not found at %s, will be created on first index", dbPath)
	}

	// Create HTTP server
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", srv.handleHealth)

	// Query endpoint - compatible with Edge Cache API
	mux.HandleFunc("/v1/query", srv.handleQuery)

	// Ensure-mounted endpoint (no-op for local, always ready)
	mux.HandleFunc("/v1/ensure-mounted", srv.handleEnsureMounted)

	// Init endpoint - initialize project
	mux.HandleFunc("/v1/init", srv.handleInit)

	// Index endpoints
	mux.HandleFunc("/v1/index", srv.handleIndex)
	mux.HandleFunc("/v1/index/", srv.handleIndexStatus)

	// Status endpoint
	mux.HandleFunc("/v1/status", srv.handleStatus)

	// Start server
	server := &http.Server{
		Addr:              ":" + f.port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down CIE server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	log.Printf("CIE Server starting on http://0.0.0.0:%s", f.port)
	log.Printf("Project: %s", f.projectID)
	log.Printf("Data dir: %s", dataDir)
	log.Printf("Repo path: %s", f.repoPath)
	log.Println("")
	log.Println("API Endpoints:")
	log.Println("  GET  /health           - Health check")
	log.Println("  POST /v1/init          - Initialize project")
	log.Println("  POST /v1/index         - Start indexing")
	log.Println("  GET  /v1/index/{id}    - Get indexing job status")
	log.Println("  GET  /v1/status        - Get project status")
	log.Println("  POST /v1/query         - Execute CozoScript query")
	log.Println("")
	log.Println("Use this URL for MCP tools:")
	log.Printf("  export CIE_BASE_URL=http://localhost:%s", f.port)
	log.Println("")

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		return 1
	}

	return 0
}

func (s *cieServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.dbMu.RLock()
	hasDB := s.hasDB
	s.dbMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"project_id": s.projectID,
		"indexed":    hasDB,
	})
}

func (s *cieServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.dbMu.RLock()
	hasDB := s.hasDB
	s.dbMu.RUnlock()

	if !hasDB {
		http.Error(w, "database not initialized, run POST /v1/index first", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ProjectID string         `json:"project_id"`
		Script    string         `json:"script"`
		Params    map[string]any `json:"params"`
		TimeoutMs int            `json:"timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Script == "" {
		http.Error(w, "script is required", http.StatusBadRequest)
		return
	}

	// Verify project ID matches (optional, for compatibility)
	if req.ProjectID != "" && req.ProjectID != s.projectID {
		http.Error(w, fmt.Sprintf("project_id mismatch: server is %s, request is %s", s.projectID, req.ProjectID), http.StatusBadRequest)
		return
	}

	// Execute query with timeout
	timeout := 60 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Run query in a goroutine to respect context cancellation
	resultCh := make(chan cozo.NamedRows, 1)
	errCh := make(chan error, 1)

	go func() {
		s.dbMu.RLock()
		result, err := s.db.Run(req.Script, req.Params)
		s.dbMu.RUnlock()
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-ctx.Done():
		http.Error(w, "query timeout", http.StatusRequestTimeout)
		return
	case err := <-errCh:
		http.Error(w, "query error: "+err.Error(), http.StatusInternalServerError)
		return
	case result := <-resultCh:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"Headers": result.Headers,
			"Rows":    result.Rows,
		})
	}
}

func (s *cieServer) handleEnsureMounted(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "log_index": 0})
}

func (s *cieServer) handleInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectID         string `json:"project_id"`
		EmbeddingProvider string `json:"embedding_provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Empty body is OK, use defaults
		req.ProjectID = s.projectID
		req.EmbeddingProvider = "ollama"
	}
	if req.ProjectID == "" {
		req.ProjectID = s.projectID
	}
	if req.EmbeddingProvider == "" {
		req.EmbeddingProvider = "ollama"
	}

	// Create config
	cfg := DefaultConfig(req.ProjectID)
	cfg.Embedding.Provider = req.EmbeddingProvider
	cfg.Embedding.BaseURL = getEnv("OLLAMA_HOST", "http://localhost:11434")
	cfg.Embedding.Model = getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text")

	// Save config to data dir
	configDir := filepath.Join(s.dataDir, req.ProjectID)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		http.Error(w, "failed to create config directory: "+err.Error(), http.StatusInternalServerError)
		return
	}

	configPath := filepath.Join(configDir, "project.yaml")
	if err := SaveConfig(cfg, configPath); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":          true,
		"project_id":  req.ProjectID,
		"config_path": configPath,
	})
}

func (s *cieServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectID string `json:"project_id"`
		RepoPath  string `json:"repo_path"`
		Full      bool   `json:"full"`
	}
	// Decode request body; empty body is OK, use defaults
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.ProjectID == "" {
		req.ProjectID = s.projectID
	}
	if req.RepoPath == "" {
		req.RepoPath = s.repoPath
	}

	// Check if repo path exists
	if _, err := os.Stat(req.RepoPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("repo path not found: %s", req.RepoPath), http.StatusBadRequest)
		return
	}

	// Check if there's already a running job
	s.jobsMu.RLock()
	for _, job := range s.jobs {
		if job.Status == "running" {
			s.jobsMu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":  "indexing already in progress",
				"job_id": job.ID,
			})
			return
		}
	}
	s.jobsMu.RUnlock()

	// Create job
	jobID := fmt.Sprintf("idx-%d", time.Now().UnixNano())
	job := &indexJob{
		ID:        jobID,
		Status:    "running",
		Phase:     "starting",
		StartedAt: time.Now(),
	}

	s.jobsMu.Lock()
	s.jobs[jobID] = job
	s.jobsMu.Unlock()

	// Run indexing in background
	go s.runIndexJob(job, req.ProjectID, req.RepoPath, req.Full)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job_id":  jobID,
		"status":  "running",
		"message": "Indexing started",
	})
}

func (s *cieServer) runIndexJob(job *indexJob, projectID, repoPath string, full bool) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Get embedding config from environment (model name used for setup below)

	// Set environment for embedding
	_ = os.Setenv("OLLAMA_BASE_URL", getEnv("OLLAMA_HOST", "http://localhost:11434"))
	_ = os.Setenv("OLLAMA_EMBED_MODEL", getEnv("OLLAMA_EMBED_MODEL", "nomic-embed-text"))

	dbPath := filepath.Join(s.dataDir, projectID)

	// Close existing database to release the lock before pipeline opens it
	s.dbMu.Lock()
	if s.hasDB {
		s.db.Close()
		s.hasDB = false
	}
	s.dbMu.Unlock()

	// If full reindex, remove existing data
	if full {
		if err := os.RemoveAll(dbPath); err != nil && !os.IsNotExist(err) {
			s.updateJobError(job, fmt.Sprintf("failed to remove existing data: %v", err))
			return
		}
	}

	// Create checkpoint directory
	checkpointDir := filepath.Join(s.dataDir, projectID+"-checkpoints")
	if err := os.MkdirAll(checkpointDir, 0750); err != nil {
		s.updateJobError(job, fmt.Sprintf("failed to create checkpoint dir: %v", err))
		return
	}

	// Default excludes
	defaults := ingestion.DefaultConfig()

	config := ingestion.Config{
		ProjectID: projectID,
		RepoSource: ingestion.RepoSource{
			Type:  "local_path",
			Value: repoPath,
		},
		IngestionConfig: ingestion.IngestionConfig{
			ParserMode:           ingestion.ParserModeAuto,
			EmbeddingProvider:    "ollama",
			EmbeddingDimensions:  768, // nomic-embed-text default
			BatchTargetMutations: 500,
			MaxFileSizeBytes:     1048576,
			CheckpointPath:       checkpointDir,
			ExcludeGlobs:         defaults.ExcludeGlobs,
			LocalDataDir:         dbPath, // Use full path including project ID
			LocalEngine:          "rocksdb",
			Concurrency: ingestion.ConcurrencyConfig{
				ParseWorkers: 4,
				EmbedWorkers: 8,
			},
		},
	}

	pipeline, err := ingestion.NewLocalPipeline(config, logger)
	if err != nil {
		s.updateJobError(job, fmt.Sprintf("failed to create pipeline: %v", err))
		return
	}
	defer func() { _ = pipeline.Close() }()

	// Set up progress reporting
	pipeline.SetProgressCallback(func(current, total int64, phase string) {
		s.jobsMu.Lock()
		job.Phase = phase
		job.Progress = &progress{Current: current, Total: total}
		s.jobsMu.Unlock()
	})

	// Run indexing
	ctx := context.Background()
	result, err := pipeline.Run(ctx)
	if err != nil {
		s.updateJobError(job, fmt.Sprintf("indexing failed: %v", err))
		return
	}

	// Update job with result
	now := time.Now()
	s.jobsMu.Lock()
	job.Status = "completed"
	job.Phase = "done"
	job.EndedAt = &now
	job.Result = &indexResult{
		FilesProcessed:     result.FilesProcessed,
		FunctionsExtracted: result.FunctionsExtracted,
		TypesExtracted:     result.TypesExtracted,
		Duration:           result.TotalDuration.String(),
	}
	s.jobsMu.Unlock()

	// Close pipeline before reopening DB (to release the lock)
	_ = pipeline.Close()

	// Reopen database
	s.dbMu.Lock()
	db, err := cozo.New("rocksdb", dbPath, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to reopen database after indexing: %v", err)
	} else {
		s.db = db
		s.hasDB = true
		log.Printf("[INFO] Database reopened successfully")
	}
	s.dbMu.Unlock()
}

func (s *cieServer) updateJobError(job *indexJob, errMsg string) {
	now := time.Now()
	s.jobsMu.Lock()
	job.Status = "failed"
	job.Error = errMsg
	job.EndedAt = &now
	s.jobsMu.Unlock()
	log.Printf("[ERROR] Index job %s failed: %s", job.ID, errMsg)
}

func (s *cieServer) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path: /v1/index/{job_id}/status or /v1/index/{job_id}
	path := strings.TrimPrefix(r.URL.Path, "/v1/index/")
	path = strings.TrimSuffix(path, "/status")
	jobID := path

	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}

	s.jobsMu.RLock()
	job, ok := s.jobs[jobID]
	s.jobsMu.RUnlock()

	if !ok {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(job)
}

func (s *cieServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.dbMu.RLock()
	hasDB := s.hasDB
	s.dbMu.RUnlock()

	status := map[string]any{
		"project_id": s.projectID,
		"indexed":    hasDB,
		"data_dir":   s.dataDir,
		"repo_path":  s.repoPath,
	}

	if hasDB {
		// Query counts
		fileCount := s.queryCount("?[count(id)] := *cie_file{id}")
		funcCount := s.queryCount("?[count(id)] := *cie_function{id}")
		typeCount := s.queryCount("?[count(id)] := *cie_type{id}")

		status["files"] = fileCount
		status["functions"] = funcCount
		status["types"] = typeCount
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (s *cieServer) queryCount(script string) int {
	s.dbMu.RLock()
	defer s.dbMu.RUnlock()
	result, err := s.db.Run(script, nil)
	if err != nil {
		return 0
	}
	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		if count, ok := result.Rows[0][0].(float64); ok {
			return int(count)
		}
		if count, ok := result.Rows[0][0].(int64); ok {
			return int(count)
		}
	}
	return 0
}

func printServeUsage() {
	fmt.Println(`Usage: cie serve [options]

Description:
  Start a local HTTP server that exposes the CIE API.
  This enables MCP tools and remote clients to use CIE.

Options:
  -p, --port <port>        Port to listen on (default: 8080, or CIE_SERVE_PORT)
  --project-id <id>        Project ID (default: from .cie/project.yaml or CIE_PROJECT_ID)
  --repo-path <path>       Repository path to index (default: /repo or CIE_REPO_PATH)
  -h, --help               Show this help message

Environment Variables:
  CIE_SERVE_PORT           Port to listen on (default: 8080)
  CIE_PROJECT_ID           Project identifier
  CIE_DATA_DIR             Data directory (default: ~/.cie/data)
  CIE_REPO_PATH            Repository path to index (default: /repo)
  OLLAMA_HOST              Ollama URL for embeddings
  OLLAMA_EMBED_MODEL       Embedding model name

API Endpoints:
  GET  /health             Health check
  POST /v1/init            Initialize project configuration
  POST /v1/index           Start indexing (async, returns job_id)
  GET  /v1/index/{id}      Get indexing job status
  GET  /v1/status          Get project status (file/function counts)
  POST /v1/query           Execute CozoScript query
  POST /v1/ensure-mounted  No-op for local (always ready)

Examples:
  # Start server with default settings
  cie serve

  # Start on a specific port with project ID
  cie serve --port 9090 --project-id myproject

  # Use with Docker
  docker run -p 8080:8080 -v /code:/repo:ro cie serve --project-id myproject

  # Use with MCP tools
  export CIE_BASE_URL=http://localhost:8080
  cie --mcp`)
}
