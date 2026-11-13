package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

// skipNoPerms skips tests that rely on chmod making a file unreadable.
func skipNoPerms(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not make files unreadable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read files regardless of permissions")
	}
}

func chmod(t *testing.T, p string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o755) })
}

// The documented exit codes are part of the CLI contract (README "Exit codes").
func TestExitCodeValues(t *testing.T) {
	if ExitOK != 0 || ExitFindings != 1 || ExitUsage != 2 || ExitRuntime != 3 {
		t.Fatalf("exit codes changed: ok=%d findings=%d usage=%d runtime=%d", ExitOK, ExitFindings, ExitUsage, ExitRuntime)
	}
}

func TestUnreadableRulesFileExits3(t *testing.T) {
	skipNoPerms(t)
	root := tmpRepo(t, map[string]string{
		"CLAUDE.md": "- Run `make test`.\n",
		"AGENTS.md": "- Write clean code.\n",
	})
	chmod(t, filepath.Join(root, "CLAUDE.md"), 0)
	code, out, errs := run(t, "", "lint", root)
	if code != 3 {
		t.Fatalf("exit %d, want 3\n%s\n%s", code, out, errs)
	}
	if !strings.Contains(errs, "cannot read CLAUDE.md: permission denied") {
		t.Errorf("stderr should name the path: %s", errs)
	}
	if !strings.Contains(out, "AGENTS.md") || !strings.Contains(out, "V001") {
		t.Errorf("findings for readable files should still print: %s", out)
	}
	// Errors in the readable files don't hide the I/O failure.
	withErr := tmpRepo(t, map[string]string{".cursor/rules/a.md": "- x\n", "GEMINI.md": "x\n"})
	chmod(t, filepath.Join(withErr, "GEMINI.md"), 0)
	if code, _, errs := run(t, "", "lint", withErr); code != 3 {
		t.Errorf("exit %d, want 3: %s", code, errs)
	}
}

func TestUnreadableDirectoryExits3(t *testing.T) {
	skipNoPerms(t)
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- Run `make test`.\n", "sub/AGENTS.md": "- Run `go test`.\n"})
	chmod(t, filepath.Join(root, "sub"), 0)
	code, _, errs := run(t, "", "lint", root)
	if code != 3 || !strings.Contains(errs, "cannot read sub:") {
		t.Fatalf("exit %d, want 3\n%s", code, errs)
	}
	// An ignored directory is never read, so it is not an error.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("sub/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := run(t, "", "lint", root); code != 0 {
		t.Fatalf("ignored unreadable dir: exit %d\n%s", code, errs)
	}
}

// An unreadable root fails inside the engine; that is an I/O error (3), not a
// usage error (2).
func TestUnreadableRootExits3(t *testing.T) {
	skipNoPerms(t)
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- x\n"})
	chmod(t, root, 0o300) // can stat, cannot list
	code, out, errs := run(t, "", "lint", root)
	if code != 3 {
		t.Fatalf("exit %d, want 3\n%s\n%s", code, out, errs)
	}
}

func TestUnreadableFileBlocksSend(t *testing.T) {
	skipNoPerms(t)
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- Write clean code.\n", "GEMINI.md": "x\n"})
	chmod(t, filepath.Join(root, "GEMINI.md"), 0)
	t.Setenv("KOGENTIC_API_KEY", testKey)
	sink := &mockkogentic.Recorder{}
	srv := startServer(t, sink, false)
	code, _, errs := run(t, "", "lint", "--send-to-kogentic", "--yes", "--kogentic-url", srv.URL, root)
	if code != 3 || !strings.Contains(errs, "Not sent") {
		t.Fatalf("exit %d\n%s", code, errs)
	}
	if n := sink.Count(); n != 0 {
		t.Fatalf("an incomplete report must not be uploaded: server got %d request(s)", n)
	}
	// Control: once the file is readable the same command does upload.
	chmod(t, filepath.Join(root, "GEMINI.md"), 0o644)
	if code, _, errs := run(t, "", "lint", "--send-to-kogentic", "--yes", "--kogentic-url", srv.URL, root); code != 0 || sink.Count() != 1 {
		t.Fatalf("readable repo should send: exit %d, requests %d\n%s", code, sink.Count(), errs)
	}
}
