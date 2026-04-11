# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.0.x   | Yes       |

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

- **Local only**: crex operates entirely on the local filesystem. It does not make network requests or transmit data.
- **No credentials stored**: Layout files contain only workspace names, directory paths, and split configurations. No passwords, API keys, or tokens.
- **File permissions**: Saved layouts inherit the user's default file permissions under `~/.config/crex/`.
