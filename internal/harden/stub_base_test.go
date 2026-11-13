package harden

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRepo makes a repo with a "main" branch (one commit) and a "work" branch
// checked out. It has no origin remote, like a shallow CI checkout that
// lacks origin/main.
func gitRepo(t *testing.T) (dir string, git func(args ...string)) {
	t.Helper()
	skipNoSh(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir = t.TempDir()
	git = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	writeFile(t, dir, "README.md", "x\n")
	git("add", ".")
	git("commit", "-q", "-m", "base")
	git("checkout", "-q", "-b", "work")
	return dir, git
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runStub runs a generated CI test stub with sh inside dir.
func runStub(t *testing.T, dir, script string, env ...string) (int, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", p)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), string(out)
	} else if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

// TestCIStubsFailWithoutBaseRef: with no origin/main (GitHub's default
// shallow checkout) the path-deny and must-have-test stubs must fail loudly
// instead of diffing nothing and printing ok.
func TestCIStubsFailWithoutBaseRef(t *testing.T) {
	stubs := map[string]string{
		"path-deny":      "Never edit files in `dist/`",
		"must-have-test": "Every change to `src/*.go` needs a test",
	}
	for name, text := range stubs {
		t.Run(name, func(t *testing.T) {
			dir, git := gitRepo(t)
			script := plan(t, dir, text, "test", "claude")[0].New
			if !strings.Contains(script, "fetch-depth: 0") {
				t.Error("stub should document fetch-depth: 0")
			}

			// Clean change: passes against main, fails without a base ref.
			writeFile(t, dir, "docs/notes.md", "x\n")
			git("add", ".")
			git("commit", "-q", "-m", "docs")
			if code, out := runStub(t, dir, script, "BASE_REF=main"); code != 0 || !strings.Contains(out, "ok") {
				t.Fatalf("clean change with BASE_REF=main: exit %d\n%s", code, out)
			}
			code, out := runStub(t, dir, script, "BASE_REF=")
			if code == 0 || !strings.Contains(out, "base ref origin/main not found") || !strings.Contains(out, "fetch-depth: 0") {
				t.Fatalf("missing origin/main must fail: exit %d\n%s", code, out)
			}
			if code, out := runStub(t, dir, script, "BASE_REF=no-such-branch"); code == 0 || !strings.Contains(out, "no-such-branch not found") {
				t.Fatalf("unknown BASE_REF must fail: exit %d\n%s", code, out)
			}

			// A real violation is still caught against main.
			writeFile(t, dir, "dist/a.js", "x\n")
			writeFile(t, dir, "src/a.go", "package a\n")
			git("add", ".")
			git("commit", "-q", "-m", "change")
			if code, out := runStub(t, dir, script, "BASE_REF=main"); code != 1 || !strings.Contains(out, "FAIL") {
				t.Fatalf("violation: exit %d\n%s", code, out)
			}

			// A base with no merge base makes git diff fail; that must not pass.
			git("checkout", "-q", "--orphan", "other")
			git("commit", "-q", "--allow-empty", "-m", "unrelated")
			git("checkout", "-q", "work")
			if code, out := runStub(t, dir, script, "BASE_REF=other"); code == 0 {
				t.Fatalf("git diff error must fail the stub: exit %d\n%s", code, out)
			}
		})
	}
}

// TestRunBeforeCommitFailsClosedOnBadProjectDir: if the hook can't cd into
// the project, it blocks the commit instead of allowing it.
func TestRunBeforeCommitFailsClosedOnBadProjectDir(t *testing.T) {
	ch := plan(t, t.TempDir(), "Always run `true` before committing", "", "claude")
	missing := filepath.Join(t.TempDir(), "gone")
	if code, _ := runHook(t, ch[0].New, toolJSON("command", "git commit -m x"), "CLAUDE_PROJECT_DIR="+missing); code != 2 {
		t.Errorf("unreachable project dir must block: exit %d", code)
	}
}
