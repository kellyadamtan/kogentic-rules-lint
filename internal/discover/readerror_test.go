package discover

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoverReportsUnreadablePaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not make files unreadable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read files regardless of permissions")
	}
	root := t.TempDir()
	write(t, root, map[string]string{
		"CLAUDE.md":      "See @docs/shared.md\n- Run `make test`.\n",
		"docs/shared.md": "- shared\n",
		"AGENTS.md":      "- root\n",
		"pkg/.gitignore": "gen/\n",
		"pkg/AGENTS.md":  "- nested\n",
		"locked/x.txt":   "x\n",
	})
	for _, rel := range []string{"docs/shared.md", "AGENTS.md", "pkg/.gitignore", "locked"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
	}
	res, err := Discover(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, re := range res.ReadErrors {
		got = append(got, re.Error())
	}
	want := []string{
		"AGENTS.md: permission denied",
		"docs/shared.md: permission denied",
		"locked: permission denied",
		"pkg/.gitignore: permission denied",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("read errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path)
	}
	if strings.Join(paths, " ") != "CLAUDE.md pkg/AGENTS.md" {
		t.Errorf("readable files should still load: %v", paths)
	}
}
