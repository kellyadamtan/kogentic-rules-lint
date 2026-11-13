package rules

import (
	"regexp"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var v002Filler = regexp.MustCompile(`\b(be helpful|be accurate|high[- ]quality code|think step[- ]by[- ]step|think carefully|you are an expert|you are a senior|you are a world[- ]class|do your best|don't make mistakes|do not make mistakes|don't hallucinate|do not hallucinate|write good code|be thorough|be smart)\b`)

func init() {
	r := &core.Rule{ID: "V002", Name: "generic-filler", Default: core.SevInfo,
		Summary: "An instruction the model already follows; it only costs tokens."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				if m := v002Filler.FindString(in.Norm); m != "" {
					out = append(out, finding(r, f, in.Line, quote(in.Text)+` is generic filler ("`+m+`"): agents already do this.`, "Delete it, or replace it with a project-specific rule.", in.Text))
				}
			}
		}
		return out
	}
	register(r)
}
