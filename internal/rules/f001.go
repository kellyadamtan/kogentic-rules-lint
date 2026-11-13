package rules

import (
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func init() {
	r := &core.Rule{ID: "F001", Name: "mdc-extension", Default: core.SevError,
		Summary: "A .md file inside .cursor/rules/, which Cursor ignores."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			if f.Kind == core.KindCursorRuleMD {
				out = append(out, finding(r, f, 1, "Cursor only loads .mdc files from .cursor/rules/, so this file is ignored.", "Rename it to "+strings.TrimSuffix(f.Path, ".md")+".mdc and add frontmatter (description, globs, alwaysApply).", ""))
			}
		}
		return out
	}
	register(r)
}
