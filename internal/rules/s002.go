package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

// DefaultBudget is the default always-on token budget per tool.
const DefaultBudget = 2000

func init() {
	r := &core.Rule{ID: "S002", Name: "always-on-budget", Default: core.SevWarning,
		Summary: "Estimated always-on tokens per tool exceed the budget."}
	r.Check = func(c *core.Context) []core.Finding {
		budget := c.Budget
		if budget <= 0 {
			budget = DefaultBudget
		}
		byTool := map[string][]*core.File{}
		for _, f := range c.Files {
			if f.AlwaysOn {
				byTool[f.Tool] = append(byTool[f.Tool], f)
			}
		}
		tools := make([]string, 0, len(byTool))
		for t := range byTool {
			tools = append(tools, t)
		}
		sort.Strings(tools)
		var out []core.Finding
		for _, t := range tools {
			files := byTool[t]
			total := 0
			for _, f := range files {
				total += f.Tokens()
			}
			if total <= budget {
				continue
			}
			sort.SliceStable(files, func(i, j int) bool { return files[i].Tokens() > files[j].Tokens() })
			parts := make([]string, len(files))
			for i, f := range files {
				parts[i] = fmt.Sprintf("%s ~%d", f.Path, f.Tokens())
			}
			out = append(out, finding(r, files[0], 1,
				fmt.Sprintf("Always-on rules for %s cost ~%d tokens every session (budget %d): %s.", t, total, budget, strings.Join(parts, ", ")),
				"Move area-specific sections into scoped rules (see S001), delete filler (V002), or raise budget.always_on_tokens in .kogentic-rules-lint.yaml.", ""))
		}
		return out
	}
	register(r)
}
