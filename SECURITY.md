# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |

## Reporting a Vulnerability

If you discover a security vulnerability in cmux-resurrect, please report it responsibly:

1. **Email**: forge@drolosoft.com
2. **Subject**: `[SECURITY] cmux-resurrect — <brief description>`

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

**Do not** open a public GitHub issue for security vulnerabilities.

## Security Considerations

- **Local by default**: crex operates on the local filesystem. Network access is limited to `crex update`, which queries the GitHub API and downloads releases (honoring `GITHUB_TOKEN` if set); every other command is local-only.
- **No credentials stored**: Layout files contain workspace names, directory paths, split configurations, pane commands, and AI-session resume IDs. No passwords, API keys, or tokens.
- **File permissions**: Saved layouts are written with mode `0600` under `~/.config/crex/`.
