package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/connector"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

func startServer(t *testing.T, h http.Handler, tls bool) *httptest.Server {
	t.Helper()
	srv, err := mockkogentic.NewServer(h, tls)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

// trustTestCert makes the CLI's client trust srv's test certificate. It keeps
// everything else, including the redirect policy, from connector.NewClient.
func trustTestCert(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := newClient
	newClient = func(base, key string) *connector.Client {
		c := orig(base, key)
		c.HTTP.Transport = srv.Client().Transport
		return c
	}
	t.Cleanup(func() { newClient = orig })
}

// A server that redirects https -> http must not receive the key or the
// report on the http side, and lint, login and whoami must exit 3.
func TestCLIRefusesRedirects(t *testing.T) {
	root := tmpRepo(t, map[string]string{"AGENTS.md": "- Write clean code.\n"})
	cases := []struct {
		name string
		tls  bool
		host string // host used in the redirect target
	}{
		{"https to http, same host", true, "127.0.0.1"},
		{"http to another host", false, "localhost"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sink := &mockkogentic.Recorder{}
			sinkSrv := startServer(t, sink, false)
			target := strings.Replace(sinkSrv.URL, "127.0.0.1", c.host, 1)
			origin := startServer(t, mockkogentic.Redirect(http.StatusTemporaryRedirect, func(r *http.Request) string { return target + r.URL.Path }), c.tls)
			if c.tls {
				trustTestCert(t, origin)
			}
			t.Setenv(connector.EnvKey, testKey)

			code, out, errs := run(t, "", "lint", "--send-to-kogentic", "--yes", "--kogentic-url", origin.URL, root)
			if code != ExitRuntime || !strings.Contains(errs, "redirect") || !strings.Contains(out, "V001") {
				t.Errorf("lint: exit %d, want %d\n%s\n%s", code, ExitRuntime, out, errs)
			}
			code, _, errs = run(t, "", "whoami", "--kogentic-url", origin.URL)
			if code != ExitRuntime || !strings.Contains(errs, "redirect") {
				t.Errorf("whoami: exit %d\n%s", code, errs)
			}
			t.Setenv(connector.EnvKey, "")
			code, _, errs = run(t, testKey+"\n", "login", "--kogentic-url", origin.URL)
			if code != ExitRuntime || !strings.Contains(errs, "redirect") {
				t.Errorf("login: exit %d\n%s", code, errs)
			}
			if strings.Contains(errs, testKey) {
				t.Error("stderr leaks the key")
			}
			if n := sink.Count(); n != 0 {
				t.Fatalf("redirect target received %d request(s), auth=%q", n, sink.Auth)
			}
		})
	}
}
