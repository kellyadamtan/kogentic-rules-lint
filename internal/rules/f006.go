package rules

import (
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func init() {
	r := &core.Rule{ID: "F006", Name: "unreachable-rule", Default: core.SevWarning,
		Summary: "A Cursor rule with alwaysApply false, no globs and no description."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			if f.Kind != core.KindCursorRule || f.FM.Err != nil {
				continue
			}
			always, _ := f.FM.Data["alwaysApply"].(bool)
			desc, _ := f.FM.Data["description"].(string)
			if always || len(stringList(f.FM.Data["globs"])) > 0 || strings.TrimSpace(desc) != "" {
				continue
			}
			out = append(out, finding(r, f, f.FM.Line("alwaysApply"), "This Cursor rule only loads when someone @-mentions it: no alwaysApply, no globs, no description.", "Add `globs:` to auto-attach it, a `description:` so the agent can pick it, or `alwaysApply: true`.", ""))
		}
		return out
	}
	register(r)
}
