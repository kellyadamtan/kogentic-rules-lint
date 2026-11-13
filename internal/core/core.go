// Package core holds the types shared by discovery, rules, the engine and reporters.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Severity of a finding.
type Severity int

// Severity levels, lowest to highest. SevOff disables a rule.
const (
	SevOff Severity = iota
	SevInfo
	SevWarning
	SevError
)

func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "info"
	case SevWarning:
		return "warning"
	case SevError:
		return "error"
	default:
		return "off"
	}
}

// ParseSeverity parses "off", "info", "warning" (or "warn") and "error".
func ParseSeverity(v string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "off", "none", "false":
		return SevOff, nil
	case "info", "note":
		return SevInfo, nil
	case "warning", "warn":
		return SevWarning, nil
	case "error":
		return SevError, nil
	}
	return SevOff, fmt.Errorf("unknown severity %q (use off, info, warning or error)", v)
}

// Kind classifies a discovered rules file.
type Kind string

// File kinds.
const (
	KindClaudeMemory   Kind = "claude-memory"   // CLAUDE.md, CLAUDE.local.md, .claude/CLAUDE.md
	KindClaudeImport   Kind = "claude-import"   // a file pulled in by an @path import
	KindClaudeRule     Kind = "claude-rule"     // .claude/rules/*.md
	KindAgents         Kind = "agents"          // AGENTS.md (any depth)
	KindGemini         Kind = "gemini"          // GEMINI.md
	KindCursorRule     Kind = "cursor-rule"     // .cursor/rules/**/*.mdc
	KindCursorRuleMD   Kind = "cursor-rule-md"  // .cursor/rules/**/*.md (ignored by Cursor)
	KindCursorLegacy   Kind = "cursor-legacy"   // .cursorrules
	KindCopilotRepo    Kind = "copilot-repo"    // .github/copilot-instructions.md
	KindCopilotInstr   Kind = "copilot-instr"   // .github/instructions/**/*.instructions.md
	KindWindsurfLegacy Kind = "windsurf-legacy" // .windsurfrules
	KindWindsurfRule   Kind = "windsurf-rule"   // .windsurf/rules/*.md
	KindCline          Kind = "cline"           // .clinerules (file or dir)
	KindRoo            Kind = "roo"             // .roo/rules*/
	KindExtra          Kind = "extra"           // matched by config paths.extra
)

// Frontmatter is the YAML block between leading --- lines.
type Frontmatter struct {
	Present   bool
	Raw       string
	Data      map[string]any
	Err       error
	StartLine int            // line of the opening ---
	EndLine   int            // line of the closing ---
	KeyLines  map[string]int // top-level key -> line number
}

// Line returns the line of key, or the frontmatter start line.
func (f Frontmatter) Line(key string) int {
	if l, ok := f.KeyLines[key]; ok {
		return l
	}
	if f.StartLine > 0 {
		return f.StartLine
	}
	return 1
}

// Instruction is one list item or sentence of a rules file body.
type Instruction struct {
	Text        string // as written (single line)
	Norm        string // lower-cased, emphasis stripped
	Line        int
	Section     string
	SectionLine int
}

// Import is an @path reference in a CLAUDE.md file.
type Import struct {
	Raw      string
	Resolved string // slash path relative to root (may start with ../)
	Line     int
	Exists   bool
	External bool // ~/ or absolute: not checked
}

// Directive is an inline <!-- rules-lint-... --> comment.
type Directive struct {
	Line   int
	Kind   string   // disable, enable, disable-line
	Rules  []string // empty = all rules
	Target int      // for disable-line: the line it applies to
}

// File is one discovered rules file.
type File struct {
	Path         string // slash path relative to the lint root
	Abs          string
	Tool         string // claude, agents, gemini, cursor, copilot, windsurf, cline, roo, extra
	Kind         Kind
	Lines        []string
	Size         int
	FM           Frontmatter
	BodyStart    int // 1-based first body line
	AlwaysOn     bool
	ImportedBy   string
	Imports      []Import
	Instructions []Instruction
	Directives   []Directive
}

// Tokens is a rough token estimate (bytes / 4, rounded up).
func (f *File) Tokens() int { return (f.Size + 3) / 4 }

// Finding is one reported problem.
type Finding struct {
	RuleID     string   `json:"rule"`
	RuleName   string   `json:"name"`
	Severity   Severity `json:"-"`
	Path       string   `json:"file"`
	Line       int      `json:"line"`
	Message    string   `json:"message"`
	Suggestion string   `json:"suggestion,omitempty"`
	Snippet    string   `json:"snippet,omitempty"`
}

// Context is what a rule sees.
type Context struct {
	Root      string
	Files     []*File
	RepoFiles []string // all non-ignored files, slash paths relative to Root
	Budget    int      // always-on token budget per tool
}

// Exists reports whether a slash path relative to Root exists on disk.
func (c *Context) Exists(rel string) bool {
	_, err := os.Stat(filepath.Join(c.Root, filepath.FromSlash(rel)))
	return err == nil
}

// Rule is one lint rule.
type Rule struct {
	ID      string
	Name    string
	Default Severity
	Summary string
	Check   func(c *Context) []Finding
}

// Snip trims s to at most 200 characters for reports.
func Snip(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 200 {
		return string(r[:197]) + "..."
	}
	return s
}
