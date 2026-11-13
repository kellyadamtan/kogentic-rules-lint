# Good first issues

Small, self-contained tasks for new contributors. Each one is a good first pull request. Comment on the matching issue (once issues are open) or open a draft PR that names the item.

See [CONTRIBUTING.md](../CONTRIBUTING.md) for how rules, docs and fixtures fit together.

1. **Add discovery for `.continue/rules/*.md`.** Classify the files in `internal/discover`, decide if they are always-on, and add a case to `TestClassify`.
2. **Add discovery for `.amazonq/rules/*.md`.** Same steps as above.
3. **Add discovery for `.junie/guidelines.md`.** Same steps as above.
4. **Add discovery for Zed `.rules`.** Same steps as above.
5. **Extend the `V001` vague-phrase lexicon (with fixtures).** Add phrases to `internal/rules/v001.go`, a bad case to `testdata/rules/V001/bad` (and `.expect`), and a near-miss to `good/`.
6. **Add a new contradiction subject key to `C002`** (for example "semicolons: always vs never", or "functional vs class components"). Extend the `subjects` table in `internal/rules/c002.go` with fixtures.
7. **Add `--format markdown`** for pull-request comments. Add a writer to `internal/report` and a golden file.
8. **Add a `harden` template: "forbid edits under a path" for Cursor hooks.** v0.1 supports this only for Claude Code.
9. **Translate rule docs** in `docs/rules/` (start with one language, one folder per locale).
10. **Add Windows path normalization tests** for discovery and globs (backslashes, drive letters).

Ideas for later releases (bigger than a first issue):

- `lint --fix` for S001 (split an always-on section into a scoped rule file) and F001 (rename `.md` to `.mdc`).
- A GitHub Action wrapper and a pre-commit hook definition.
- Signed and notarized release binaries.
