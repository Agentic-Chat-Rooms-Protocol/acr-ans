# Contributing to Route ANS

Thank you for your interest in contributing to Route ANS! This document provides guidelines and instructions for contributing.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/route-ans.git
   cd route-ans
   ```
3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/route-ans/route-ans.git
   ```

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Make
- Docker (optional, for integration tests)

### Install Dependencies

```bash
make deps
```

### Running Locally

```bash
# Run with default config
make run

# Run with custom config
go run ./cmd/resolver --config configs/resolver.yaml
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint
```

## Code Guidelines

### Go Style

- Follow standard Go formatting (`gofmt`, `goimports`)
- Run `make fmt` before committing
- Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- Use meaningful variable and function names

### Code Organization

- Keep packages focused and cohesive
- Use interfaces for extensibility (cache, registry, trust)
- Write unit tests for new functionality
- Add integration tests for API endpoints

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types:**
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code style changes (formatting)
- `refactor:` Code refactoring
- `test:` Adding/updating tests
- `chore:` Maintenance tasks

**Example:**
```
feat(resolver): add version negotiation support

Implement semantic version range matching for ANS resolution.
Supports caret (^), tilde (~), and wildcard (x) ranges.

Closes #123
```

## Pull Request Process

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**:
   - Write code following our guidelines
   - Add tests for new functionality
   - Update documentation as needed

3. **Run tests and linter**:
   ```bash
   make lint test
   ```

4. **Commit your changes**:
   ```bash
   git add .
   git commit -m "feat(component): description"
   ```

5. **Push to your fork**:
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request** on GitHub

### PR Requirements

- [ ] All tests pass
- [ ] Code is properly formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventions
- [ ] PR description explains the change

## Documentation

### Updating Documentation

Documentation is in the `docs/` directory using MkDocs:

```bash
# Install doc dependencies
make docs-install

# Serve docs locally
make docs-serve

# Build docs
make docs-build
```

### Documentation Guidelines

- Keep architecture docs high-level (no code snippets)
- Use Mermaid diagrams for visualizations
- Update API docs via Swagger annotations in code
- Regenerate API docs: `make api-docs`

## Testing

### Unit Tests

```bash
# Run unit tests
go test ./...

# Run specific package
go test ./internal/resolver/...
```

### Integration Tests

```bash
# Start dependencies
docker-compose up -d redis

# Run integration tests
go test -tags=integration ./...
```

### Test Coverage

- Aim for >80% coverage on new code
- Write table-driven tests for multiple cases
- Mock external dependencies (registry, cache)

## Adding New Features

### Registry Adapters

Implement the `registry.Registry` interface:

```go
type Registry interface {
    Lookup(ctx context.Context, ansName string) (*Record, error)
    LookupByFQDN(ctx context.Context, fqdn string) ([]*Record, error)
}
```

### Cache Providers

Implement the `cache.Cache` interface:

```go
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}
```

### Trust Verifiers

Implement the `trust.Verifier` interface for custom verification logic.

## Issue Reporting

### Bug Reports

Include:
- Route ANS version
- Go version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs/errors

### Feature Requests

Include:
- Use case description
- Proposed solution
- Alternative solutions considered
- Impact on existing functionality

## Code Review

All contributions go through code review. Reviewers will check:

- Code quality and style
- Test coverage
- Documentation completeness
- Backward compatibility
- Performance implications

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

## Questions?

- Open an issue for discussion
- Check existing issues and PRs
- Review documentation at https://route-ans.github.io/route-ans

Thank you for contributing!
