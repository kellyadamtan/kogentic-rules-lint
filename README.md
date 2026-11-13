<p align="center">
  <img src="assets/kogentic-icon-512.png" alt="Kogentic" width="96" height="96">
</p>

<h1 align="center">kogentic-rules-lint</h1>

<p align="center">
  Free, offline linter for coding-agent rule files: CLAUDE.md, AGENTS.md, .cursor/rules, Copilot, Windsurf, Cline, Roo and Gemini.<br>
  By <a href="https://github.com/kellyadamtan">Kogentic</a>.
</p>

<p align="center">
  <a href="https://github.com/kellyadamtan/kogentic-rules-lint/actions/workflows/ci.yml"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/kellyadamtan/kogentic-rules-lint/ci.yml?branch=main&label=ci&labelColor=c6613f&color=black"></a>
  <a href="https://github.com/kellyadamtan/kogentic-rules-lint/releases"><img alt="Release" src="https://img.shields.io/github/v/release/kellyadamtan/kogentic-rules-lint?labelColor=c6613f&color=black"></a>
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/license-MIT-black?labelColor=c6613f"></a>
</p>

One command tells you why your agent keeps ignoring or misreading your rules. It finds **vague**, **mis-scoped**, **duplicate** and **contradictory** instructions across every agent's rules files in the repo, and it turns a mistake you keep correcting into an **enforced hook or test**.

- A single static binary. No account, no network, **no telemetry**.
- 18 deterministic rules with stable IDs. No LLM.
- Text, JSON, SARIF (GitHub code scanning) and GitHub annotation output.

```console
$ kogentic-rules-lint
CLAUDE.md:5: warning V001 vague-instruction: "Write clean code." has no checkable criterion.
    fix: Say what to do and how to check it: "When <situation>, do <action>; verify by <check>".
CLAUDE.md:6: error C003 drift-between-tools: CLAUDE.md and AGENTS.md:3 disagree on indentation (2 spaces vs tabs), so different agents get different rules.
    fix: Keep one source of truth (for example AGENTS.md) and import it from CLAUDE.md with `@AGENTS.md`, or symlink one to the other.
CLAUDE.md:15: warning S001 always-on-should-auto-attach: Section "Migrations" is loaded every session but only refers to migrations (3 references).
    fix: Move this section to .claude/rules/migrations.md with frontmatter `paths: ["migrations/**"]`.
...
7 errors, 14 warnings, 5 info in 11 rules files (always-on: ~242 tokens)
```

Try it on the sample repo in [`examples/messy-repo`](examples/messy-repo).

## Install

