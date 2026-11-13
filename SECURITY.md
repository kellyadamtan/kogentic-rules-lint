# Security policy

## Reporting a vulnerability

Please **do not open a public issue** for security problems. Report them privately with GitHub's
[private vulnerability reporting](https://github.com/kellyadamtan/kogentic-rules-lint/security/advisories/new).
We aim to acknowledge reports within a few working days.

Useful details: the version (`kogentic-rules-lint version`), your OS, and steps or a minimal repo to reproduce.

## Scope notes

- The linter reads files and writes only when you run `init` or `harden --write`.
- `harden` generates shell scripts from your correction text. Review generated hooks before committing them.
- The optional connector sends data only with `--send-to-kogentic`, `login` or `whoami`, refuses plain `http://` except for localhost, and stores the API key in the OS keychain.
- v0.1 release binaries are not code-signed for macOS or Windows. Always verify downloads against `checksums.txt`, and check build provenance with `gh attestation verify <file> --repo kellyadamtan/kogentic-rules-lint`. Each archive ships with an SPDX SBOM.
