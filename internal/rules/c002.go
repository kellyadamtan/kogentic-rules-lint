package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

// claim is one directive about a subject, e.g. package manager = pnpm.
type claim struct {
	f        *core.File
	in       core.Instruction
	key      string
	value    string
	positive bool
}

type subject struct {
	key string
	re  *regexp.Regexp // group 1 is the value
}

// subjects is the deterministic subject-key table used by C002 and C003.
var subjects = []subject{
	{"package manager", regexp.MustCompile(`\b(?:use|prefer|only use|install (?:dependencies|deps|packages) with|run scripts with)\s+(npm|pnpm|yarn|bun)\b`)},
	{"indentation", regexp.MustCompile(`\b(?:use|indent with|indent using|prefer)\s+(tabs|\d+[ -]?spaces?|spaces)\b`)},
	{"quotes", regexp.MustCompile(`\b(?:use|prefer)\s+(single|double)[ -]quotes?\b`)},
	{"test runner", regexp.MustCompile(`\b(?:use|prefer|run tests with|write tests (?:with|in|using)|test with)\s+(jest|vitest|mocha|pytest|unittest)\b`)},
	{"language", regexp.MustCompile(`\b(?:use|write in|write code in|prefer)\s+(typescript|javascript)\b`)},
}

var (
	negBefore = regexp.MustCompile(`\b(don't|do not|dont|never|avoid|must not|should not|shouldn't|no longer)\s+(always\s+)?$`)
	notAfter  = regexp.MustCompile(`^\s*,?\s*(?:not|never|instead of|rather than)\s+([a-z0-9-]+(?: spaces?)?)`)
	spacesRe  = regexp.MustCompile(`^(\d+)[ -]?spaces?$`)
	alwaysRe  = regexp.MustCompile(`^(always|never)\s+(.+)$`)
)

func normValue(v string) string {
	if m := spacesRe.FindStringSubmatch(v); m != nil {
		return m[1] + " spaces"
	}
	return v
}

// claims extracts subject claims from an instruction.
func claims(f *core.File, in core.Instruction) []claim {
	var out []claim
	n := in.Norm
	for _, s := range subjects {
		for _, m := range s.re.FindAllStringSubmatchIndex(n, -1) {
			val := normValue(n[m[2]:m[3]])
			pos := !negBefore.MatchString(n[:m[0]])
			out = append(out, claim{f, in, s.key, val, pos})
			if mm := notAfter.FindStringSubmatch(n[m[1]:]); mm != nil {
				out = append(out, claim{f, in, s.key, normValue(mm[1]), false})
			}
		}
	}
	if m := alwaysRe.FindStringSubmatch(strings.Trim(n, " .!")); m != nil {
		ws := words(m[2])
		if len(ws) > 6 {
			ws = ws[:6]
		}
		if len(ws) >= 2 {
			out = append(out, claim{f, in, "always/never", strings.Join(ws, " "), m[1] == "always"})
		}
	}
	return out
}

func conflicts(a, b claim) bool {
	if a.key != b.key || (a.f == b.f && a.in.Line == b.in.Line) {
		return false
	}
	if a.key == "always/never" {
		return a.value == b.value && a.positive != b.positive
	}
	if a.positive && b.positive && a.value != b.value {
		return true
	}
	return a.value == b.value && a.positive != b.positive
}

func describe(c claim) string {
	if c.key == "always/never" {
		if c.positive {
			return "always " + c.value
		}
		return "never " + c.value
	}
	if c.positive {
		return c.value
	}
	return "not " + c.value
}

// isDriftPair reports whether a and b are a Claude file vs an AGENTS.md or
// Copilot file (C003 territory).
func isDriftPair(a, b *core.File) bool {
	if a == b {
		return false
	}
	other := func(f *core.File) bool { return f.Tool == "agents" || f.Tool == "copilot" }
	return (a.Tool == "claude" && other(b)) || (b.Tool == "claude" && other(a))
}

type conflict struct{ a, b claim }

// findConflicts returns one conflict per (instruction pair, subject).
func findConflicts(c *core.Context) []conflict {
	var all []claim
	for _, f := range instructionFiles(c) {
		for _, in := range f.Instructions {
			all = append(all, claims(f, in)...)
		}
	}
	seen := map[string]bool{}
	var out []conflict
	for j := range all {
		for i := 0; i < j; i++ {
			if !conflicts(all[i], all[j]) {
				continue
			}
			k := fmt.Sprintf("%s:%d|%s:%d|%s", all[i].f.Path, all[i].in.Line, all[j].f.Path, all[j].in.Line, all[i].key)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, conflict{all[i], all[j]})
		}
	}
	return out
}

func init() {
	r := &core.Rule{ID: "C002", Name: "contradiction", Default: core.SevError,
		Summary: "Opposite directives on the same subject."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, cf := range findConflicts(c) {
			if isDriftPair(cf.a.f, cf.b.f) {
				continue // reported by C003
			}
			out = append(out, finding(r, cf.b.f, cf.b.in.Line,
				fmt.Sprintf("%s contradicts %s:%d on %s (%s vs %s).", quote(cf.b.in.Text), cf.a.f.Path, cf.a.in.Line, cf.a.key, describe(cf.b), describe(cf.a)),
				"Keep one directive and delete the other, or scope each one to where it applies.", cf.b.in.Text))
		}
		return out
	}
	register(r)
}
