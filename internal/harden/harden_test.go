package harden

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/patterns"
)

func plan(t *testing.T, root, text, as, target string) []Change {
	t.Helper()
	ch, _, err := Plan(Options{Root: root, Text: text, As: as, Target: target})
	if err != nil {
		t.Fatalf("Plan(%q, %s, %s): %v", text, as, target, err)
	}
	return ch
}

// runHook executes a generated hook script with input on stdin and returns
// its exit code and output.
func runHook(t *testing.T, script, input string, env ...string) (int, string) {
	t.Helper()
	skipNoSh(t)
	p := filepath.Join(t.TempDir(), "hook.sh")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", p)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, string(out)
}

func skipNoSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("generated hooks are POSIX sh scripts")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
}

func toolJSON(field, value string) string {
	b, _ := json.Marshal(map[string]any{"tool_name": "X", "tool_input": map[string]string{field: value}})
	return string(b)
}

func TestSlug(t *testing.T) {
	if s := Slug("Never edit files in `dist/`!"); s != "never-edit-files-in-dist" {
		t.Errorf("slug %q", s)
	}
	if s := Slug("!!!"); s != "hardened-rule" {
		t.Errorf("slug %q", s)
	}
}

func TestPathDenyClaudeHookBlocks(t *testing.T) {
	ch := plan(t, t.TempDir(), "Never edit files in `dist/`", "", "claude")
	if len(ch) != 2 || ch[0].Path != ".claude/hooks/never-edit-files-in-dist.sh" || ch[1].Path != ".claude/settings.json" {
		t.Fatalf("changes: %+v", ch)
	}
	if code, out := runHook(t, ch[0].New, toolJSON("file_path", "/repo/dist/app.js")); code != 2 || !strings.Contains(out, "Blocked") {
		t.Errorf("dist edit should be blocked: %d %s", code, out)
	}
	if code, _ := runHook(t, ch[0].New, toolJSON("file_path", "/repo/src/dist.go")); code != 0 {
		t.Errorf("src/dist.go should pass: %d", code)
	}
	if code, _ := runHook(t, ch[0].New, toolJSON("file_path", "/repo/distro/x")); code != 0 {
		t.Errorf("distro/ should pass: %d", code)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(ch[1].New), &settings); err != nil {
		t.Fatal(err)
	}
	pre := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	entry := pre[0].(map[string]any)
	if entry["matcher"] != "Edit|Write|MultiEdit|NotebookEdit" {
		t.Errorf("matcher: %v", entry["matcher"])
	}
}

func TestSettingsMergeIsIdempotent(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	existing := `{"permissions":{"allow":["Bash(ls)"]},"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`
	_ = os.WriteFile(filepath.Join(root, ".claude/settings.json"), []byte(existing), 0o644)
	ch := plan(t, root, "Don't run `rm -rf`", "hook", "claude")
	if ch[1].Old != existing {
		t.Error("Old should hold the existing settings")
	}
	var s map[string]any
	_ = json.Unmarshal([]byte(ch[1].New), &s)
	if s["permissions"] == nil || len(s["hooks"].(map[string]any)["PreToolUse"].([]any)) != 2 {
		t.Fatalf("merge lost data: %s", ch[1].New)
	}
	if err := Apply(root, ch); err != nil {
		t.Fatal(err)
	}
	ch2 := plan(t, root, "Don't run `rm -rf`", "hook", "claude")
	_ = json.Unmarshal([]byte(ch2[1].New), &s)
	if n := len(s["hooks"].(map[string]any)["PreToolUse"].([]any)); n != 2 {
		t.Errorf("re-running harden should not add a duplicate hook, got %d entries", n)
	}
	if !strings.Contains(Diff(ch2[1]), "(no change)") {
		t.Errorf("diff of identical file: %s", Diff(ch2[1]))
	}
}

