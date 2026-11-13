package harden

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// brokenGrep returns a PATH whose grep always fails with exit status 2, like
// grep does for a bad pattern or an unreadable input.
func brokenGrep(t *testing.T) string {
	t.Helper()
	skipNoSh(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "grep"), []byte("#!/bin/sh\necho 'grep: simulated failure' >&2\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return "PATH=" + dir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// TestHooksFailClosedWhenGrepFails: if the pattern check itself errors, the
// hook must block, not allow. Inputs are ones the working hook would allow.
func TestHooksFailClosedWhenGrepFails(t *testing.T) {
	path := brokenGrep(t)
	cases := []struct {
		text, target, input string
	}{
		{"Never edit files in `dist/`", "claude", toolJSON("file_path", "/repo/src/a.go")},
		{"Don't run `rm -rf`", "claude", toolJSON("command", "ls")},
		{"Don't run `rm -rf`", "cursor", toolJSON("command", "ls")},
		{"Never commit .env files", "claude", toolJSON("file_path", "/repo/src/a.go")},
		{"Never commit .env files", "cursor", toolJSON("command", "ls")},
		{"Always run `true` before committing", "claude", toolJSON("command", "git status")},
		{"Always run `true` before committing", "cursor", toolJSON("command", "git status")},
	}
	for _, c := range cases {
		t.Run(c.target+"/"+c.text, func(t *testing.T) {
			ch := plan(t, t.TempDir(), c.text, "hook", c.target)
			// Sanity: with a working grep the input is allowed.
			code, out := runHook(t, ch[0].New, c.input)
			if code != 0 || strings.Contains(out, `"deny"`) {
				t.Fatalf("working grep should allow: %d %s", code, out)
			}
			code, out = runHook(t, ch[0].New, c.input, path)
			blocked := code == 2 || (c.target == "cursor" && strings.Contains(out, `"permission":"deny"`))
			if !blocked || !strings.Contains(out, "could not run its check") {
				t.Errorf("broken grep must block: exit %d\n%s", code, out)
			}
		})
	}
}
