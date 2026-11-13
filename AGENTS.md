# Agent instructions for this repo

- Before you finish a change, run `go vet ./...`, `gofmt -l .` and `go test ./...`; all must pass and `gofmt -l .` must print nothing.
- Each lint rule is `internal/rules/<id>.go` plus `docs/rules/<ID>.md` plus `testdata/rules/<ID>/{bad,good}`; change all three together.
- When output text changes, run `go test ./internal/report -update` and review `git diff testdata/golden`.
- Keep the linter offline: the lint path (`internal/discover`, `rules`, `engine`, `report`, `patterns`, `config`, `core`) must not depend on `net`, `net/http`, `net/url` or `os/exec`, even indirectly. `TestLinterPathIsOffline` enforces this.
- Use obvious fake keys in tests, such as `kg_test_...`.
