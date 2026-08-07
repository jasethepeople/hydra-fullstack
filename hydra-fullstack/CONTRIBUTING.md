# Contributing to HYDRA Fullstack

Thank you for your interest in contributing to HYDRA Fullstack! This document provides guidelines for contributing to the project.

## Code of Conduct

This project and everyone participating in it is governed by our commitment to:
- Be respectful and inclusive
- Welcome newcomers and help them learn
- Focus on constructive feedback
- Respect differing viewpoints and experiences

## How to Contribute

### Reporting Bugs

Before creating a bug report, please:
1. Check the existing issues to avoid duplicates
2. Use the latest version of the code
3. Collect information about the bug (logs, environment, steps to reproduce)

**Bug report template:**
```markdown
**Description:**
Clear description of the bug

**Steps to Reproduce:**
1. Step one
2. Step two
3. Step three

**Expected Behavior:**
What you expected to happen

**Actual Behavior:**
What actually happened

**Environment:**
- OS: [e.g., macOS 14.0]
- Go Version: [e.g., 1.22.0]
- Deployment: [e.g., Fly.io, Docker, local]

**Logs:**
```
Paste relevant logs here
```
```

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. Please:
1. Use a clear title
2. Provide detailed description
3. Explain why this enhancement would be useful

### Pull Requests

1. Fork the repository
2. Create a branch (`git checkout -b feature/your-feature`)
3. Make your changes
4. Run tests (`make test`)
5. Format code (`make fmt`)
6. Commit with clear messages
7. Push and create a Pull Request

**Commit message format:**
```
type(scope): subject

body (optional)

footer (optional)
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Example:
```
feat(alerts): add PagerDuty integration

Implements PagerDuty v2 Events API for critical alerts.
Includes circuit breaker protection and retry logic.

Closes #123
```

## Development Setup

```bash
# Clone your fork
git clone https://github.com/yourusername/hydra-fullstack.git
cd hydra-fullstack

# Install dependencies
go mod download

# Run tests
make test

# Start development server
make dev
```

## Project Structure Guidelines

- `internal/` — Private application code (not importable by external packages)
- `cmd/` — Application entry points
- `web/` — Frontend assets
- `proto/` — Protocol Buffer definitions
- `scripts/` — Deployment and utility scripts

## Testing

All new features must include tests:

```go
func TestNewFeature(t *testing.T) {
    // Arrange
    input := "test"

    // Act
    result := NewFeature(input)

    // Assert
    assert.Equal(t, expected, result)
}
```

Run tests before submitting:
```bash
make ci  # Runs fmt, lint, test, build
```

## Documentation

- Update README.md if adding new features
- Add comments to exported functions and types
- Update CHANGELOG.md for user-facing changes

## Questions?

Feel free to open an issue with the `question` label.

Thank you for contributing!
