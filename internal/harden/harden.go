// Package harden turns a correction you keep repeating into an enforced
// rule file, hook or test stub, from a small library of templates. No LLM.
package harden

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/patterns"
)

// Change is one file harden would create or modify.
type Change struct {
	Path string // slash path relative to the repo root
	Old  string // current content ("" if new)
	New  string
	Mode os.FileMode
}

// Options for Plan.
type Options struct {
	Root   string
	Text   string // the correction
	As     string // rule, hook, test, or "" for automatic
	Target string // claude or cursor
}

// ErrUnsupported is returned when the template library cannot express the
// correction in the requested form.
var ErrUnsupported = errors.New("unsupported")

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug makes a file-name-safe slug from the first words of text.
func Slug(text string) string {
	ws := strings.Fields(strings.Trim(slugRe.ReplaceAllString(strings.ToLower(text), " "), " "))
	if len(ws) > 6 {
		ws = ws[:6]
	}
	s := strings.Join(ws, "-")
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	if s == "" {
		s = "hardened-rule"
	}
	return s
}

// Plan works out the changes for opts without touching the disk.
func Plan(opts Options) ([]Change, patterns.Pattern, error) {
	text := strings.TrimSpace(opts.Text)
	if text == "" {
		return nil, patterns.Pattern{}, fmt.Errorf("%w: empty correction text", ErrUnsupported)
	}
	if opts.Target == "" {
		opts.Target = "claude"
	}
	if opts.Target != "claude" && opts.Target != "cursor" {
		return nil, patterns.Pattern{}, fmt.Errorf("%w: --target must be claude or cursor", ErrUnsupported)
	}
	p := patterns.Classify(text)
	as := opts.As
	if as == "" {
		switch p.Kind {
		case patterns.None:
			as = "rule"
		case patterns.MustHaveTest:
			as = "test"
		default:
			as = "hook"
		}
	}
	slug := Slug(text)
	var changes []Change
	var err error
	switch as {
	case "rule":
		changes = []Change{ruleFile(opts.Target, slug, text, p)}
	case "hook":
		changes, err = hookFiles(opts.Root, opts.Target, slug, text, p)
	case "test":
		var c Change
		c, err = testFile(slug, text, p)
		changes = []Change{c}
	default:
		return nil, p, fmt.Errorf("%w: --as must be rule, hook or test", ErrUnsupported)
	}
	if err != nil {
		return nil, p, err
	}
	for i := range changes {
		if b, rerr := os.ReadFile(filepath.Join(opts.Root, filepath.FromSlash(changes[i].Path))); rerr == nil && changes[i].Old == "" {
			changes[i].Old = string(b)
		}
	}
	return changes, p, nil
}

// Apply writes the changes under root.
func Apply(root string, changes []Change) error {
	for _, c := range changes {
		p := filepath.Join(root, filepath.FromSlash(c.Path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		mode := c.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(p, []byte(c.New), mode); err != nil {
			return err
		}
		if err := os.Chmod(p, mode); err != nil {
			return err
		}
	}
	return nil
}

func scopeGlobs(p patterns.Pattern) []string {
	var out []string
	for _, ps := range p.Paths {
		ps = strings.TrimPrefix(ps, "./")
		switch {
		case strings.HasSuffix(ps, "/"):
			out = append(out, ps+"**")
		case strings.ContainsAny(ps, "*?"):
			out = append(out, ps)
		case strings.Contains(ps, "/"):
			out = append(out, ps)
		default:
			out = append(out, "**/"+ps)
		}
	}
	return out
}

func codeList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = "`" + x + "`"
	}
	return strings.Join(q, ", ")
}

