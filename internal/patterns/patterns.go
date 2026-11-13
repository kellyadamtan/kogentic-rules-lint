// Package patterns holds the deterministic phrase detectors shared by the
// lint rules and the harden command. No LLM, just regular expressions.
package patterns

import (
	"regexp"
	"strings"
)

// Kind is the kind of mechanically checkable rule.
type Kind int

// Pattern kinds used by E001 and harden.
const (
	None            Kind = iota
	PathDeny             // never edit dist/
	CommandDeny          // don't run rm -rf
	SecretDeny           // never commit .env
	MustHaveTest         // every change to src/*.ts needs a test
	RunBeforeCommit      // always run `make test` before committing
)

func (k Kind) String() string {
	switch k {
	case PathDeny:
		return "path deny-list"
	case CommandDeny:
		return "command deny-list"
	case SecretDeny:
		return "secret-file deny"
	case MustHaveTest:
		return "file pattern must have a test"
	case RunBeforeCommit:
		return "run a check before commit"
	}
	return "none"
}

// Pattern is the result of Classify.
type Pattern struct {
	Kind     Kind
	Paths    []string // paths or globs mentioned
	Commands []string // commands mentioned
	Text     string
}

var (
	backtickRe = regexp.MustCompile("`([^`]+)`")
	negRe      = regexp.MustCompile(`\b(never|don't|dont|do not|must not|mustn't|shall not|should not|shouldn't|avoid|no)\b`)
	editVerbRe = regexp.MustCompile(`\b(edit|edits|editing|modify|change|touch|write to|write|delete|update|overwrite)\b`)
	runVerbRe  = regexp.MustCompile(`\b(run|execute|use|call|invoke)\b`)
	dangerRe   = regexp.MustCompile(`(rm -rf|git push --force|git push -f\b|force[- ]push|--no-verify|git reset --hard|drop table|drop database|sudo\b|chmod 777)`)
	secretVerb = regexp.MustCompile(`\b(commit|commits|push|add|check in|read|print|log|expose|paste|share)\b`)
	secretObj  = regexp.MustCompile(`(\.env\b|\bsecrets?\b|\bcredentials?\b|\bapi[ -]?keys?\b|\bprivate keys?\b|\.pem\b|\bid_rsa\b|\btokens?\b|\bpasswords?\b)`)
	testNeedRe = regexp.MustCompile(`\b(every|each|all|any)\b.*\b(needs?|must have|requires?|should have|must come with|must include)\b.*\btests?\b|\b(add|write) (a |unit )?tests? (for|with) (every|each|all|any)\b`)
	runBefore  = regexp.MustCompile(`\b(always )?run\b.*\bbefore\b.*\b(commit|committing|push|pushing|merge|merging)\b`)
	fileNameRe = regexp.MustCompile(`^(\.?[A-Za-z0-9_\-]+(\.[A-Za-z0-9]{1,8})+|\.[A-Za-z0-9_\-]+)$`)
	wordPathRe = regexp.MustCompile(`(?:^|\s)((?:\*\*/)?\*\.[A-Za-z0-9]+|\.?[A-Za-z0-9_\-]+(?:/[A-Za-z0-9_.*\-]+)*/|[A-Za-z0-9_\-]+(?:/[A-Za-z0-9_.*\-]+)+|\.env(?:\.[a-z]+)?)(?:[\s,.;:)]|$)`)
)

// Backticked returns the contents of `code spans` in s.
func Backticked(s string) []string {
	var out []string
	for _, m := range backtickRe.FindAllStringSubmatch(s, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// LooksLikePath reports whether tok looks like a file path or glob rather
// than a command or identifier.
func LooksLikePath(tok string) bool {
	if tok == "" || strings.ContainsAny(tok, " \t'\"=<>|&;$") || strings.Contains(tok, "://") {
		return false
	}
	if strings.HasPrefix(tok, "-") {
		return false
	}
	if strings.Contains(tok, "/") || strings.HasPrefix(tok, "*.") || strings.HasPrefix(tok, "**/") {
		return true
	}
	// dotfiles and file names with a short extension: .env, package.json
	if fileNameRe.MatchString(tok) {
		return true
	}
	return false
}

// Paths returns the path-like tokens in s: backticked first, then bare words
// such as dist/, src/api/*.ts or *.tsx.
func Paths(s string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimRight(p, ".,;:)")
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, b := range Backticked(s) {
		if LooksLikePath(b) {
			add(b)
		}
	}
	stripped := backtickRe.ReplaceAllString(s, " ")
	for _, m := range wordPathRe.FindAllStringSubmatch(stripped, -1) {
		tok := m[1]
		if strings.Contains(tok, "://") || tok == "/" || strings.EqualFold(tok, "and/or") {
			continue
		}
		add(tok)
	}
	return out
}

// Commands returns backticked tokens that look like commands, plus known
// dangerous commands written without backticks.
func Commands(s string) []string {
	var out []string
	for _, b := range Backticked(s) {
		if !LooksLikePath(b) {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		if m := dangerRe.FindString(strings.ToLower(s)); m != "" {
			if strings.HasPrefix(m, "force") {
				m = "git push --force"
			}
			out = append(out, m)
		}
	}
	return out
}

// IsNegative reports whether text contains a prohibition (never, don't, ...).
func IsNegative(text string) bool { return negRe.MatchString(strings.ToLower(text)) }

// Classify detects whether an instruction is a mechanically checkable rule.
func Classify(text string) Pattern {
	n := strings.ToLower(text)
	p := Pattern{Text: text}
	neg := negRe.MatchString(n)
	switch {
	case neg && secretVerb.MatchString(n) && secretObj.MatchString(n):
		p.Kind = SecretDeny
		p.Paths = Paths(text)
	case runBefore.MatchString(n) && len(backtickCommands(text)) > 0 && !neg:
		p.Kind = RunBeforeCommit
		p.Commands = backtickCommands(text)
	case testNeedRe.MatchString(n) && len(Paths(text)) > 0:
		p.Kind = MustHaveTest
		p.Paths = Paths(text)
	case neg && (dangerRe.MatchString(n) || (runVerbRe.MatchString(n) && len(backtickCommands(text)) > 0)):
		p.Kind = CommandDeny
		p.Commands = Commands(text)
	case neg && editVerbRe.MatchString(n) && len(Paths(text)) > 0:
		p.Kind = PathDeny
		p.Paths = Paths(text)
	}
	return p
}

func backtickCommands(s string) []string {
	var out []string
	for _, b := range Backticked(s) {
		if !LooksLikePath(b) {
			out = append(out, b)
		}
	}
	return out
}
