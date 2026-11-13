package rules

import (
	"fmt"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func init() {
	r := &core.Rule{ID: "C003", Name: "drift-between-tools", Default: core.SevError,
		Summary: "CLAUDE.md and AGENTS.md (or Copilot instructions) disagree."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, cf := range findConflicts(c) {
			if !isDriftPair(cf.a.f, cf.b.f) {
				continue
			}
			out = append(out, finding(r, cf.b.f, cf.b.in.Line,
				fmt.Sprintf("%s and %s:%d disagree on %s (%s vs %s), so different agents get different rules.", cf.b.f.Path, cf.a.f.Path, cf.a.in.Line, cf.a.key, describe(cf.b), describe(cf.a)),
				"Keep one source of truth (for example AGENTS.md) and import it from CLAUDE.md with `@AGENTS.md`, or symlink one to the other.", cf.b.in.Text))
		}
		return out
	}
	register(r)
}
