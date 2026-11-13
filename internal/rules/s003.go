package rules

import (
	"fmt"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

const (
	cursorMaxLines   = 500
	windsurfMaxChars = 12000
)

func init() {
	r := &core.Rule{ID: "S003", Name: "too-long", Default: core.SevWarning,
		Summary: "A rules file longer than its vendor's guidance."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			switch f.Kind {
			case core.KindCursorRule:
				if n := len(f.Lines); n > cursorMaxLines {
					out = append(out, finding(r, f, cursorMaxLines+1, fmt.Sprintf("Cursor rule has %d lines; Cursor recommends keeping rules under %d.", n, cursorMaxLines), "Split it into smaller rules scoped with globs.", ""))
				}
			case core.KindWindsurfRule, core.KindWindsurfLegacy:
				if f.Size > windsurfMaxChars {
					out = append(out, finding(r, f, 1, fmt.Sprintf("Windsurf rule has %d characters; Windsurf truncates workspace rules over about %d.", f.Size, windsurfMaxChars), "Split it into several .windsurf/rules files.", ""))
				}
			}
		}
		return out
	}
	register(r)
}