Download a binary from [GitHub Releases](https://github.com/kellyadamtan/kogentic-rules-lint/releases). Each release has archives for macOS (Apple Silicon and Intel), Linux (x86-64 and arm64) and Windows (x86-64), plus `checksums.txt` (SHA-256).

macOS / Linux:

```sh
VERSION=0.1.0
OS=linux        # or darwin
ARCH=amd64      # or arm64
BASE=https://github.com/kellyadamtan/kogentic-rules-lint/releases/download/v$VERSION
curl -fsSLO "$BASE/kogentic-rules-lint_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSLO "$BASE/checksums.txt"

# Verify the download before you run it
sha256sum --ignore-missing -c checksums.txt          # Linux
shasum -a 256 --ignore-missing -c checksums.txt      # macOS

tar -xzf "kogentic-rules-lint_${VERSION}_${OS}_${ARCH}.tar.gz" kogentic-rules-lint
sudo mv kogentic-rules-lint /usr/local/bin/          # or anywhere on your PATH
```

Windows (PowerShell):

```powershell
$Version = "0.1.0"
$Base = "https://github.com/kellyadamtan/kogentic-rules-lint/releases/download/v$Version"
Invoke-WebRequest "$Base/kogentic-rules-lint_${Version}_windows_amd64.zip" -OutFile krl.zip
Invoke-WebRequest "$Base/checksums.txt" -OutFile checksums.txt

# Compare with the matching line in checksums.txt
(Get-FileHash krl.zip -Algorithm SHA256).Hash.ToLower()
Select-String "windows_amd64.zip" checksums.txt

Expand-Archive krl.zip -DestinationPath .
.\kogentic-rules-lint.exe version
```

### Unsigned binaries (macOS and Windows)

v0.1 binaries are **not code-signed**, so your OS warns the first time you run one. Check the SHA-256 against `checksums.txt` first, then:

- **macOS (Gatekeeper)**: "cannot be opened because the developer cannot be verified". Remove the download quarantine flag:

  ```sh
  xattr -d com.apple.quarantine ./kogentic-rules-lint
  ```

  Or in Finder, right-click (Control-click) the binary, choose **Open**, then **Open** again. You can also allow it under **System Settings → Privacy & Security → Open Anyway**.
- **Windows (SmartScreen)**: "Windows protected your PC". Click **More info**, then **Run anyway**. If the file is blocked, right-click it, open **Properties**, tick **Unblock** and click OK (or run `Unblock-File .\kogentic-rules-lint.exe` in PowerShell).

## Usage

```text
kogentic-rules-lint [path]                  same as `lint [path]`
kogentic-rules-lint lint [path] [flags]
    --format text|json|sarif|github         output format (default text)
    --strict                                exit 1 on warnings too
    --rule V001,S001                        run only these rules
    --ignore C002                           skip these rules
    --config FILE                           config file (default <path>/.kogentic-rules-lint.yaml)
    --send-to-kogentic                      opt-in upload of the findings report (see Privacy)
kogentic-rules-lint explain [RULE_ID]       print a rule's docs, or list all rules
kogentic-rules-lint init [path]             write .kogentic-rules-lint.yaml with defaults
kogentic-rules-lint harden "<correction>" [--as rule|hook|test] [--target claude|cursor] [--write]
kogentic-rules-lint login | logout | whoami opt-in Kogentic connector
kogentic-rules-lint version
```

### Files it reads

`CLAUDE.md`, `CLAUDE.local.md`, `.claude/CLAUDE.md`, `.claude/rules/*.md`, `AGENTS.md` (any folder), `.cursor/rules/**/*.mdc` (and stray `.md`), `.cursorrules`, `.github/copilot-instructions.md`, `.github/instructions/**/*.instructions.md`, `.windsurfrules`, `.windsurf/rules/*.md`, `.clinerules` (file or folder), `.roo/rules*/`, `GEMINI.md`. It follows `@path` imports in CLAUDE.md one level deep and respects `.gitignore` files (root and nested). Add more files with `paths.extra` in the config.

## Rules

| ID | Name | Default | What it flags |
|---|---|---|---|
| [V001](docs/rules/V001.md) | vague-instruction | warning | An instruction with no checkable criterion ("clean code", "be careful", "as needed", "etc."). |
| [V002](docs/rules/V002.md) | generic-filler | info | Things agents already do ("be helpful", "think step by step"). |
| [V003](docs/rules/V003.md) | unbounded-quantifier | warning | "always" / "never" with no scope. |
| [V004](docs/rules/V004.md) | hedged-rule | warning | "try to", "if possible", "ideally" on a rule. |
| [S001](docs/rules/S001.md) | always-on-should-auto-attach | warning | Always-on content that only talks about one folder or file type; suggests the exact `globs:` / `paths:` / `applyTo:` / `trigger: glob`. |
| [S002](docs/rules/S002.md) | always-on-budget | warning | Estimated always-on tokens per tool over budget (default 2,000), with a per-file breakdown. |
| [S003](docs/rules/S003.md) | too-long | warning | Cursor rule over 500 lines; Windsurf rule over ~12k characters. |
| [S004](docs/rules/S004.md) | glob-matches-nothing | warning | A `globs` / `paths` / `applyTo` pattern that matches no file. |
| [F001](docs/rules/F001.md) | mdc-extension | error | A `.md` file in `.cursor/rules/` (Cursor ignores it). |
| [F002](docs/rules/F002.md) | bad-frontmatter | error | Invalid YAML, unknown keys or wrong types in frontmatter. |
| [F003](docs/rules/F003.md) | cross-vendor-frontmatter | warning | Another tool's frontmatter key, or a Windsurf rule without `trigger`. |
| [F004](docs/rules/F004.md) | missing-applyTo | error | A Copilot `*.instructions.md` without `applyTo`. |
| [F005](docs/rules/F005.md) | dead-reference | warning | An `@path` import or backticked path that does not exist. |
| [F006](docs/rules/F006.md) | unreachable-rule | warning | A Cursor rule with no `alwaysApply`, no globs and no description. |
| [C001](docs/rules/C001.md) | duplicate | warning | Near-duplicate instructions within or across files. |
| [C002](docs/rules/C002.md) | contradiction | error | Opposite directives on the same subject (package manager, indentation, quotes, test runner, language, always X vs never X). |
| [C003](docs/rules/C003.md) | drift-between-tools | error | CLAUDE.md and AGENTS.md (or Copilot instructions) disagree. |
| [E001](docs/rules/E001.md) | should-be-hook | info | A hard rule a hook or test could enforce; prints the `harden` command. |

Rule IDs are stable: a rule may be retired, but its ID will never be reused for something else.

## Output formats

- `text` (default): `file:line: severity ID name: message` plus a `fix:` line, and a summary.
- `json`: `{tool, version, summary, files, findings[]}`; each finding has `rule`, `name`, `severity`, `file`, `line`, `message`, `suggestion`, `snippet`.
- `sarif`: SARIF 2.1.0 for GitHub code scanning; each rule links to its doc page.
- `github`: `::error file=...,line=...::` workflow annotations.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | No errors (warnings and info allowed, unless `--strict`). |
| 1 | At least one error-level finding, or a warning with `--strict`. |
| 2 | Usage problem: bad flag, unknown rule ID, missing path, invalid config, unsupported `harden` request, or a refused `--send-to-kogentic`. |
| 3 | Runtime failure: an I/O error (a rules file, import, `.gitignore` or directory that can't be read; each path is printed on stderr and nothing is uploaded), or `--send-to-kogentic` / `login` / `whoami` failed (unreachable server, rejected key, or a redirect). Findings are still printed. |

## Configuration

`kogentic-rules-lint init` writes `.kogentic-rules-lint.yaml`:

```yaml
version: 1
rules:            # per-rule severity: off, info, warning, error
  V003: off
  E001: warning
budget:
  always_on_tokens: 2000
paths:
  extra: ["docs/agent-rules/*.md"]   # more files to lint
  ignore: ["vendor/**"]              # skipped in addition to .gitignore
```

Inline disables in any rules file:

```markdown
<!-- rules-lint-disable-next-line V003 -->
- Never use `any`.
- Ship it. <!-- rules-lint-disable V001 -->     (this line only)
<!-- rules-lint-disable C001 -->                 (from here on)
<!-- rules-lint-enable C001 -->                  (back on)
```

Leave out the rule IDs to disable every rule.

## harden: from correction to enforcement

You keep telling the agent the same thing. `harden` turns that sentence into something the agent cannot ignore, from a small template library (no LLM):

| Template | Example correction | `--as hook` (Claude Code) | `--as hook --target cursor` | `--as test` |
|---|---|---|---|---|
| path deny-list | "Never edit files in `dist/`" | blocks Edit/Write under the path | not in v0.1 | fails CI if the path changed |
| command deny-list | "Don't run `rm -rf`" | blocks matching Bash commands | `beforeShellExecution` deny | not possible |
| secret-file deny | "Never commit .env files" | blocks reading/editing/staging `.env`, `*.pem`, keys | shell + read-file deny | fails CI if secret files are tracked |
| must-have-test | "Every change to `src/*.go` needs a test" | blocks `git commit` when a staged file has no test | same | fails CI for changed files without tests |
| run-before-commit | "Always run `make test` before committing" | runs the command on `git commit`, blocks on failure | same | not possible |

`--as rule` writes a scoped rule file (`.claude/rules/<slug>.md` with `paths:`, or `.cursor/rules/<slug>.mdc` with `globs:`) for any correction. Without `--as`, harden picks one.

harden **always prints a diff and writes nothing unless you pass `--write`**. Claude Code hooks go in `.claude/hooks/<slug>.sh` and are registered in `.claude/settings.json` as a `PreToolUse` hook (exit code 2 blocks the call); existing settings are kept. Cursor hooks go in `.cursor/hooks/<slug>.sh` and `.cursor/hooks.json`. Test stubs go in `.kogentic/tests/<slug>.sh`. Generated scripts are plain POSIX `sh`; review them before you commit.

```sh
kogentic-rules-lint harden "Never edit files in dist/"            # preview
kogentic-rules-lint harden "Never edit files in dist/" --write    # apply
```

## In CI (GitHub Actions)

```yaml
- name: Lint agent rules
  run: |
    VERSION=0.1.0
    BASE=https://github.com/kellyadamtan/kogentic-rules-lint/releases/download/v$VERSION
    curl -fsSLO "$BASE/kogentic-rules-lint_${VERSION}_linux_amd64.tar.gz"
    curl -fsSLO "$BASE/checksums.txt"
    sha256sum --ignore-missing -c checksums.txt
    tar -xzf kogentic-rules-lint_${VERSION}_linux_amd64.tar.gz kogentic-rules-lint
    ./kogentic-rules-lint lint --format github .
```

For code scanning, run with `--format sarif > rules-lint.sarif` and upload it with `github/codeql-action/upload-sarif`.

## Privacy

- The linter runs **fully offline**. It sends nothing anywhere, has **no telemetry**, no phone-home and no update checks.
- The **optional** Kogentic connector is **off by default** and does nothing unless you pass `--send-to-kogentic` or run `login` / `whoami`.

### Optional: connect to Kogentic

`--send-to-kogentic` uploads the findings **report** only: rule IDs, severities, file paths, line numbers, short snippets (at most 200 characters; `--redact-snippets` removes them), the repo folder name, a SHA-256 hash of the `origin` remote URL, the commit SHA, and the summary counts. It **never sends file contents**. Before uploading it prints exactly what will be sent and asks `y/N` (use `--yes` in CI).

- API key: `kogentic-rules-lint login` stores it in your OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service), or set `KOGENTIC_API_KEY` (the environment variable wins). Keys are shown masked (`kg_…abcd`) and never written to config files or logs. `logout` removes it.
- Endpoint: there is **no built-in default**. Set `--kogentic-url` or `KOGENTIC_BASE_URL`. Plain `http://` is only allowed for `localhost`. Redirects are never followed: if the server answers with any 3xx, nothing is sent onward and the command exits 3.
- The Kogentic API is not public yet. To try the flow, run the local mock: `go run ./tools/mock-kogentic` (see [docs/connector.md](docs/connector.md)).

## Contributing

Contributions are welcome. Each rule is one Go file, one doc page and a pair of fixture folders, so adding or improving a rule is a small, self-contained change. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [good first issues](docs/good-first-issues.md).

## License

[MIT](LICENSE)
