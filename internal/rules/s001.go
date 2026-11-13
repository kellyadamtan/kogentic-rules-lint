package rules

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/patterns"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	s = strings.Trim(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		s = "scoped-rule"
	}
	return s
}

// area maps a path reference to the area it belongs to and the glob that
// would scope a rule to it: "*.tsx" -> ("ext .tsx", "**/*.tsx"),
// "migrations/001.sql" -> ("dir migrations", "migrations/**").
func area(ref string) (key, glob string, ok bool) {
	ref = strings.TrimPrefix(strings.TrimPrefix(ref, "./"), "/")
	if strings.HasPrefix(ref, "../") || ref == "" {
		return "", "", false
	}
	if i := strings.Index(ref, "/"); i > 0 && !strings.ContainsAny(ref[:i], "*?") {
		dir := ref[:i]
		return "dir " + dir, dir + "/**", true
	}
	base := path.Base(ref)
	if ext := path.Ext(base); ext != "" && len(ext) <= 8 && !strings.HasPrefix(base, ".") {
		return "ext " + ext, "**/*" + ext, true
	}
	return "", "", false
}

type segment struct {
	title string
	line  int
	ins   []core.Instruction
}

// segments splits always-on root files by heading; scoped-capable rule files
// are judged as a whole.
func segments(f *core.File) []segment {
	switch f.Kind {
	case core.KindCursorRule, core.KindClaudeRule, core.KindCopilotInstr, core.KindWindsurfRule:
		return []segment{{title: path.Base(f.Path), line: 1, ins: f.Instructions}}
	}
	var out []segment
	idx := map[int]int{}
	for _, in := range f.Instructions {
		k, ok := idx[in.SectionLine]
		if !ok {
			k = len(out)
			idx[in.SectionLine] = k
			out = append(out, segment{title: in.Section, line: in.SectionLine})
		}
		out[k].ins = append(out[k].ins, in)
	}
	return out
}

func scopeSuggestion(f *core.File, seg segment, glob string) string {
	name := slug(seg.title)
	switch f.Kind {
	case core.KindCursorRule:
		return fmt.Sprintf("Set frontmatter `globs: %s` and `alwaysApply: false`.", glob)
	case core.KindCursorLegacy:
		return fmt.Sprintf("Move it to .cursor/rules/%s.mdc with `globs: %s` and `alwaysApply: false`.", name, glob)
	case core.KindClaudeRule:
		return fmt.Sprintf("Add frontmatter `paths: [\"%s\"]`.", glob)
	case core.KindClaudeMemory, core.KindClaudeImport:
		return fmt.Sprintf("Move this section to .claude/rules/%s.md with frontmatter `paths: [\"%s\"]`.", name, glob)
	case core.KindCopilotInstr:
		return fmt.Sprintf("Set frontmatter `applyTo: \"%s\"`.", glob)
	case core.KindCopilotRepo:
		return fmt.Sprintf("Move this section to .github/instructions/%s.instructions.md with `applyTo: \"%s\"`.", name, glob)
	case core.KindWindsurfRule:
		return fmt.Sprintf("Set frontmatter `trigger: glob` and `globs: %s`.", glob)
	case core.KindWindsurfLegacy:
		return fmt.Sprintf("Move it to .windsurf/rules/%s.md with `trigger: glob` and `globs: %s`.", name, glob)
	case core.KindAgents:
		if strings.HasSuffix(glob, "/**") {
			return fmt.Sprintf("Move this section to %s/AGENTS.md so it only loads for that folder.", strings.TrimSuffix(glob, "/**"))
		}
	}
	return fmt.Sprintf("Scope it to `%s` with your tool's glob frontmatter instead of loading it every session.", glob)
}

func init() {
	r := &core.Rule{ID: "S001", Name: "always-on-should-auto-attach", Default: core.SevWarning,
		Summary: "Always-on content that only talks about one path area or file type."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			if !f.AlwaysOn {
				continue
			}
			for _, seg := range segments(f) {
				var keys = map[string]string{}
				refs := 0
				for _, in := range seg.ins {
					for _, p := range patterns.Paths(in.Text) {
						k, g, ok := area(p)
						if !ok {
							continue
						}
						keys[k] = g
						refs++
					}
				}
				if refs < 2 || len(keys) != 1 {
					continue
				}
				for k, g := range keys {
					where := "This file"
					if seg.title != "" && seg.line > 0 && len(segments(f)) > 1 {
						where = fmt.Sprintf("Section %q", seg.title)
					}
					out = append(out, finding(r, f, seg.line, fmt.Sprintf("%s is loaded every session but only refers to %s (%d references).", where, strings.TrimPrefix(strings.TrimPrefix(k, "dir "), "ext "), refs), scopeSuggestion(f, seg, g), seg.title))
				}
			}
		}
		return out
	}
	register(r)
}
