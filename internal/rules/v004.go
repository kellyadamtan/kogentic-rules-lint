package rules

import (
	"regexp"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var v004Hedge = regexp.MustCompile(`\b(try to|try and|if possible|when possible|where possible|whenever possible|ideally|if you can|preferably|if feasible|as much as possible)\b`)

func init() {
	r := &core.Rule{ID: "V004", Name: "hedged-rule", Default: core.SevWarning,
		Summary: `A hedge ("try to", "if possible", "ideally") on something that reads like a rule.`}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				if m := v004Hedge.FindString(in.Norm); m != "" {
					out = append(out, finding(r, f, in.Line, quote(in.Text)+` is hedged ("`+m+`"), so the agent may skip it.`, "Decide: make it a hard rule (or a hook, see `harden`), or say plainly that it is optional and when.", in.Text))
				}
			}
		}
		return out
	}
	register(r)
}
