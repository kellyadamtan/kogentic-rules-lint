package rules

import (
	"regexp"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var (
	v001MakeMore  = regexp.MustCompile(`\bmake (it|this|that|the code|code|them|things|everything|it all) (more|less) [a-z-]+`)
	v001Phrases   = regexp.MustCompile(`\b(be careful|as needed|where appropriate|when appropriate|as appropriate|if needed|when necessary|where necessary|best practices?|properly|appropriately|sensible defaults)\b`)
	v001Adjective = regexp.MustCompile(`^(please\s+)?(always\s+)?(write|keep|make|use|follow|produce|ensure|create|structure|name|format|design|refactor|code|do)\b.*\b(clean|nice|nicer|better|proper|good|neat|elegant|cleaner|tidy)\b`)
	v001Etc       = regexp.MustCompile(`\betc\.?\s*\)?\s*$`)
)

func init() {
	r := &core.Rule{ID: "V001", Name: "vague-instruction", Default: core.SevWarning,
		Summary: "An instruction with no checkable criterion."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		const fix = `Say what to do and how to check it: "When <situation>, do <action>; verify by <check>".`
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				switch {
				case v001Etc.MatchString(in.Norm):
					out = append(out, finding(r, f, in.Line, quote(in.Text)+` ends a list with "etc.": the agent cannot guess the rest.`, "List every case, or describe the rule that covers them.", in.Text))
				case hasCriterion(in.Norm):
				case v001MakeMore.MatchString(in.Norm), v001Phrases.MatchString(in.Norm), v001Adjective.MatchString(in.Norm):
					out = append(out, finding(r, f, in.Line, quote(in.Text)+" has no checkable criterion.", fix, in.Text))
				}
			}
		}
		return out
	}
	register(r)
}
