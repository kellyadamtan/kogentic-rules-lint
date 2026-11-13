package connector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

func TestMask(t *testing.T) {
	cases := map[string]string{"kg_abcdefgh1234": "kg_…1234", "short": "****", "nounderscore1234": "…1234"}
	for in, want := range cases {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBaseURL(t *testing.T) {
	ok := []string{"https://api.example.test", "http://127.0.0.1:4810", "http://localhost:4810/", "http://[::1]:4810"}
	for _, u := range ok {
		if _, err := BaseURL(u); err != nil {
			t.Errorf("BaseURL(%q): %v", u, err)
		}
	}
	bad := []string{"", "http://example.com", "ftp://x", "not a url", "http://10.0.0.1"}
	for _, u := range bad {
		if _, err := BaseURL(u); err == nil {
			t.Errorf("BaseURL(%q) should fail", u)
		}
	}
	if got, _ := BaseURL("https://x.test/api/"); got != "https://x.test/api" {
		t.Errorf("trailing slash not trimmed: %s", got)
	}
}

func TestResolveKeyOrder(t *testing.T) {
	keyring.MockInit()
	t.Setenv(EnvKey, "")
	if _, _, err := ResolveKey(); err != ErrNoKey {
		t.Errorf("want ErrNoKey, got %v", err)
	}
	if err := StoreKey("kg_keychain_0001"); err != nil {
		t.Fatal(err)
	}
	if k, src, _ := ResolveKey(); k != "kg_keychain_0001" || src != "OS keychain" {
		t.Errorf("keychain: %s %s", k, src)
	}
	t.Setenv(EnvKey, "kg_env_0002")
	if k, src, _ := ResolveKey(); k != "kg_env_0002" || src != "env "+EnvKey {
		t.Errorf("env should win: %s %s", k, src)
	}
	if ok, err := DeleteKey(); !ok || err != nil {
		t.Errorf("delete: %v %v", ok, err)
	}
	if ok, err := DeleteKey(); ok || err != nil {
		t.Errorf("second delete: %v %v", ok, err)
	}
}

func TestClientAgainstMock(t *testing.T) {
	mock := &mockkogentic.Server{Key: "kg_test_key_1234"}
	srv := httptest.NewServer(mock)
	defer srv.Close()
	c := NewClient(srv.URL, "kg_test_key_1234")
	me, err := c.Me(context.Background())
	if err != nil || me.User != "mock-user" {
		t.Fatalf("me: %v %+v", err, me)
	}
	res := &engine.Result{AlwaysOnTokens: 42, Findings: []core.Finding{{RuleID: "V001", Severity: core.SevWarning, Path: "CLAUDE.md", Line: 3, Snippet: "Write clean code."}}}
	rep := BuildReport(res, "1.2.3", RepoInfo{Name: "demo", SHA: "abc"}, false)
	if rep.Summary.Warnings != 1 || rep.Summary.AlwaysOnTokens != 42 || rep.Findings[0].Snippet != "Write clean code." {
		t.Errorf("report: %+v", rep)
	}
	out, err := c.SendReport(context.Background(), rep)
	if err != nil || out.ReportID != "rpt_1" {
		t.Fatalf("send: %v %+v", err, out)
	}
	if _, err := NewClient(srv.URL, "kg_wrong_key_9999").Me(context.Background()); err == nil {
		t.Error("wrong key must fail")
	}
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := NewClient(notFound.URL, "k").SendReport(context.Background(), rep); err == nil {
		t.Error("HTTP 404 must fail")
	}
	if red := BuildReport(res, "1", RepoInfo{}, true); red.Findings[0].Snippet != "" {
		t.Error("redact must strip snippets")
	}
}

func TestRepoInfoNoGit(t *testing.T) {
	dir := t.TempDir()
	info := RepoInfoFor(dir)
	if info.Name == "" {
		t.Error("name should fall back to the folder name")
	}
}
