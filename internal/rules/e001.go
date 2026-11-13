package rules

import (
	"fmt"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/patterns"
)

func init() {
	r := &core.Rule{ID: "E001", Name: "should-be-hook", Default: core.SevInfo,
		Summary: "A hard rule that a hook or test could enforce."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				p := patterns.Classify(in.Text)
				if p.Kind == patterns.None {
					continue
				}
				as := "hook"
				if p.Kind == patterns.MustHaveTest {
					as = "test"
				}
				cmd := fmt.Sprintf("kogentic-rules-lint harden %q --as %s", strings.TrimSpace(in.Text), as)
				out = append(out, finding(r, f, in.Line,
					fmt.Sprintf("%s is mechanically checkable (%s); an agent can still ignore a sentence.", quote(in.Text), p.Kind),
					"Enforce it: "+cmd, in.Text))
			}
		}
		return out
	}
	register(r)
}
