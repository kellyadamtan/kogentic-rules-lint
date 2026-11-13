package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/connector"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

const testKey = "kg_test_0000000000abcd"

func run(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Execute(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func tmpRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, c := range files {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestMain(m *testing.M) {
	keyring.MockInit() // never touch the real OS keychain in tests
	_ = os.Unsetenv(connector.EnvKey)
	_ = os.Unsetenv(connector.EnvURL)
	os.Exit(m.Run())
}

func TestExitCodes(t *testing.T) {
	clean := tmpRepo(t, map[string]string{"AGENTS.md": "- Run `make test` before you finish.\n"})
	warnOnly := tmpRepo(t, map[string]string{"AGENTS.md": "- Try to keep functions short.\n"})
	withErr := tmpRepo(t, map[string]string{".cursor/rules/a.md": "- x\n"})
	empty := t.TempDir()
	badCfg := tmpRepo(t, map[string]string{"AGENTS.md": "- x\n", ".kogentic-rules-lint.yaml": "version: 1\nnope: true\n"})

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"clean default command", []string{clean}, ExitOK},
		{"clean lint", []string{"lint", clean}, ExitOK},
		{"warnings pass", []string{"lint", warnOnly}, ExitOK},
		{"warnings fail with --strict", []string{"lint", "--strict", warnOnly}, ExitFindings},
		{"errors fail", []string{withErr}, ExitFindings},
		{"errors ignored", []string{"lint", "--ignore", "F001", withErr}, ExitOK},
		{"errors downgraded by --rule", []string{"lint", "--rule", "V001", withErr}, ExitOK},
		{"no rules files", []string{empty}, ExitOK},
		{"missing path", []string{filepath.Join(empty, "nope")}, ExitUsage},
		{"bad format", []string{"lint", "--format", "xml", clean}, ExitUsage},
		{"unknown rule", []string{"lint", "--rule", "Q123", clean}, ExitUsage},
		{"unknown flag", []string{"lint", "--bogus", clean}, ExitUsage},
		{"too many args", []string{"lint", clean, clean}, ExitUsage},
		{"bad config", []string{badCfg}, ExitUsage},
		{"explain ok", []string{"explain", "v001"}, ExitOK},
		{"explain list", []string{"explain"}, ExitOK},
		{"explain unknown", []string{"explain", "Z999"}, ExitUsage},
		{"version", []string{"version"}, ExitOK},
		{"send flags need --send", []string{"lint", "--yes", clean}, ExitUsage},
		{"send without url", []string{"lint", "--send-to-kogentic", clean}, ExitUsage},
		{"harden unsupported", []string{"harden", "be nice", "--as", "hook"}, ExitUsage},
		{"harden bad --as", []string{"harden", "x", "--as", "poem"}, ExitUsage},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errs := run(t, "", c.args...)
			if code != c.want {
				t.Errorf("exit %d, want %d\nstdout: %s\nstderr: %s", code, c.want, out, errs)
			}
		})
	}
}

func TestFormatsViaCLI(t *testing.T) {
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- Write clean code.\n"})
	_, out, _ := run(t, "", "lint", "--format", "json", root)
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out)
	}
	_, out, _ = run(t, "", "lint", "--format", "github", root)
	if !strings.HasPrefix(out, "::warning file=AGENTS.md,line=1,title=V001 vague-instruction::") {
		t.Errorf("github output: %q", out)
	}
	_, out, _ = run(t, "", "lint", "--format", "sarif", root)
	if !strings.Contains(out, `"version": "2.1.0"`) {
		t.Errorf("sarif output: %s", out)
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if code, _, e := run(t, "", "init", dir); code != 0 {
		t.Fatalf("init: %d %s", code, e)
	}
	if code, _, _ := run(t, "", "init", dir); code != ExitUsage {
		t.Error("second init without --force should fail")
	}
	if code, _, _ := run(t, "", "init", "--force", dir); code != 0 {
		t.Error("init --force should succeed")
	}
	if code, _, e := run(t, "", "lint", dir); code != 0 {
		t.Errorf("generated config should load: %s", e)
	}
}

