# Rules

- Wrap returned errors with `fmt.Errorf`; verify by `go vet ./...`.
- Keep functions under 40 lines.
- Use good names such as `userID`, not `u`.
- Run the linter before you finish.
