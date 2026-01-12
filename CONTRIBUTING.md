# Contributing to go-openexr

Thank you for your interest in contributing to go-openexr! This document provides guidelines and information for contributors.

## Code of Conduct

This project follows the [Linux Foundation Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

### Prerequisites

| Requirement | Version            |
| ----------- | ------------------ |
| Go          | 1.21 or later      |
| Git         | Any recent version |

### Setting Up Your Development Environment

```bash
# Fork and clone the repository
git clone https://github.com/YOUR_USERNAME/go-openexr.git
cd go-openexr

# Verify tests pass
go test ./...

# Run benchmarks
go test -bench=. ./...
```

### Test Images

For testing with real EXR files, download test images from the [openexr-images](https://github.com/AcademySoftwareFoundation/openexr-images) repository and place them in `testdata/`.

## Making Changes

### Branching Strategy

1. Create a feature branch from `main`:

   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes with clear, focused commits

3. Ensure all tests pass:

   ```bash
   go test ./...
   go test -race ./...  # Run with race detector
   ```

4. Push and create a pull request

### Coding Standards

- Follow standard Go conventions and idioms
- Run `gofmt` on all code
- Add tests for new functionality
- Update documentation for API changes
- Prefer short, clear functions over deeply nested code
- Use meaningful variable and function names

### Commit Messages

- Use clear, descriptive commit messages
- Start with a brief summary (50 chars or less)
- Include details in the body if needed
- Reference issues when applicable

Example:

```
Add support for deep tiled images

Implements reading and writing of deep tiled EXR files as specified
in the OpenEXR 2.0 format.

Fixes #123
```

### Testing

- Write tests for all new functionality
- Ensure tests cover both success and error paths
- Use table-driven tests where appropriate
- Run tests with race detection before submitting

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

## Pull Request Process

1. Update documentation for any API changes
2. Add or update tests as needed
3. Ensure all tests pass, including race detection
4. Update the README.md if needed
5. Request review from maintainers

### What We Look For

| Criteria      | Description                                    |
| ------------- | ---------------------------------------------- |
| Correctness   | Code works as intended with good test coverage |
| Go idioms     | Follows standard Go conventions and patterns   |
| Documentation | Clear comments and godoc for exported APIs     |
| Performance   | Considers allocations and hot paths            |
| Compatibility | Maintains backward compatibility               |

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

| Information        | Example                                     |
| ------------------ | ------------------------------------------- |
| Go version         | Output of `go version`                      |
| OS / Architecture  | macOS 14 / arm64, Ubuntu 22.04 / amd64      |
| Steps to reproduce | Minimal code or commands                    |
| Expected behavior  | What should happen                          |
| Actual behavior    | What actually happens                       |
| Sample file        | Attach EXR file or describe how to generate |

### Feature Requests

For feature requests, please describe:

| Information          | Description                            |
| -------------------- | -------------------------------------- |
| Use case             | Why you need this feature              |
| Relation to existing | How it fits with current functionality |
| Implementation ideas | Any suggestions for how to implement   |

## Documentation

- Add godoc comments to all exported functions, types, and constants
- Include examples in documentation where helpful
- Update the README for significant changes

## Performance

This library aims to be performant for production use. When making changes:

- Consider memory allocations
- Use benchmarks to measure performance impact
- Profile code for hot paths
- Consider concurrent access patterns

## Questions?

Feel free to open an issue for questions about contributing.

## License

By contributing to go-openexr, you agree that your contributions will be licensed under the Apache License 2.0.
