package connector

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/mockkogentic"
)

const redirKey = "kg_redirect_test_5678"

func server(t *testing.T, h http.Handler, tls bool) *httptest.Server {
	t.Helper()
	srv, err := mockkogentic.NewServer(h, tls)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func sampleReport() *Report {
	res := &engine.Result{Findings: []core.Finding{{RuleID: "V001", Severity: core.SevWarning, Path: "AGENTS.md", Line: 1, Snippet: "SNIPPET"}}}
	return BuildReport(res, "test", RepoInfo{Name: "demo"}, false)
}

// client returns a NewClient whose transport trusts srv's test certificate.
// CheckRedirect is left exactly as NewClient set it.
func client(base string, tlsSrv *httptest.Server) *Client {
	c := NewClient(base, redirKey)
	if tlsSrv != nil {
		c.HTTP.Transport = tlsSrv.Client().Transport
	}
	return c
}

// calls runs every request the client makes: upload, login and whoami.
func calls(c *Client) map[string]error {
	_, sendErr := c.SendReport(context.Background(), sampleReport())
	_, meErr := c.Me(context.Background())
	return map[string]error{"SendReport": sendErr, "Me": meErr}
}

func checkRefused(t *testing.T, errs map[string]error) {
	t.Helper()
	for name, err := range errs {
		if err == nil || !strings.Contains(err.Error(), "redirect") {
			t.Errorf("%s: want a redirect error, got %v", name, err)
		}
		if err != nil && strings.Contains(err.Error(), redirKey) {
			t.Errorf("%s: error leaks the key: %v", name, err)
		}
	}
}

// (a) https -> http downgrade on the same host: the http server must receive
// nothing at all (no key, no report).
func TestRedirectHTTPSToHTTPRefused(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			plain := &mockkogentic.Recorder{}
			plainSrv := server(t, plain, false)
			origin := server(t, mockkogentic.Redirect(status, func(r *http.Request) string { return plainSrv.URL + r.URL.Path }), true)
			if !strings.HasPrefix(origin.URL, "https://127.0.0.1:") || !strings.HasPrefix(plainSrv.URL, "http://127.0.0.1:") {
				t.Fatalf("unexpected test URLs %s %s", origin.URL, plainSrv.URL)
			}
			checkRefused(t, calls(client(origin.URL, origin)))
			if n := plain.Count(); n != 0 {
				t.Fatalf("plain http server received %d request(s), auth=%q", n, plain.Auth)
			}
		})
	}
}

// (b) a redirect to another host must deliver nothing, not even the report
// without the key.
func TestRedirectOtherHostRefused(t *testing.T) {
	other := &mockkogentic.Recorder{}
	otherSrv := server(t, other, false)
	otherURL := strings.Replace(otherSrv.URL, "127.0.0.1", "localhost", 1)
	origin := server(t, mockkogentic.Redirect(http.StatusTemporaryRedirect, func(r *http.Request) string { return otherURL + r.URL.Path }), false)
	checkRefused(t, calls(client(origin.URL, nil)))
	if n := other.Count(); n != 0 {
		t.Fatalf("other host received %d request(s): %q", n, other.Bodies)
	}
}

// (c) even a same-host https redirect is refused: the target path is never hit.
func TestRedirectSameHostHTTPSRefused(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}
	var srv *httptest.Server
	srv = server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits[r.URL.Path]++
		mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/moved") {
			_, _ = w.Write([]byte(`{"report_id":"x","user":"x"}`))
			return
		}
		http.Redirect(w, r, srv.URL+"/moved"+r.URL.Path, http.StatusPermanentRedirect)
	}), true)
	checkRefused(t, calls(client(srv.URL, srv)))
	mu.Lock()
	defer mu.Unlock()
	if hits["/moved/v1/rules-lint/reports"] != 0 || hits["/moved/v1/me"] != 0 {
		t.Fatalf("redirect target was requested: %v", hits)
	}
	if hits["/v1/rules-lint/reports"] != 1 || hits["/v1/me"] != 1 {
		t.Fatalf("expected one request per call to the configured URL: %v", hits)
	}
}

func TestNewClientNeverFollowsRedirects(t *testing.T) {
	c := NewClient("https://example.invalid", "k")
	if c.HTTP.CheckRedirect == nil {
		t.Fatal("NewClient must set CheckRedirect")
	}
	if err := c.HTTP.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatalf("CheckRedirect = %v, want http.ErrUseLastResponse", err)
	}
}
