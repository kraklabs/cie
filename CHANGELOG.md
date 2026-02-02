# Changelog

All notable changes to CIE (Code Intelligence Engine) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.0] - 2026-02-01

### Added
- New git history MCP tools (v1.5.0):
  - `cie_function_history`: Get git commit history for a specific function using line-based tracking.
  - `cie_find_introduction`: Find the commit that first introduced a code pattern (git pickaxe).
  - `cie_blame_function`: Get aggregated blame analysis showing code ownership percentages.
- `GitExecutor` helper in `pkg/tools/git.go` for safe git command execution with timeout support.
- Git repository auto-discovery from config file location.

### Changed
- MCP server version bumped to 1.5.0.

## [0.4.7] - 2026-01-23

### Fixed
- Fixed Docker image tags to include `v` prefix (e.g., `v0.4.7`) matching CLI version.

## [0.4.6] - 2026-01-23

### Changed
- `cie start` now pulls latest Docker images before starting containers.
- Docker image tag now matches CLI version (e.g., `v0.4.6` instead of `latest`).
- Setup container now uses `docker compose run --rm` for cleaner execution.

### Fixed
- Fixed `cie start` hanging on first run when downloading embedding model.

## [0.4.5] - 2026-01-23

### Fixed
- Added QEMU setup to Docker build workflow for proper ARM64 CGO compilation with tree-sitter.

## [0.4.4] - 2026-01-23

### Fixed
- Fixed tree-sitter parser not being used on ARM64 Linux (Docker on Apple Silicon). Previously always fell back to simplified parser which extracts ~40% fewer functions. Now tries tree-sitter first.

## [0.4.3] - 2026-01-23

### Fixed
- Fixed Docker container unable to access project files for indexing (missing `/repo` volume mount in embedded docker-compose.yml).
- `cie init` now defaults `edge_cache` to `http://localhost:9090` (Docker mode) instead of leaving it empty.

## [0.4.2] - 2026-01-23

### Added
- Incremental indexing: only process changed files since last indexed commit.
- Uses `git diff --name-status` to detect added, modified, deleted, and renamed files.
- Added `cie_project_meta` table for storing project metadata (last indexed SHA).
- New methods in EmbeddedBackend: `GetProjectMeta`, `SetProjectMeta`, `GetLastIndexedSHA`, `SetLastIndexedSHA`, `DeleteEntitiesForFile`.
- Integration tests for incremental indexing.

### Changed
- Default indexing behavior is now incremental when possible (git repo with previous index).
- Use `ForceReindex: true` in config to force full re-indexing.

### Fixed
- Fixed `cie index` running locally instead of using Docker server when `cie init` was run before `cie start`.
- `cie index` now auto-detects running CIE server at `localhost:9090` even if `edge_cache` is not set in config.
- `detectDockerCompose` now checks `~/.cie/docker-compose.yml` in addition to the project directory.

### Documentation
- Clarified that `cie serve --port 9090` can be used to serve local data on the same port as Docker, enabling MCP tools to work with locally indexed data without reconfiguration.

## [0.4.1] - 2026-01-23

### Fixed
- Fixed Docker container not reading project config by mounting `.cie/` directory.
- Pass `CIE_PROJECT_ID` and `CIE_PROJECT_DIR` environment variables to Docker Compose.
- Check `CIE_CONFIG_PATH` environment variable in config loader for Docker support.

## [0.4.0] - 2026-01-23

### Added
- Docker image published to GitHub Container Registry (`ghcr.io/kraklabs/cie`).
- Multi-platform Docker images (linux/amd64, linux/arm64) built with Docker Buildx.
- Embedded `docker-compose.yml` in CIE binary - no need to clone repository.
- `cie start` now extracts docker-compose to `~/.cie/` automatically.

### Changed
- Simplified installation: `brew install` + `cie start` works without cloning repo.
- Docker Compose now uses published `ghcr.io/kraklabs/cie:latest` image.
- Updated README Quick Start to a 2-step process.
- Added `brew tap kraklabs/cie` to all documentation.

