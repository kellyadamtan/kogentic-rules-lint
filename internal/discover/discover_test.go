package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]core.Kind{
		"CLAUDE.md":                                core.KindClaudeMemory,
		"CLAUDE.local.md":                          core.KindClaudeMemory,
		".claude/CLAUDE.md":                        core.KindClaudeMemory,
		".claude/rules/db.md":                      core.KindClaudeRule,
		"AGENTS.md":                                core.KindAgents,
		"pkg/api/AGENTS.md":                        core.KindAgents,
		"GEMINI.md":                                core.KindGemini,
		".cursor/rules/a.mdc":                      core.KindCursorRule,
		".cursor/rules/sub/b.mdc":                  core.KindCursorRule,
		".cursor/rules/c.md":                       core.KindCursorRuleMD,
		".cursorrules":                             core.KindCursorLegacy,
		".github/copilot-instructions.md":          core.KindCopilotRepo,
		".github/instructions/x/y.instructions.md": core.KindCopilotInstr,
		".windsurfrules":                           core.KindWindsurfLegacy,
		".windsurf/rules/a.md":                     core.KindWindsurfRule,
		".clinerules":                              core.KindCline,
		".clinerules/01-style.md":                  core.KindCline,
		".roo/rules/a.md":                          core.KindRoo,
		".roo/rules-code/a.md":                     core.KindRoo,
	}
	for rel, want := range cases {
		if _, got, ok := Classify(rel); !ok || got != want {
			t.Errorf("Classify(%q) = %q, %v; want %q", rel, got, ok, want)
		}
	}
	for _, rel := range []string{"README.md", "docs/CLAUDE.md", "sub/GEMINI.md", ".windsurf/rules/x/y.md", ".github/instructions/a.md", ".roo/other/a.md", "src/x.mdc"} {
		if _, k, ok := Classify(rel); ok {
			t.Errorf("Classify(%q) should not match, got %q", rel, k)
		}
	}
}

const claudeMD = "# R\n\n- Rule one.\n\nSee @docs/shared.md and @docs/missing.md and @~/.claude/me.md\n```\n@docs/in-fence.md\n```\nInline `@docs/code.md` is not an import.\n"

func TestDiscoverGitignoreImportsAndAlwaysOn(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		".gitignore":               "build/\n*.log\n",
		"CLAUDE.md":                claudeMD,
		"docs/shared.md":           "- Shared rule.\n",
		"docs/in-fence.md":         "x\n",
		"build/AGENTS.md":          "- ignored by .gitignore\n",
		"pkg/AGENTS.md":            "- nested\n",
		"pkg/.gitignore":           "gen/\n",
		"pkg/gen/AGENTS.md":        "- ignored by nested .gitignore\n",
		"AGENTS.md":                "- root\n",
		"debug.log":                "x\n",
		".cursor/rules/on.mdc":     "---\nalwaysApply: true\n---\n- on\n",
		".cursor/rules/off.mdc":    "---\nglobs: *.go\n---\n- off\n",
		".claude/rules/scoped.md":  "---\npaths: [\"src/**\"]\n---\n- scoped\n",
		".claude/rules/always.md":  "- always\n",
		"node_modules/x/AGENTS.md": "- never walked\n",
	})
	write(t, root, map[string]string{
		".github/instructions/all.instructions.md": "---\napplyTo: \"**\"\n---\n- all\n",
		".windsurf/rules/w.md":                     "---\ntrigger: model_decision\n---\n- w\n",
	})
	res, err := Discover(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*core.File{}
	for _, f := range res.Files {
		got[f.Path] = f
	}
	for _, p := range []string{"build/AGENTS.md", "pkg/gen/AGENTS.md", "node_modules/x/AGENTS.md", "docs/in-fence.md"} {
		if got[p] != nil {
			t.Errorf("%s should not be discovered", p)
		}
	}
	for _, rf := range res.RepoFiles {
		if strings.HasSuffix(rf, ".log") || strings.HasPrefix(rf, "build/") {
			t.Errorf("ignored file in RepoFiles: %s", rf)
		}
	}
	imp := got["docs/shared.md"]
	if imp == nil || imp.Kind != core.KindClaudeImport || imp.ImportedBy != "CLAUDE.md" || !imp.AlwaysOn {
		t.Fatalf("import not followed correctly: %+v", imp)
	}
	c := got["CLAUDE.md"]
	if len(c.Imports) != 3 {
		t.Fatalf("want 3 imports (shared, missing, ~), got %+v", c.Imports)
	}
	if !c.Imports[0].Exists || c.Imports[1].Exists || !c.Imports[2].External {
		t.Errorf("import flags wrong: %+v", c.Imports)
	}
	always := map[string]bool{
		"CLAUDE.md": true, "AGENTS.md": true, "pkg/AGENTS.md": false,
		".cursor/rules/on.mdc": true, ".cursor/rules/off.mdc": false,
		".claude/rules/scoped.md": false, ".claude/rules/always.md": true,
		".github/instructions/all.instructions.md": true, ".windsurf/rules/w.md": false,
	}
	for p, want := range always {
		f := got[p]
		if f == nil {
			t.Errorf("%s not discovered", p)
			continue
		}
		if f.AlwaysOn != want {
			t.Errorf("%s AlwaysOn = %v, want %v", p, f.AlwaysOn, want)
		}
	}
	if off := got[".cursor/rules/off.mdc"]; off.FM.Err != nil || off.FM.Data["globs"] != "*.go" {
		t.Errorf("unquoted cursor glob should parse: %v %v", off.FM.Err, off.FM.Data)
	}
}