func ruleBody(text string, p patterns.Pattern) string {
	switch p.Kind {
	case patterns.PathDeny:
		return fmt.Sprintf("Do not create, edit or delete files under %s. If a change seems to need it, change the source that generates them instead.\n\nVerify: `git status --short` lists nothing under %s.", codeList(p.Paths), codeList(p.Paths))
	case patterns.CommandDeny:
		return fmt.Sprintf("Never run %s. If you think you need it, stop and ask the user.\n\nVerify: no tool call in this session contains %s.", codeList(p.Commands), codeList(p.Commands))
	case patterns.SecretDeny:
		return "Never read, print, stage or commit secret files (`.env`, `.env.*`, `*.pem`, `id_rsa`, credentials files). Use the example file (for instance `.env.example`) for documentation.\n\nVerify: `git diff --cached --name-only` lists no secret files."
	case patterns.MustHaveTest:
		return fmt.Sprintf("Every new or changed file matching %s needs a matching test file (`<name>_test.<ext>`, `<name>.test.<ext>` or `<name>.spec.<ext>`).\n\nVerify: each changed file has its test file, and the test suite passes.", codeList(p.Paths))
	case patterns.RunBeforeCommit:
		return fmt.Sprintf("Before every commit, run %s and fix any failure first.\n\nVerify: %s exits 0 on the commit you make.", codeList(p.Commands), codeList(p.Commands))
	}
	return fmt.Sprintf("When <situation>, %s\n\nVerify by <check>.\n\n<!-- Fill in the situation and the check so the agent can follow and prove it. -->", lowerFirst(text))
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if len(r) > 1 && r[1] >= 'a' && r[1] <= 'z' {
		r[0] = []rune(strings.ToLower(string(r[0])))[0]
	}
	return string(r)
}

func yamlQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func ruleFile(target, slug, text string, p patterns.Pattern) Change {
	globs := scopeGlobs(p)
	if p.Kind == patterns.SecretDeny || p.Kind == patterns.CommandDeny || p.Kind == patterns.RunBeforeCommit {
		globs = nil // these apply everywhere
	}
	body := ruleBody(text, p)
	title := "# " + strings.TrimSpace(text) + "\n\n"
	if target == "cursor" {
		fm := "---\ndescription: " + yamlQuote(text) + "\n"
		if len(globs) > 0 {
			fm += "globs: " + strings.Join(globs, ",") + "\nalwaysApply: false\n"
		} else {
			fm += "alwaysApply: true\n"
		}
		fm += "---\n"
		return Change{Path: ".cursor/rules/" + slug + ".mdc", New: fm + title + body + "\n"}
	}
	fm := ""
	if len(globs) > 0 {
		fm = "---\npaths:\n"
		for _, g := range globs {
			fm += "  - " + yamlQuote(g) + "\n"
		}
		fm += "---\n"
	}
	return Change{Path: ".claude/rules/" + slug + ".md", New: fm + title + body + "\n"}
}

// shQuote single-quotes s for POSIX sh.
func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// ereEscape escapes s for a POSIX extended regular expression.
func ereEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`\.^$|?*+()[]{}`, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// globERE converts a path or glob to an ERE fragment that matches it inside
// a JSON string value.
func globERE(g string) string {
	g = strings.TrimPrefix(g, "./")
	dir := strings.HasSuffix(g, "/")
	var b strings.Builder
	for i := 0; i < len(g); i++ {
		switch {
		case strings.HasPrefix(g[i:], "**/"):
			b.WriteString(`([^"]*/)?`)
			i += 2
		case strings.HasPrefix(g[i:], "**"):
			b.WriteString(`[^"]*`)
			i++
		case g[i] == '*':
			b.WriteString(`[^"/]*`)
		case g[i] == '?':
			b.WriteString(`[^"/]`)
		default:
			b.WriteString(ereEscape(string(g[i])))
		}
	}
	if !dir && !strings.ContainsAny(g, "*?") {
		b.WriteString(`("|/)`)
	} else if !dir {
		b.WriteString(`"`)
	}
	return `([^"]*/)?` + b.String()
}

func caseGlob(g string) string {
	g = strings.TrimPrefix(g, "./")
	g = strings.ReplaceAll(g, "**/", "*")
	g = strings.ReplaceAll(g, "**", "*")
	if strings.HasSuffix(g, "/") {
		g += "*"
	}
	return g
}

const secretERE = `(^|[^A-Za-z0-9_.])(\.env(\.[A-Za-z0-9_-]+)?|[^"/[:space:]]*\.pem|id_rsa|id_ed25519|credentials\.json)("|[[:space:]]|$)`
const secretAllowERE = `\.env\.(example|sample|template)`

