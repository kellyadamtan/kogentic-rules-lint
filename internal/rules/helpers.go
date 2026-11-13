package rules

import (
	"regexp"
	"strings"
)

var criterionRe = regexp.MustCompile("`|\\d|\\b(verify|verified|check that|checked by|must pass|passes|such as|e\\.g\\.|for example|i\\.e\\.|defined as|meaning|means)\\b")

// hasCriterion reports whether a normalized instruction already carries
// something checkable: code, a number, an example or an explicit check.
func hasCriterion(norm string) bool { return criterionRe.MatchString(norm) }

func quote(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 80 {
		s = string(r[:77]) + "..."
	}
	return "\"" + s + "\""
}

// stringList reads a frontmatter value that may be a comma-separated string
// or a list of strings.
func stringList(v any) []string {
	var out []string
	switch t := v.(type) {
	case string:
		for _, p := range strings.Split(t, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}
