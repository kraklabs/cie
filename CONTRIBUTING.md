# Contributing to CIE

Thank you for your interest in contributing to CIE! This document provides guidelines and information for contributors.

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all experience levels.

## Getting Started

### Prerequisites

- Go 1.24 or later
- CozoDB C library (libcozo_c)
- Git

### Setting Up Development Environment

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/cie.git
   cd cie
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Build the project:
   ```bash
   go build ./...
   ```

5. Run tests:
   ```bash
   go test ./...
   ```

### Installing CozoDB

CIE requires the CozoDB C library. Follow the [CozoDB installation guide](https://github.com/cozodb/cozo) for your platform.

For macOS with Homebrew:
```bash
# Download prebuilt library from CozoDB releases
# Set CGO_LDFLAGS and CGO_CFLAGS to point to the library
```

## Development Workflow

### Branching

- Create a feature branch from `main`:
  ```bash
  git checkout -b feature/your-feature-name
  ```

### Making Changes

1. Make your changes
2. Add tests for new functionality
3. Run the test suite:
   ```bash
   go test ./...
   ```

4. Run the linter:
   ```bash
   make lint
   ```

5. Format your code:
   ```bash
   make fmt
   ```

### Commit Messages

Use clear, descriptive commit messages:

```
type(scope): short description

Longer description if needed.
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

Example:
```
feat(tools): add cie_find_implementations tool

Adds a new MCP tool that finds types implementing a given interface.
Supports Go structs, TypeScript classes, and Python classes.
```

### Pull Requests

1. Push your branch to your fork
2. Create a Pull Request against `main`
3. Fill out the PR template with:
   - Description of changes
   - Related issues
   - Testing done

### Code Review

- Address feedback promptly
- Keep discussions constructive
- Squash commits if requested

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/tools/...

# Run tests with CozoDB (requires libcozo_c)
go test -tags=cozodb ./...
```

### Writing Tests

- Place tests in `*_test.go` files
- Use table-driven tests where appropriate
- Mock external dependencies
- Test both success and error cases

## Architecture Guidelines

### Package Structure

- `cmd/`: Executable commands
- `internal/`: Private packages
- `pkg/`: Public packages (if any)

### Code Style

- Follow standard Go conventions
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and small

### Error Handling

- Return errors instead of panicking
- Wrap errors with context
- Use structured logging

## Documentation

- Update README.md for user-facing changes
- Add godoc comments to exported functions
- Update CLI help text when adding commands

## Releasing

Releases are managed by maintainers. If you need a release:

1. Ensure all tests pass
2. Update version numbers if needed
3. Create a GitHub release with changelog

## Getting Help

- Open an issue for bugs or feature requests
- Use discussions for questions
- Check existing issues before creating new ones

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
