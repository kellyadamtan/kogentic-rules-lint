# Contributing to kogentic-rules-lint

Thanks for helping. Small, focused pull requests are easiest to review.

## Dev setup (3 commands)

You need Go (the version in `go.mod`) and git.

```sh
git clone https://github.com/kellyadamtan/kogentic-rules-lint && cd kogentic-rules-lint
go test ./...
go run ./cmd/kogentic-rules-lint examples/messy-repo
```

Before you push, run what CI runs:

```sh
go vet ./...
gofmt -l .            # must print nothing
go test -race ./...
```

`golangci-lint run` (v2) also runs in CI with `.golangci.yml`.

## Project layout

```text
cmd/kogentic-rules-lint/   main package
internal/discover/         finds rules files, parses frontmatter, splits instructions
internal/rules/            one file per rule (v001.go, s001.go, ...)
internal/patterns/         shared phrase detectors (used by E001 and harden)
internal/engine/           runs rules, applies config and inline disables
internal/report/           text, json, sarif, github output
internal/harden/           harden templates
internal/connector/        opt-in Kogentic client (off by default)
internal/mockkogentic/     local mock API for tests
docs/rules/<ID>.md         rule docs (embedded for `explain`)
testdata/rules/<ID>/       bad/ and good/ fixtures per rule
testdata/golden/           golden output for examples/messy-repo
examples/                  sample repo with planted problems
```

## Adding or changing a rule

Each rule is three things:

1. **Code**: `internal/rules/<id>.go` (lower-case ID). Register it in `init()`:

   ```go
   package rules

   import "github.com/kellyadamtan/kogentic-rules-lint/internal/core"

   func init() {
   	r := &core.Rule{ID: "V005", Name: "my-new-rule", Default: core.SevWarning,
   		Summary: "One sentence on what it flags."}
   	r.Check = func(c *core.Context) []core.Finding {
   		var out []core.Finding
   		for _, f := range instructionFiles(c) {
   			for _, in := range f.Instructions {
   				if /* in.Norm matches your pattern */ false {
   					out = append(out, finding(r, f, in.Line, quote(in.Text)+" ...what is wrong.", "How to fix it.", in.Text))
   				}
   			}
   		}
   		return out
   	}
   	register(r)
   }
   ```

   Rules must be deterministic and offline: no network, no LLM, no randomness.

2. **Docs**: `docs/rules/<ID>.md` with what it flags, why it matters, a bad and a good example, and how to fix it. Copy an existing page.

3. **Fixtures**: `testdata/rules/<ID>/bad/` and `testdata/rules/<ID>/good/`, each a tiny repo. `bad/.expect` lists every expected finding as `path:line`, one per line:

   ```text
   CLAUDE.md:3
   .cursor/rules/a.mdc:2
   ```

   `TestFixtures` runs the rule on `bad/` and requires exactly those findings, and requires `good/` to be clean. Put the near-misses that must not be flagged in `good/`.

If output changes, update the golden files and review the diff:

```sh
go test ./internal/report -update
git diff testdata/golden
```

Rule IDs are stable. Never reuse or renumber an ID; retire it instead.

## Commit sign-off (DCO)

We use the [Developer Certificate of Origin](https://developercertificate.org/) instead of a CLA. Sign off every commit to certify you wrote it or have the right to submit it under the MIT license:

```sh
git commit -s -m "Add V005 my-new-rule"
```

This adds a `Signed-off-by: Your Name <you@example.com>` line.

## Ground rules

- No telemetry, no network calls in the linter path. The Kogentic connector stays opt-in and off by default.
- Never commit secrets, tokens or real API keys, including in tests (use obvious fakes like `kg_test_...`).
- Be kind. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