func TestDiscoverExtraAndIgnoreConfig(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"docs/agent-rules/a.md": "- extra\n",
		"vendor/AGENTS.md":      "- skip\n",
		"AGENTS.md":             "- root\n",
	})
	res, err := Discover(root, []string{"docs/agent-rules/*.md"}, []string{"vendor/**"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path+"="+string(f.Kind))
	}
	if strings.Join(paths, ",") != "AGENTS.md=agents,docs/agent-rules/a.md=extra" {
		t.Errorf("got %v", paths)
	}
}

func TestDiscoverNotADir(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{"f": "x"})
	if _, err := Discover(filepath.Join(root, "f"), nil, nil); err == nil {
		t.Error("expected error for a file root")
	}
	if _, err := Discover(filepath.Join(root, "nope"), nil, nil); err == nil {
		t.Error("expected error for a missing root")
	}
}

func TestExtract(t *testing.T) {
	src := `---
alwaysApply: true
---
# Title

Intro sentence one. Second sentence, e.g. like this. Third!
- item one
  continued here
- [ ] task item
<!-- a comment that is not an instruction -->
| a | table |
` + "```" + `
code is ignored
` + "```" + `
<!-- rules-lint-disable-next-line V001 -->

- target line
- trailing <!-- rules-lint-disable V003 -->
<!-- rules-lint-disable -->
<!-- rules-lint-enable V002 -->
<!--
multi-line comment
-->
## Section two
> quoted rule
`
	lines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	fm, start := parseFrontmatter(lines, core.KindCursorRule)
	if !fm.Present || start != 4 || fm.Data["alwaysApply"] != true || fm.KeyLines["alwaysApply"] != 2 {
		t.Fatalf("frontmatter: %+v start=%d", fm, start)
	}
	ins, dirs := Extract(lines, start)
	var texts []string
	for _, in := range ins {
		texts = append(texts, in.Text)
	}
	want := []string{"Intro sentence one.", "Second sentence, e.g. like this.", "Third!", "item one continued here", "task item", "target line", "trailing", "quoted rule"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("instructions:\n got %q\nwant %q", texts, want)
	}
	if ins[0].Section != "Title" || ins[0].SectionLine != 4 || ins[7].Section != "Section two" {
		t.Errorf("sections wrong: %+v / %+v", ins[0], ins[7])
	}
	if len(dirs) != 4 {
		t.Fatalf("want 4 directives, got %+v", dirs)
	}
	if dirs[0].Kind != "disable-line" || dirs[0].Target != 17 || dirs[0].Rules[0] != "V001" {
		t.Errorf("next-line directive: %+v", dirs[0])
	}
	if dirs[1].Kind != "disable-line" || dirs[1].Target != 18 {
		t.Errorf("trailing directive: %+v", dirs[1])
	}
	if dirs[2].Kind != "disable" || len(dirs[2].Rules) != 0 || dirs[3].Kind != "enable" {
		t.Errorf("range directives: %+v", dirs[2:])
	}
}

func TestFrontmatterErrors(t *testing.T) {
	fm, _ := parseFrontmatter([]string{"---", "a: [unclosed", "---", "x"}, core.KindCursorRule)
	if fm.Err == nil {
		t.Error("expected YAML error")
	}
	fm, _ = parseFrontmatter([]string{"---", "- a list", "---"}, core.KindClaudeRule)
	if fm.Err == nil {
		t.Error("expected non-mapping error")
	}
	fm, start := parseFrontmatter([]string{"---", "no closing"}, core.KindClaudeRule)
	if fm.Present || start != 1 {
		t.Error("unterminated frontmatter is body")
	}
}