func TestHardenDryRunAndWrite(t *testing.T) {
	dir := t.TempDir()
	code, out, errs := run(t, "", "harden", "Never edit files in dist/", "--dir", dir)
	if code != 0 || !strings.Contains(out, "+++ b/.claude/hooks/never-edit-files-in-dist.sh") || !strings.Contains(out, "+++ b/.claude/settings.json") {
		t.Fatalf("dry run: %d\n%s\n%s", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(dir, ".claude")); err == nil {
		t.Fatal("dry run must not write")
	}
	if code, _, _ := run(t, "", "harden", "x", "--write", "--dry-run", "--dir", dir); code != ExitUsage {
		t.Error("--write with --dry-run should be a usage error")
	}
	code, _, errs = run(t, "", "harden", "Never edit files in dist/", "--dir", dir, "--write")
	if code != 0 {
		t.Fatalf("write: %d %s", code, errs)
	}
	st, err := os.Stat(filepath.Join(dir, ".claude/hooks/never-edit-files-in-dist.sh"))
	if err != nil || (runtime.GOOS != "windows" && st.Mode().Perm()&0o100 == 0) {
		t.Fatalf("hook script missing or not executable: %v", err)
	}
}

func TestSendToKogenticMock(t *testing.T) {
	mock := &mockkogentic.Server{Key: testKey}
	srv := httptest.NewServer(mock)
	defer srv.Close()
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- Write clean code.\n"})

	t.Setenv(connector.EnvKey, testKey)
	// Non-interactive without --yes: refuse, send nothing.
	code, _, errs := run(t, "", "lint", "--send-to-kogentic", "--kogentic-url", srv.URL, root)
	if code != ExitUsage || len(mock.Received()) != 0 || !strings.Contains(errs, "will POST this report") {
		t.Fatalf("expected refusal: code=%d sent=%d\n%s", code, len(mock.Received()), errs)
	}
	// Interactive "n": nothing sent.
	isTerminal = func(io.Reader) bool { return true }
	defer func() { isTerminal = func(io.Reader) bool { return false } }()
	code, _, _ = run(t, "n\n", "lint", "--send-to-kogentic", "--kogentic-url", srv.URL, root)
	if code != 0 || len(mock.Received()) != 0 {
		t.Fatalf("answer n must not send: code=%d sent=%d", code, len(mock.Received()))
	}
	// Interactive "y": sent.
	code, _, errs = run(t, "y\n", "lint", "--send-to-kogentic", "--kogentic-url", srv.URL, root)
	if code != 0 || len(mock.Received()) != 1 || !strings.Contains(errs, "Sent report rpt_1") {
		t.Fatalf("answer y should send: code=%d sent=%d\n%s", code, len(mock.Received()), errs)
	}
	// --yes with env URL and --redact-snippets.
	t.Setenv(connector.EnvURL, srv.URL)
	code, _, _ = run(t, "", "lint", "--send-to-kogentic", "--yes", "--redact-snippets", root)
	if code != 0 || len(mock.Received()) != 2 {
		t.Fatalf("--yes should send: code=%d", code)
	}
	var rep connector.Report
	if err := json.Unmarshal(mock.Received()[1], &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Tool != "rules-lint" || len(rep.Findings) != 1 || rep.Findings[0].Rule != "V001" || rep.Findings[0].Snippet != "" {
		t.Errorf("unexpected report: %+v", rep)
	}
	raw := string(mock.Received()[0])
	if strings.Contains(raw, testKey) || !strings.Contains(raw, "Write clean code.") {
		t.Errorf("report must carry the snippet but never the key: %s", raw)
	}
	// Wrong key: runtime error, findings still printed.
	t.Setenv(connector.EnvKey, "kg_wrong_000000000000")
	code, out, errs := run(t, "", "lint", "--send-to-kogentic", "--yes", root)
	if code != ExitRuntime || !strings.Contains(errs, "rejected") || !strings.Contains(out, "V001") {
		t.Errorf("wrong key: code=%d\n%s\n%s", code, out, errs)
	}
}

func TestLoginWhoamiLogout(t *testing.T) {
	mock := &mockkogentic.Server{Key: testKey}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	if code, _, _ := run(t, "", "whoami"); code != ExitUsage {
		t.Error("whoami with no key should be a usage error")
	}
	if code, _, _ := run(t, "kg_bad_00000000000000\n", "login", "--kogentic-url", srv.URL); code != ExitRuntime {
		t.Error("login with a rejected key should fail")
	}
	code, out, errs := run(t, testKey+"\n", "login", "--kogentic-url", srv.URL)
	if code != 0 || !strings.Contains(out, "mock-user") || strings.Contains(out+errs, testKey) {
		t.Fatalf("login: %d\n%s\n%s", code, out, errs)
	}
	code, out, _ = run(t, "", "whoami", "--kogentic-url", srv.URL)
	if code != 0 || !strings.Contains(out, "kg_…abcd") || !strings.Contains(out, "OS keychain") || !strings.Contains(out, "mock-workspace") {
		t.Errorf("whoami: %d %s", code, out)
	}
	if code, out, _ = run(t, "", "logout"); code != 0 || !strings.Contains(out, "Removed") {
		t.Errorf("logout: %d %s", code, out)
	}
	if code, out, _ = run(t, "", "logout"); code != 0 || !strings.Contains(out, "No Kogentic API key") {
		t.Errorf("second logout: %d %s", code, out)
	}
	if code, _, _ := run(t, testKey+"\n", "login", "--kogentic-url", "http://example.com"); code != ExitUsage {
		t.Error("plain http to a non-loopback host must be refused")
	}
}
