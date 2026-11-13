## What and why

## Checklist

- [ ] `go vet ./...`, `gofmt -l .` and `go test ./...` pass
- [ ] New or changed rules have fixtures in `testdata/rules/<ID>/{bad,good}` and docs in `docs/rules/<ID>.md`
- [ ] Golden files updated (`go test ./internal/report -update`) and the diff reviewed, if output changed
- [ ] Commits are signed off (`git commit -s`, see CONTRIBUTING.md)