### Fixed
- Fixed `cie start` failing when docker-compose.yml not found in current directory.

## [0.3.1] - 2026-01-23

### Fixed
- Fixed `cie init` not setting `edge_cache` URL when Docker server is starting but not yet responding.
- Auto-detect `docker-compose.yml` with CIE server configuration and default to `http://localhost:9090`.

## [0.3.0] - 2026-01-23

### Added
- New `cie start` command to automate Docker infrastructure setup and model pulling.
- New `cie stop` command to stop Docker containers (preserves data).
- New `cie reset` command to delete indexed data with `--docker` flag for full cleanup.
- Automatic detection of Docker-based CIE server during `cie init` and command execution.
- Persistence of server URL in `.cie/project.yaml` via `edge_cache` field.
- Static linking for CozoDB library in `Makefile` for better portability on macOS/Linux.
- Thread-safety fixes for parallel indexing (Tree-sitter parser pool and CallResolver locks).
- Homebrew tap support: `brew install kraklabs/cie`.
- GitHub Actions release workflow with multi-platform binaries (linux-amd64, linux-arm64, darwin-arm64).

### Changed
- Improved `cie init` flow to suggest `cie start` as the next step.
- Simplified `README.md` and documentation with the new `init` -> `start` -> `index` workflow.
- Updated `install.sh` with the new workflow instructions.
- Client-server architecture: CLI delegates to Docker-based CIE server for heavy operations.

### Fixed
- Fixed critical `SIGSEGV` and `semawakeup` crashes on macOS/Darwin when running indexing.
- Fixed `concurrent map read and map write` panics during large codebase indexing.
- Fixed volume permission issues in `docker-compose.yml`.

## [0.1.0] - 2026-01-XX

Initial open source release of CIE (Code Intelligence Engine).

### Added

- CLI with 20+ MCP tools for code intelligence
- Tree-sitter parsing for Go, Python, TypeScript, JavaScript, and Protocol Buffers
- Semantic search with embedding support (OpenAI, Ollama, Nomic)
- Call graph analysis and function tracing
- CozoDB-based Datalog code graph storage
- HTTP endpoint discovery for Go frameworks (Gin, Echo, Chi, Fiber)
- gRPC service and RPC method detection from .proto files
- Interface implementation finder
- Directory and file summary tools
- Security audit tools (pattern verification, absence checking)
- Shell completion for bash, zsh, and fish
- JSON output mode for scripting (`--json` flag)
- Verbose mode for debugging (`-v`, `-vv`)
- Quiet mode for scripts (`-q`)
- Semantic exit codes (0-10) for error handling
- Comprehensive documentation:
  - Getting started guide
  - Configuration reference
  - Tools reference with examples
  - Architecture overview
  - MCP integration guides for Claude Code and Cursor
  - Troubleshooting guide
- Docker image with multi-stage build (<100MB)
- docker-compose.yml for local development with Ollama
- GitHub Actions CI/CD workflows (test, lint, build, release)
- Goreleaser configuration for multi-platform binaries

### Changed

- Error messages now include context and fix suggestions
- CLI help text improved with usage examples
- Output formatting optimized for terminal readability

### Security

- gosec security scanning integrated
- gitleaks secret detection configured
- Security policy documented in SECURITY.md
- No hardcoded credentials in codebase
- All API keys via environment variables only

[unreleased]: https://github.com/kraklabs/cie/compare/v0.4.7...HEAD
[0.4.7]: https://github.com/kraklabs/cie/compare/v0.4.6...v0.4.7
[0.4.6]: https://github.com/kraklabs/cie/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/kraklabs/cie/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/kraklabs/cie/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/kraklabs/cie/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/kraklabs/cie/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/kraklabs/cie/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/kraklabs/cie/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/kraklabs/cie/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/kraklabs/cie/compare/v0.1.0...v0.3.0
[0.1.0]: https://github.com/kraklabs/cie/releases/tag/v0.1.0
