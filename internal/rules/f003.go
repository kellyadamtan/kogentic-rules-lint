package rules

import (
	"fmt"
	"sort"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func init() {
	r := &core.Rule{ID: "F003", Name: "cross-vendor-frontmatter", Default: core.SevWarning,
		Summary: "Another vendor's frontmatter key, or a Windsurf rule without trigger."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			known, ok := knownKeys[f.Kind]
			if !ok || !f.FM.Present || f.FM.Err != nil {
				if ok && f.Kind == core.KindWindsurfRule && !f.FM.Present {
					out = append(out, finding(r, f, 1, "Windsurf rule has no frontmatter, so it has no `trigger`.", "Add frontmatter with `trigger: always_on`, `manual`, `model_decision` or `glob`.", ""))
				}
				continue
			}
			keys := make([]string, 0, len(f.FM.Data))
			for k := range f.FM.Data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if vendor, isVendor := vendorKeys[k]; isVendor && !known[k] {
					out = append(out, finding(r, f, f.FM.Line(k), fmt.Sprintf("%q is a %s key; this file's tool ignores it.", k, vendor), fmt.Sprintf("Use this tool's own scoping key (%s) instead.", scopeKey(f.Kind)), k))
				}
			}
			if f.Kind == core.KindWindsurfRule {
				if _, has := f.FM.Data["trigger"]; !has {
					out = append(out, finding(r, f, f.FM.StartLine, "Windsurf rule has no `trigger`.", "Add `trigger: always_on`, `manual`, `model_decision` or `glob`.", ""))
				}
			}
		}
		return out
	}
	register(r)
}
