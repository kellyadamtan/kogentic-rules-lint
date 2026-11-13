// Package rules contains every lint rule. Each rule lives in its own file
// (v001.go, s001.go, ...) with docs in docs/rules/<ID>.md and fixtures in
// testdata/rules/<ID>/{bad,good}.
package rules

import (
	"sort"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var registry = map[string]*core.Rule{}

func register(r *core.Rule) {
	if _, dup := registry[r.ID]; dup {
		panic("duplicate rule " + r.ID)
	}
	registry[r.ID] = r
}

// All returns every rule sorted by ID.
func All() []*core.Rule {
	out := make([]*core.Rule, 0, len(registry))
	for _, r := range registry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns the rule with the given ID.
func Get(id string) (*core.Rule, bool) {
	r, ok := registry[id]
	return r, ok
}

// finding builds a finding for rule r.
func finding(r *core.Rule, f *core.File, line int, msg, suggestion, snippet string) core.Finding {
	if line < 1 {
		line = 1
	}
	return core.Finding{
		RuleID: r.ID, RuleName: r.Name, Severity: r.Default,
		Path: f.Path, Line: line, Message: msg, Suggestion: suggestion, Snippet: core.Snip(snippet),
	}
}

// instructionFiles are files whose body is natural-language instructions
// (everything except Cursor .md files, which Cursor ignores).
func instructionFiles(c *core.Context) []*core.File {
	var out []*core.File
	for _, f := range c.Files {
		if f.Kind != core.KindCursorRuleMD {
			out = append(out, f)
		}
	}
	return out
}
