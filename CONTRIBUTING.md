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
make hooks   # installs the pre-commit guard described below
```

Requires Go 1.26+.

### What belongs in the repository

This is a public repository. Only the product ships here: source, tests,
user documentation, release tooling. Working material does not: planning
notes, specs, prompt files, drafts, reports, or editor/agent configuration.
`scripts/check-internal-files.sh` enforces this as a pre-commit hook
(`make hooks`) and in CI, so a stray note fails the build instead of landing
in every fork's history. If it flags a file that is genuinely public, add it
to the allow-list in that script with a one-line reason.

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
