# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Email: **byrankon18@gmail.com**

Include in your report:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

You will receive an acknowledgement within 48 hours. We aim to release a patch within 14 days of confirmation.

## Security Model

Helix Gateway is designed for deployment behind a load balancer or inside a private network. Security considerations:

- **Admin API** (default `:9090`) must never be exposed to the public internet. Protect with `HELIX_ADMIN_USER` / `HELIX_ADMIN_PASSWORD` env vars and network-level controls.
- **JWT secret** is provided via `JWT_SECRET` env var — never commit it to version control.
- **API keys** are stored as SHA-256 hashes; plaintext is returned only on creation and never persisted.
- **IP policies** operate on `X-Forwarded-For` — ensure your ingress/load balancer sets this header correctly and strips untrusted values from clients.
- **TLS** is handled via ACME/Let's Encrypt when `HELIX_TLS_DOMAINS` is set. Ensure the cache directory (`HELIX_TLS_CACHE_DIR`) has appropriate permissions.

## Dependency Updates

Dependencies are kept up to date. Run `go list -m -u all` to check for updates.
