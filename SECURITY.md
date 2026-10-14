# Security

## Reporting a vulnerability

Please use [GitHub private vulnerability reporting](https://github.com/Johnny008-creator/offcode/security/advisories/new).
Include affected versions, reproduction steps, impact and a minimal example.
Do not post credentials, private conversations or exploit details in public
issues. There is no guaranteed response time; this is an experimental project.
Security fixes target the latest development branch and subsequent releases;
older releases do not receive a separate backport guarantee.

## Trust boundaries

Offcode runs tools with your operating-system permissions. Permission modes
are confirmation controls, not a sandbox. In `auto` mode a model can execute
commands and modify files. Use a disposable checkout or an OS sandbox for
untrusted tasks, review commands, and keep backups. Read-only mode still allows
reading local data and network requests; it is not a confidentiality boundary.
Repository content, fetched pages and model output can contain prompt injection.
Project hooks and configured MCP servers execute local programs: review their
configuration before using an unfamiliar project.

The web UI binds only to loopback IP addresses. Host and origin checks reject
foreign browser requests and DNS rebinding hosts. It has no user authentication
or multi-user isolation. Do not expose it through a public reverse proxy or
port forward. Other local processes can access it. Generated HTML previews run
in sandboxed frames; do not treat arbitrary generated files as trusted software.

See [docs/PRIVACY.md](docs/PRIVACY.md) for local storage and network behavior.
Release checksums detect mismatched downloads; they are published alongside
the binaries and are not independent publisher signatures.
