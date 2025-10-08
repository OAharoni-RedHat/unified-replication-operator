# Contributing to Unified Replication Wrapper Operator

Thank you for your interest in contributing to the Unified Replication Wrapper Operator! This document provides guidelines and information for contributors.

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Kubernetes 1.20 or later
- Docker for building images
- kubectl for cluster interactions

### Development Environment Setup

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/your-username/unified-replication-operator.git
   cd unified-replication-operator
   ```

2. Install development dependencies:
   ```bash
   make install
   ```

3. Install pre-commit hooks:
   ```bash
   pip install pre-commit
   pre-commit install
   ```

4. Verify your setup:
   ```bash
   make smoke-test
   ```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Changes

Follow these guidelines:
- Write tests first (TDD approach)
- Keep changes focused and atomic
- Follow Go best practices
- Update documentation as needed

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run specific test types
make test-unit
make test-integration
make lint
make security-scan
```

### 4. Commit Changes

Use conventional commit messages:
```bash
git commit -m "feat: add new backend adapter for storage system X"
git commit -m "fix: resolve state transition bug in Ceph adapter"
git commit -m "docs: update API reference for new fields"
```

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Run `golangci-lint` and fix all issues
- Write comprehensive tests (>80% coverage)
- Use structured logging with consistent fields

### Testing Standards

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test component interactions
- **End-to-End Tests**: Test complete workflows
- **Benchmark Tests**: For performance-critical code

Example test structure:
```go
func TestTranslationEngine_TranslateState(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"valid source state", "source", "primary", false},
        {"invalid state", "invalid", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := TranslateState(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Documentation Standards

- Update README.md for user-facing changes
- Add docstrings for all public functions
- Update API documentation for CRD changes
- Include examples in documentation

## Pull Request Process

1. **PR Title**: Use conventional commit format
2. **Description**: Explain what and why, not just what
3. **Testing**: Describe how you tested the changes
4. **Breaking Changes**: Clearly mark any breaking changes
5. **Reviews**: Address all reviewer feedback
6. **CI/CD**: Ensure all checks pass

### PR Template

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change)
- [ ] New feature (non-breaking change)
- [ ] Breaking change (fix or feature causing existing functionality to change)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed
- [ ] Performance impact assessed

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Comments added to complex code
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] No breaking changes (or clearly documented)
```

## Architecture Guidelines

### Adding New Backend Adapters

1. Implement the `BackendAdapter` interface
2. Add discovery logic for the new backend
3. Create translation mappings
4. Write comprehensive tests
5. Update documentation

### Modifying Core Components

Core components (discovery, translation, controller) require:
- Design discussion in GitHub issues first
- Comprehensive test coverage
- Performance impact assessment
- Backward compatibility consideration

## Release Process

### Semantic Versioning

We follow [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

### Release Checklist

- [ ] All tests pass
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version bumped in appropriate files
- [ ] Tag created and pushed
- [ ] Release notes written

### Common Development Tasks

#### Adding a New Backend Adapter

1. Create adapter file in `pkg/adapters/`
2. Implement `BackendAdapter` interface
3. Add to adapter registry
4. Create translation mappings
5. Write tests
6. Update documentation

#### Modifying the API

1. Update types in `api/v1alpha1/`
2. Run `make manifests generate`
3. Update validation logic
4. Update documentation
5. Add migration logic if needed

#### Adding New Tests

1. Unit tests go in `*_test.go` files alongside code
2. Integration tests go in `test/integration/`
3. End-to-end tests go in `test/e2e/`
4. Use testify for assertions

## Security

### Security Guidelines

- Never commit secrets or credentials
- Use RBAC with minimal required permissions
- Validate all inputs
- Follow secure coding practices

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