// blockLines returns the shell snippet that blocks, for each target.
func block(target, msg string) string {
	if target == "cursor" {
		b, _ := json.Marshal(map[string]string{"permission": "deny", "userMessage": msg, "agentMessage": msg})
		return "  printf '%s\\n' " + shQuote(string(b)) + "\n  exit 0\n"
	}
	return "  echo " + shQuote(msg) + " >&2\n  exit 2\n"
}

func allow(target string) string {
	if target == "cursor" {
		return "printf '%s\\n' '{\"permission\":\"allow\"}'\nexit 0\n"
	}
	return "exit 0\n"
}

// indent adds two spaces to every line of a script fragment.
func indent(frag string) string {
	lines := strings.SplitAfter(frag, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "")
}

// checkFailed blocks when grep itself fails (exit status above 1, for example
// a bad pattern or a missing grep), so a broken hook fails closed instead of
// letting the action through.
func checkFailed(target, text string) string {
	return "if [ \"$rc\" -gt 1 ]; then\n" + block(target, "Blocked: the hook for this rule could not run its check (grep failed). Fix the hook script. Rule: "+strings.TrimSpace(text)) + "fi\n"
}

// grepBlock blocks when re matches the hook input, and fails closed when grep
// errors.
func grepBlock(target, text, re, msg string) string {
	return "printf '%s' \"$input\" | grep -Eq " + shQuote(re) + "\nrc=$?\n" +
		"if [ \"$rc\" -eq 0 ]; then\n" + block(target, msg) + "fi\n" + checkFailed(target, text)
}

// onlyGitCommit lets every command except `git commit` through, and fails
// closed when grep errors.
func onlyGitCommit(target, text, re string) string {
	return "printf '%s' \"$input\" | grep -Eq " + shQuote(re) + "\nrc=$?\n" + checkFailed(target, text) +
		"if [ \"$rc\" -ne 0 ]; then\n" + indent(allow(target)) + "fi\n"
}

func header(target, text string) string {
	h := "#!/bin/sh\n# Generated by kogentic-rules-lint harden from the correction:\n#   " + strings.ReplaceAll(text, "\n", " ") + "\n"
	if target == "cursor" {
		h += "# Cursor hook: reads the event JSON on stdin and prints a permission decision.\n"
	} else {
		h += "# Claude Code PreToolUse hook: reads the tool call JSON on stdin; exit 2 blocks it\n# and shows the message to Claude.\n"
	}
	return h + "input=$(cat)\n"
}

