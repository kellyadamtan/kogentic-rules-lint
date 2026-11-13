# The optional Kogentic connector

The linter is fully useful offline. The connector exists for teams who want to collect lint reports in one place. It is **off by default** and nothing in the linter talks to the network unless you run one of:

- `kogentic-rules-lint lint --send-to-kogentic`
- `kogentic-rules-lint login`
- `kogentic-rules-lint whoami` (only with a base URL set)

There is no telemetry, no phone-home and no update check.

## Configuration

| Setting | Flag | Environment | Default |
|---|---|---|---|
| Base URL | `--kogentic-url` | `KOGENTIC_BASE_URL` | none (required) |
| API key | `login` (OS keychain) | `KOGENTIC_API_KEY` | none |

- The key is stored in the OS keychain under service `kogentic`, account `default` (macOS Keychain, Windows Credential Manager, Linux Secret Service). `KOGENTIC_API_KEY` overrides it, which is handy in CI.
- Keys are printed masked (`kg_…abcd`) and never written to config files or logs.
- `https://` is required, except `http://localhost` / loopback addresses for local testing.
- Redirects are never followed. Any 3xx answer (to login, whoami or the upload) is an error with exit code 3, so the key and the report only ever go to the configured URL.

## What `--send-to-kogentic` sends

`POST <base>/v1/rules-lint/reports` with `Authorization: Bearer <key>` and this JSON body:

```json
{
  "tool": "rules-lint",
  "version": "0.1.0",
  "repo": {
    "name": "my-repo",
    "remote_hash": "<sha256 of the origin remote URL>",
    "sha": "<HEAD commit>"
  },
  "findings": [
    { "rule": "V001", "severity": "warning", "file": "CLAUDE.md", "line": 5, "snippet": "Write clean code." }
  ],
  "summary": { "errors": 0, "warnings": 1, "always_on_tokens": 215 }
}
```

- Snippets are at most 200 characters. `--redact-snippets` removes them.
- File contents are never sent. The remote URL itself is never sent, only its hash.
- The full payload is printed before sending, and you are asked `Send it? [y/N]`. Without a terminal, the upload is refused unless you pass `--yes`.

`login` and `whoami` call `GET <base>/v1/me` to check the key.

## Trying it locally

The Kogentic API is not public yet. A local mock implements the two endpoints:

```sh
go run ./tools/mock-kogentic -addr 127.0.0.1:4810 -key kg_test_example &
export KOGENTIC_BASE_URL=http://127.0.0.1:4810
export KOGENTIC_API_KEY=kg_test_example
kogentic-rules-lint lint examples/messy-repo --send-to-kogentic --yes
```
