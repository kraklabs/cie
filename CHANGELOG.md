# Changelog

All notable changes to CIE (Code Intelligence Engine) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

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

[unreleased]: https://github.com/kraklabs/cie/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/kraklabs/cie/releases/tag/v0.1.0
