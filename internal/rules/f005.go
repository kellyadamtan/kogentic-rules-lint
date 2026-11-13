package rules

import (
	"path"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/patterns"
)

// checkablePath reports whether a backticked token is a concrete repo path
// worth checking: it has a slash, no globs or placeholders, and is relative.
func checkablePath(tok string) bool {
	if !patterns.LooksLikePath(tok) || !strings.Contains(tok, "/") {
		return false
	}
	if strings.ContainsAny(tok, "*?[]{}<>$~:@") || strings.HasPrefix(tok, "/") || strings.HasPrefix(tok, "..") {
		return false
	}
	return true
}

func init() {
	r := &core.Rule{ID: "F005", Name: "dead-reference", Default: core.SevWarning,
		Summary: "An @path import or a backticked path that does not exist."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range instructionFiles(c) {
			for _, im := range f.Imports {
				if im.External || im.Exists {
					continue
				}
				out = append(out, finding(r, f, im.Line, "Import "+im.Raw+" points to a file that does not exist, so it loads nothing.", "Fix the path (it is relative to "+f.Path+") or remove the import.", im.Raw))
			}
			for _, in := range f.Instructions {
				if patterns.IsNegative(in.Text) {
					continue // "never edit `dist/`" is about a path that may not exist yet
				}
				for _, tok := range patterns.Backticked(in.Text) {
					if !checkablePath(tok) {
						continue
					}
					clean := strings.TrimSuffix(strings.TrimPrefix(tok, "./"), "/")
					if c.Exists(clean) || c.Exists(path.Join(path.Dir(f.Path), clean)) {
						continue
					}
					out = append(out, finding(r, f, in.Line, "Path `"+tok+"` does not exist in the repo.", "Update the path, or remove the reference if the file is gone.", tok))
				}
			}
		}
		return out
	}
	register(r)
}
