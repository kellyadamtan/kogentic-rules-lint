// Command kogentic-rules-lint lints coding-agent rules files.
package main

import (
	"os"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/cli"
)

// Set by the release build (-ldflags "-X main.version=...").
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.Version, cli.Commit, cli.Date = version, commit, date
	os.Exit(cli.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
