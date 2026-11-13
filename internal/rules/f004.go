package rules

import "github.com/kellyadamtan/kogentic-rules-lint/internal/core"

func init() {
	r := &core.Rule{ID: "F004", Name: "missing-applyTo", Default: core.SevError,
		Summary: "A Copilot *.instructions.md file without applyTo."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			if f.Kind != core.KindCopilotInstr || f.FM.Err != nil {
				continue
			}
			if v, ok := f.FM.Data["applyTo"].(string); !ok || v == "" {
				out = append(out, finding(r, f, f.FM.Line("applyTo"), "Copilot instructions file has no `applyTo`, so Copilot never applies it automatically.", "Add frontmatter, e.g. `applyTo: \"**/*.ts\"` (or \"**\" for every file).", ""))
			}
		}
		return out
	}
	register(r)
}