func TestCommandDenyBothTargets(t *testing.T) {
	text := "Don't run `rm -rf` or `git push --force`"
	ch := plan(t, t.TempDir(), text, "hook", "claude")
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "cd x && rm -rf build")); code != 2 {
		t.Errorf("rm -rf should be blocked: %d", code)
	}
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "git push --force origin x")); code != 2 {
		t.Errorf("force push should be blocked: %d", code)
	}
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "rm build.log")); code != 0 {
		t.Errorf("plain rm should pass: %d", code)
	}
	cur := plan(t, t.TempDir(), text, "hook", "cursor")
	if cur[0].Path != ".cursor/hooks/don-t-run-rm-rf-or.sh" || cur[1].Path != ".cursor/hooks.json" {
		t.Fatalf("cursor paths: %s %s", cur[0].Path, cur[1].Path)
	}
	code, out := runHook(t, cur[0].New, `{"command":"rm -rf /tmp/x","cwd":"/repo"}`)
	if code != 0 || !strings.Contains(out, `"permission":"deny"`) {
		t.Errorf("cursor deny: %d %s", code, out)
	}
	_, out = runHook(t, cur[0].New, `{"command":"ls","cwd":"/repo"}`)
	if !strings.Contains(out, `"permission":"allow"`) {
		t.Errorf("cursor allow: %s", out)
	}
	var hj map[string]any
	_ = json.Unmarshal([]byte(cur[1].New), &hj)
	if hj["version"] != float64(1) || hj["hooks"].(map[string]any)["beforeShellExecution"] == nil {
		t.Errorf("hooks.json: %s", cur[1].New)
	}
}

func TestSecretDeny(t *testing.T) {
	ch := plan(t, t.TempDir(), "Never commit .env files", "", "claude")
	if !strings.HasSuffix(ch[0].Path, ".sh") {
		t.Fatalf("auto --as for a secret rule should be a hook: %s", ch[0].Path)
	}
	for _, in := range []string{toolJSON("file_path", "/repo/.env"), toolJSON("command", "git add .env.local"), toolJSON("file_path", "/repo/certs/server.pem")} {
		if code, _ := runHook(t, ch[0].New, in); code != 2 {
			t.Errorf("should block %s", in)
		}
	}
	for _, in := range []string{toolJSON("file_path", "/repo/.env.example"), toolJSON("file_path", "/repo/src/environment.ts"), toolJSON("command", "echo $ENV")} {
		if code, _ := runHook(t, ch[0].New, in); code != 0 {
			t.Errorf("should allow %s", in)
		}
	}
	ts := plan(t, t.TempDir(), "Never commit .env files", "test", "claude")
	if ts[0].Path != ".kogentic/tests/never-commit-env-files.sh" || !strings.Contains(ts[0].New, "git ls-files") {
		t.Errorf("secret test stub: %+v", ts[0])
	}
}

func TestRunBeforeCommit(t *testing.T) {
	ch := plan(t, t.TempDir(), "Always run `false` before committing", "", "claude")
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "git commit -m x"), "CLAUDE_PROJECT_DIR="+t.TempDir()); code != 2 {
		t.Errorf("failing check should block commit: %d", code)
	}
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "git status")); code != 0 {
		t.Errorf("non-commit commands pass: %d", code)
	}
	ok := plan(t, t.TempDir(), "Always run `true` before committing", "", "claude")
	if code, _ := runHook(t, ok[0].New, toolJSON("command", "git commit -m x"), "CLAUDE_PROJECT_DIR="+t.TempDir()); code != 0 {
		t.Errorf("passing check should allow commit: %d", code)
	}
}

