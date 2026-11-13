// Package docs embeds the rule documentation for `kogentic-rules-lint explain`.
package docs

import (
	"embed"
	"io/fs"
)

//go:embed rules/*.md
var rulesFS embed.FS

// Rule returns the markdown doc for a rule ID.
func Rule(id string) (string, bool) {
	b, err := fs.ReadFile(rulesFS, "rules/"+id+".md")
	if err != nil {
		return "", false
	}
	return string(b), true
}
