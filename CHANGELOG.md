# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0]

### Added

- `lint` command (also the default) with 18 rules: V001-V004 (vagueness), S001-S004 (scoping and load cost), F001-F006 (format validity), C001-C003 (consistency), E001 (enforceability).
- Discovery for Claude Code, AGENTS.md, Cursor, Copilot, Windsurf, Cline, Roo and Gemini rules files, `@path` imports (one level) and `.gitignore`.
- Output formats: text, json, sarif, github. Documented exit codes.
- `.kogentic-rules-lint.yaml` config and inline `rules-lint-disable` comments.
- `explain`, `init`, `version`.
- `harden` with templates for path deny-list, command deny-list, secret-file deny, must-have-test and run-before-commit, as Claude Code hooks, Cursor hooks, rule files or CI test stubs. Prints a diff; writes only with `--write`.
- Opt-in Kogentic connector: `login`, `logout`, `whoami`, `lint --send-to-kogentic` (off by default, no telemetry, configurable endpoint, local mock server for testing).
- Release builds for macOS (arm64, amd64), Linux (amd64, arm64) and Windows (amd64) with SHA-256 checksums, published as GitHub Release assets. Binaries are unsigned in this release.
