package rules

import (
	"regexp"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var (
	v003Quant = regexp.MustCompile(`\b(always|never)\b`)
	v003Scope = regexp.MustCompile(`\b(when|whenever|if|unless|before|after|while|during|in|inside|within|under|for|on|only|except)\b`)
)

func init() {
	r := &core.Rule{ID: "V003", Name: "unbounded-quantifier", Default: core.SevWarning,
		Summary: `"always" or "never" with no scope.`}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				m := v003Quant.FindString(in.Norm)
				if m == "" || strings.ContainsAny(in.Norm, "`*/") || v003Scope.MatchString(in.Norm) {
					continue
				}
				out = append(out, finding(r, f, in.Line, quote(in.Text)+` says "`+m+`" with no scope.`, "Add a condition or a glob (e.g. \"in `src/**/*.ts`\" or \"when editing migrations\"), or confirm it truly applies everywhere.", in.Text))
			}
		}
		return out
	}
	register(r)
}
