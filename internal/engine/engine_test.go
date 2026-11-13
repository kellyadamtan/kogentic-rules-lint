package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/config"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func repo(t *testing.T, files map[string]string) string {
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

func ids(res *Result) map[string]int {
	m := map[string]int{}
	for _, f := range res.Findings {
		m[f.RuleID]++
	}
	return m
}

func TestInlineDisables(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": `- Write clean code.
<!-- rules-lint-disable-next-line V001 -->
- Write nice code.
- Write better code. <!-- rules-lint-disable V001 -->
<!-- rules-lint-disable V004 -->
- Try to write proper code.
<!-- rules-lint-enable V004 -->
- Ideally keep it short.
<!-- rules-lint-disable -->
- Never use any.
`})
	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.RuleID+"@"+itoa(f.Line))
	}
	want := "V001@1 V004@8"
	if j := join(got); j != want {
		t.Errorf("got %s, want %s", j, want)
	}
}

func itoa(i int) string { return fmtInt(i) }

func TestConfigSeverityOnlyIgnore(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "- Write clean code.\n- Try to test.\n- Be helpful.\n"})
	cfg := config.Default()
	cfg.Rules["V001"] = "error"
	cfg.Rules["V004"] = "off"
	res, err := Run(Options{Root: root, Config: cfg})
	if err != nil {
		t.Fatal(err)
	}
	m := ids(res)
	if m["V001"] != 1 || m["V004"] != 0 || m["V002"] != 1 {
		t.Errorf("unexpected findings %v", m)
	}
	for _, f := range res.Findings {
		if f.RuleID == "V001" && f.Severity != core.SevError {
			t.Errorf("severity override not applied: %v", f.Severity)
		}
	}
	e, w, i := res.Counts()
	if e != 1 || w != 0 || i != 1 {
		t.Errorf("counts %d %d %d", e, w, i)
	}
	res, _ = Run(Options{Root: root, Only: []string{"V002"}})
	if m := ids(res); len(m) != 1 || m["V002"] != 1 {
		t.Errorf("--rule V002: %v", m)
	}
	res, _ = Run(Options{Root: root, Ignore: []string{"V001", "V002"}})
	if m := ids(res); m["V001"] != 0 || m["V002"] != 0 || m["V004"] != 1 {
		t.Errorf("--ignore: %v", m)
	}
}

func TestBadIDsAndConfig(t *testing.T) {
	root := repo(t, map[string]string{"CLAUDE.md": "x\n"})
	if _, err := Run(Options{Root: root, Only: []string{"X999"}}); err == nil {
		t.Error("unknown --rule should fail")
	}
	cfg := config.Default()
	cfg.Rules["Z001"] = "error"
	if _, err := Run(Options{Root: root, Config: cfg}); err == nil {
		t.Error("unknown config rule should fail")
	}
	cfg = config.Default()
	cfg.Rules["V001"] = "loud"
	if _, err := Run(Options{Root: root, Config: cfg}); err == nil {
		t.Error("bad severity should fail")
	}
}

func TestSortedAndTokens(t *testing.T) {
	root := repo(t, map[string]string{
		"CLAUDE.md": "- Write clean code.\n- Be helpful.\n",
		"AGENTS.md": "- Write nice code.\n",
	})
	res, err := Run(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(res.Findings); i++ {
		a, b := res.Findings[i-1], res.Findings[i]
		if a.Path > b.Path || (a.Path == b.Path && a.Line > b.Line) {
			t.Errorf("not sorted: %v before %v", a, b)
		}
	}
	if res.AlwaysOnTokens != (35+3)/4+(19+3)/4 {
		t.Errorf("tokens = %d", res.AlwaysOnTokens)
	}
}