func hookScript(target, text string, p patterns.Pattern) (script, matcher string, cursorEvents []string, err error) {
	msg := "Blocked by project rule: " + strings.TrimSpace(text)
	cmdField := `"command"[[:space:]]*:[[:space:]]*"[^"]*`
	switch p.Kind {
	case patterns.PathDeny:
		if target == "cursor" {
			return "", "", nil, fmt.Errorf("%w: Cursor hooks cannot block file edits in v0.1; use --as rule or --target claude", ErrUnsupported)
		}
		var alts []string
		for _, ps := range p.Paths {
			alts = append(alts, globERE(ps))
		}
		re := `"(file_path|notebook_path)"[[:space:]]*:[[:space:]]*"(` + strings.Join(alts, "|") + `)`
		script = header(target, text) + grepBlock(target, text, re, msg) + allow(target)
		return script, "Edit|Write|MultiEdit|NotebookEdit", nil, nil
	case patterns.CommandDeny:
		var alts []string
		for _, c := range p.Commands {
			alts = append(alts, ereEscape(c))
		}
		re := cmdField + "(" + strings.Join(alts, "|") + ")"
		script = header(target, text) + grepBlock(target, text, re, msg) + allow(target)
		return script, "Bash", []string{"beforeShellExecution"}, nil
	case patterns.SecretDeny:
		script = header(target, text) +
			"found=$(printf '%s' \"$input\" | grep -Eo " + shQuote(secretERE) + ")\nrc=$?\n" +
			"if [ \"$rc\" -eq 0 ]; then\n" +
			"  printf '%s\\n' \"$found\" | grep -Ev " + shQuote(secretAllowERE) + " >/dev/null\n  rc=$?\n" +
			"  if [ \"$rc\" -eq 0 ]; then\n" + indent(block(target, msg)) + "  fi\nfi\n" +
			checkFailed(target, text) + allow(target)
		return script, "Read|Edit|Write|MultiEdit|Bash", []string{"beforeShellExecution", "beforeReadFile"}, nil
	case patterns.RunBeforeCommit:
		cmd := p.Commands[0]
		script = header(target, text) +
			onlyGitCommit(target, text, cmdField+`git commit`) +
			"cd \"${CLAUDE_PROJECT_DIR:-.}\" || exit 2\n" +
			"if ! ( " + cmd + " ) >&2; then\n" + block(target, "Blocked: `"+cmd+"` failed. Fix it before committing.") + "fi\n" + allow(target)
		return script, "Bash", []string{"beforeShellExecution"}, nil
	case patterns.MustHaveTest:
		var cases []string
		for _, ps := range p.Paths {
			cases = append(cases, caseGlob(ps))
		}
		script = header(target, text) +
			onlyGitCommit(target, text, cmdField+`git commit`) +
			"missing=\"\"\n" +
			"for f in $(git diff --cached --name-only --diff-filter=AM); do\n" +
			"  case \"$f\" in\n    " + strings.Join(cases, "|") + ") ;;\n    *) continue ;;\n  esac\n" +
			"  case \"$f\" in *_test.*|*.test.*|*.spec.*) continue ;; esac\n" +
			"  dir=$(dirname \"$f\"); base=$(basename \"$f\"); stem=${base%.*}; ext=${base##*.}\n" +
			"  found=0\n" +
			"  for t in \"$dir/${stem}_test.$ext\" \"$dir/${stem}.test.$ext\" \"$dir/${stem}.spec.$ext\"; do\n" +
			"    [ -e \"$t\" ] && found=1\n  done\n" +
			"  [ \"$found\" = 1 ] || missing=\"$missing $f\"\n" +
			"done\n" +
			"if [ -n \"$missing\" ]; then\n" + block(target, "Blocked: add tests for the staged files first (rule: "+strings.TrimSpace(text)+")") + "fi\n" + allow(target)
		return script, "Bash", []string{"beforeShellExecution"}, nil
	}
	return "", "", nil, fmt.Errorf("%w: no hook template matches this correction (path deny-list, command deny-list, secret-file deny, must-have-test, run-before-commit); use --as rule", ErrUnsupported)
}

func readJSON(root, rel string) (map[string]any, string, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	m := map[string]any{}
	if len(strings.TrimSpace(string(b))) > 0 {
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, "", fmt.Errorf("%s is not valid JSON: %w", rel, err)
		}
	}
	return m, string(b), nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func asList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func hasCommand(list []any, cmd string) bool {
	for _, e := range list {
		m := asMap(e)
		if m["command"] == cmd {
			return true
		}
		for _, h := range asList(m["hooks"]) {
			if asMap(h)["command"] == cmd {
				return true
			}
		}
	}
	return false
}

func marshal(m map[string]any) string {
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b) + "\n"
}

func hookFiles(root, target, slug, text string, p patterns.Pattern) ([]Change, error) {
	script, matcher, events, err := hookScript(target, text, p)
	if err != nil {
		return nil, err
	}
	if target == "cursor" {
		scriptPath := ".cursor/hooks/" + slug + ".sh"
		cfg, old, err := readJSON(root, ".cursor/hooks.json")
		if err != nil {
			return nil, err
		}
		if _, ok := cfg["version"]; !ok {
			cfg["version"] = 1
		}
		hooks := asMap(cfg["hooks"])
		for _, ev := range events {
			list := asList(hooks[ev])
			if !hasCommand(list, scriptPath) {
				list = append(list, map[string]any{"command": scriptPath})
			}
			hooks[ev] = list
		}
		cfg["hooks"] = hooks
		return []Change{
			{Path: scriptPath, New: script, Mode: 0o755},
			{Path: ".cursor/hooks.json", Old: old, New: marshal(cfg)},
		}, nil
	}
	scriptPath := ".claude/hooks/" + slug + ".sh"
	command := `"$CLAUDE_PROJECT_DIR"/` + scriptPath
	cfg, old, err := readJSON(root, ".claude/settings.json")
	if err != nil {
		return nil, err
	}
	hooks := asMap(cfg["hooks"])
	list := asList(hooks["PreToolUse"])
	if !hasCommand(list, command) {
		list = append(list, map[string]any{
			"matcher": matcher,
			"hooks":   []any{map[string]any{"type": "command", "command": command}},
		})
	}
	hooks["PreToolUse"] = list
	cfg["hooks"] = hooks
	return []Change{
		{Path: scriptPath, New: script, Mode: 0o755},
		{Path: ".claude/settings.json", Old: old, New: marshal(cfg)},
	}, nil
}

