# Agent instructions for this repo

- Before you finish a change, run `go vet ./...`, `gofmt -l .` and `go test ./...`; all must pass and `gofmt -l .` must print nothing.
- Each lint rule is `internal/rules/<id>.go` plus `docs/rules/<ID>.md` plus `testdata/rules/<ID>/{bad,good}`; change all three together.
- When output text changes, run `go test ./internal/report -update` and review `git diff testdata/golden`.
- Keep the linter offline: code under `internal/rules`, `internal/discover` and `internal/engine` must not import `net/http`.
- Use obvious fake keys in tests, such as `kg_test_...`.
