# Contributing to cmux-resurrect

Contributions are welcome — bug fixes, new workspace templates, feature ideas, and documentation improvements.

## Getting Started

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Run tests: `make test`
5. Run linting: `make lint`
6. Commit your changes
7. Open a pull request

## Development Setup

```bash
git clone https://github.com/YOUR-USERNAME/cmux-resurrect.git
cd cmux-resurrect
make build
make test
```

Requires Go 1.26+.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Run `golangci-lint run` before submitting
- Write tests for new functionality
- Keep commits focused and descriptive

## Testing

```bash
make test              # unit tests
make test-integration  # integration tests (requires cmux or Ghostty)
```

Test fixtures live in `testdata/`. Add new fixtures there when testing new layout formats.

## Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if behavior changes
- Reference any related issues

## Bug Reports

Use [GitHub Issues](https://github.com/drolosoft/cmux-resurrect/issues) with:
- Steps to reproduce
- Expected vs actual behavior
- crex version (`crex version`)
- OS, terminal backend (cmux or Ghostty), and its version