func TestMustHaveTestHookInGitRepo(t *testing.T) {
	skipNoSh(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	gitc := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	gitc("init", "-q")
	_ = os.MkdirAll(filepath.Join(repo, "src"), 0o755)
	_ = os.WriteFile(filepath.Join(repo, "src/a.go"), []byte("package a\n"), 0o644)
	gitc("add", "src/a.go")
	ch := plan(t, repo, "Every change to `src/*.go` needs a test", "hook", "claude")
	script := ch[0].New
	run := func() int {
		p := filepath.Join(t.TempDir(), "h.sh")
		_ = os.WriteFile(p, []byte(script), 0o755)
		cmd := exec.Command("sh", p)
		cmd.Dir = repo
		cmd.Stdin = strings.NewReader(toolJSON("command", "git commit -m x"))
		err := cmd.Run()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		return 0
	}
	if code := run(); code != 2 {
		t.Errorf("missing test should block: %d", code)
	}
	_ = os.WriteFile(filepath.Join(repo, "src/a_test.go"), []byte("package a\n"), 0o644)
	if code := run(); code != 0 {
		t.Errorf("with test should pass: %d", code)
	}
	auto := plan(t, repo, "Every change to `src/*.go` needs a test", "", "claude")
	if auto[0].Path != ".kogentic/tests/every-change-to-src-go-needs.sh" {
		t.Errorf("auto --as for must-have-test should be a test stub: %s", auto[0].Path)
	}
}

func TestRuleFiles(t *testing.T) {
	c := plan(t, t.TempDir(), "Never edit files in `dist/`", "rule", "claude")[0]
	if c.Path != ".claude/rules/never-edit-files-in-dist.md" || !strings.Contains(c.New, "paths:\n  - \"dist/**\"") {
		t.Errorf("claude rule: %s\n%s", c.Path, c.New)
	}
	c = plan(t, t.TempDir(), "Never edit files in `dist/`", "rule", "cursor")[0]
	if c.Path != ".cursor/rules/never-edit-files-in-dist.mdc" || !strings.Contains(c.New, "globs: dist/**\nalwaysApply: false") {
		t.Errorf("cursor rule: %s\n%s", c.Path, c.New)
	}
	c = plan(t, t.TempDir(), "Prefer composition over inheritance", "", "cursor")[0]
	if !strings.Contains(c.New, "alwaysApply: true") || !strings.Contains(c.New, "When <situation>") {
		t.Errorf("generic rule: %s", c.New)
	}
}

func TestUnsupported(t *testing.T) {
	cases := []Options{
		{Text: "", As: "rule"},
		{Text: "Be nice", As: "hook"},
		{Text: "Don't run `rm -rf`", As: "test"},
		{Text: "Never edit `dist/`", As: "hook", Target: "cursor"},
		{Text: "x", As: "rule", Target: "vim"},
		{Text: "x", As: "poem"},
	}
	for _, o := range cases {
		o.Root = t.TempDir()
		if _, _, err := Plan(o); !errors.Is(err, ErrUnsupported) {
			t.Errorf("Plan(%+v) err = %v, want ErrUnsupported", o, err)
		}
	}
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".claude"), 0o755)
	_ = os.WriteFile(filepath.Join(root, ".claude/settings.json"), []byte("{not json"), 0o644)
	if _, _, err := Plan(Options{Root: root, Text: "Don't run `rm -rf`", As: "hook"}); err == nil || errors.Is(err, ErrUnsupported) {
		t.Errorf("broken settings.json should be a plain error: %v", err)
	}
}

func TestDiffNewFile(t *testing.T) {
	d := Diff(Change{Path: "a.txt", New: "one\ntwo\n"})
	if d != "--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1,2 @@\n+one\n+two\n" {
		t.Errorf("diff:\n%s", d)
	}
	d = Diff(Change{Path: "a.txt", Old: "one\ntwo\n", New: "one\nthree\n"})
	if !strings.Contains(d, " one\n") || !strings.Contains(d, "-two\n") || !strings.Contains(d, "+three\n") {
		t.Errorf("diff:\n%s", d)
	}
}

func TestGlobERE(t *testing.T) {
	cases := map[string]string{
		"dist/":  `([^"]*/)?dist/`,
		".env":   `([^"]*/)?\.env("|/)`,
		"*.lock": `([^"]*/)?[^"/]*\.lock"`,
		"src/**": `([^"]*/)?src/[^"]*"`,
	}
	for in, want := range cases {
		if got := globERE(in); got != want {
			t.Errorf("globERE(%q) = %q, want %q", in, got, want)
		}
	}
	_ = patterns.None
}