func testFile(slug, text string, p patterns.Pattern) (Change, error) {
	head := "#!/bin/sh\n# Generated by kogentic-rules-lint harden from the correction:\n#   " + strings.ReplaceAll(text, "\n", " ") +
		"\n# Run it in CI. It compares against BASE_REF (default origin/main) and fails if\n# that ref is missing, so check out full history (actions/checkout with\n# fetch-depth: 0) or set BASE_REF.\nset -eu\nbase=\"${BASE_REF:-origin/main}\"\n"
	// needBase fails the stub when the base ref can't be resolved, and
	// captures the diff on its own line so set -e stops on a git error.
	// Without it a shallow checkout would diff nothing and print ok.
	needBase := "if ! git rev-parse --verify -q \"$base^{commit}\" >/dev/null; then\n" +
		"  echo \"FAIL: base ref $base not found. Check out full history (actions/checkout with fetch-depth: 0) or set BASE_REF.\" >&2\n" +
		"  exit 1\nfi\n"
	var body string
	switch p.Kind {
	case patterns.PathDeny:
		var cases []string
		for _, ps := range p.Paths {
			cases = append(cases, caseGlob(ps))
		}
		body = needBase + "changed=$(git diff --name-only \"$base\"...HEAD)\nbad=\"\"\nfor f in $changed; do\n  case \"$f\" in\n    " + strings.Join(cases, "|") +
			") bad=\"$bad $f\" ;;\n  esac\ndone\nif [ -n \"$bad\" ]; then\n  echo " + shQuote("FAIL ("+text+"): changed") + " \"$bad\" >&2\n  exit 1\nfi\necho ok\n"
	case patterns.SecretDeny:
		body = "bad=$(git ls-files | grep -E " + shQuote(`(^|/)(\.env(\.[A-Za-z0-9_-]+)?|[^/]*\.pem|id_rsa|id_ed25519|credentials\.json)$`) + " | grep -Ev " + shQuote(secretAllowERE) + " || true)\n" +
			"if [ -n \"$bad\" ]; then\n  echo 'FAIL: secret files are tracked:' >&2\n  echo \"$bad\" >&2\n  exit 1\nfi\necho ok\n"
	case patterns.MustHaveTest:
		var cases []string
		for _, ps := range p.Paths {
			cases = append(cases, caseGlob(ps))
		}
		body = needBase + "changed=$(git diff --name-only --diff-filter=AM \"$base\"...HEAD)\nmissing=\"\"\nfor f in $changed; do\n  case \"$f\" in\n    " + strings.Join(cases, "|") +
			") ;;\n    *) continue ;;\n  esac\n  case \"$f\" in *_test.*|*.test.*|*.spec.*) continue ;; esac\n" +
			"  dir=$(dirname \"$f\"); base_name=$(basename \"$f\"); stem=${base_name%.*}; ext=${base_name##*.}\n  found=0\n" +
			"  for t in \"$dir/${stem}_test.$ext\" \"$dir/${stem}.test.$ext\" \"$dir/${stem}.spec.$ext\"; do\n    [ -e \"$t\" ] && found=1\n  done\n" +
			"  [ \"$found\" = 1 ] || missing=\"$missing $f\"\ndone\nif [ -n \"$missing\" ]; then\n  echo 'FAIL: these files have no test:' \"$missing\" >&2\n  exit 1\nfi\necho ok\n"
	default:
		return Change{}, fmt.Errorf("%w: a static test cannot check this correction (%s); use --as hook or --as rule", ErrUnsupported, p.Kind)
	}
	return Change{Path: path.Join(".kogentic", "tests", slug+".sh"), New: head + body, Mode: 0o755}, nil
}
