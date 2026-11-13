package rules

import (
	"fmt"
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

// scopeKey returns the frontmatter key that holds globs for a file kind.
func scopeKey(k core.Kind) string {
	switch k {
	case core.KindCursorRule, core.KindWindsurfRule:
		return "globs"
	case core.KindClaudeRule:
		return "paths"
	case core.KindCopilotInstr:
		return "applyTo"
	}
	return ""
}

func globMatchesAny(g string, files []string) bool {
	g = strings.TrimPrefix(strings.TrimPrefix(g, "./"), "/")
	g = strings.TrimSuffix(g, "/")
	for _, f := range files {
		if ok, _ := doublestar.Match(g, f); ok {
			return true
		}
		if !strings.Contains(g, "/") {
			if ok, _ := doublestar.Match(g, path.Base(f)); ok {
				return true
			}
		}
		// a bare directory name or prefix ("src" or "src/") matches files under it
		if !strings.ContainsAny(g, "*?[{") && strings.HasPrefix(f, g+"/") {
			return true
		}
	}
	return false
}

func init() {
	r := &core.Rule{ID: "S004", Name: "glob-matches-nothing", Default: core.SevWarning,
		Summary: "A globs/paths/applyTo pattern matches no file in the repo."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			key := scopeKey(f.Kind)
			if key == "" || f.FM.Data == nil {
				continue
			}
			for _, g := range stringList(f.FM.Data[key]) {
				if g == "**" || g == "**/*" || g == "*" {
					continue
				}
				if !doublestar.ValidatePattern(g) {
					out = append(out, finding(r, f, f.FM.Line(key), fmt.Sprintf("%s pattern %q is not a valid glob.", key, g), "Fix the pattern syntax.", g))
					continue
				}
				if !globMatchesAny(g, c.RepoFiles) {
					out = append(out, finding(r, f, f.FM.Line(key), fmt.Sprintf("%s pattern %q matches no file in the repo, so this rule never loads.", key, g), "Fix the pattern (paths are relative to the repo root) or delete the rule.", g))
				}
			}
		}
		return out
	}
	register(r)
}
