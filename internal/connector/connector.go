// Package connector is the optional, opt-in Kogentic client. Nothing in this
// package runs unless the user passes --send-to-kogentic or runs
// login/logout/whoami. There is no telemetry and no default endpoint.
package connector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
)

// Keychain entry and environment variable names.
const (
	Service = "kogentic"
	Account = "default"
	EnvKey  = "KOGENTIC_API_KEY"
	EnvURL  = "KOGENTIC_BASE_URL"
)

// ErrNoKey means no API key is configured.
var ErrNoKey = errors.New("no Kogentic API key: run `kogentic-rules-lint login` or set " + EnvKey)

// ErrNoURL means no base URL is configured.
var ErrNoURL = errors.New("no Kogentic base URL: pass --kogentic-url or set " + EnvURL + " (there is no default endpoint)")

// Keychain functions, replaceable in tests.
var (
	keyringGet    = keyring.Get
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

// ResolveKey returns the API key and where it came from ("env" or
// "keychain"). The environment variable wins so CI can override.
func ResolveKey() (key, source string, err error) {
	if k := strings.TrimSpace(os.Getenv(EnvKey)); k != "" {
		return k, "env " + EnvKey, nil
	}
	k, err := keyringGet(Service, Account)
	if err == nil && strings.TrimSpace(k) != "" {
		return strings.TrimSpace(k), "OS keychain", nil
	}
	return "", "", ErrNoKey
}

// StoreKey saves the key in the OS keychain.
func StoreKey(key string) error { return keyringSet(Service, Account, key) }

// DeleteKey removes the key from the OS keychain. It reports false if there
// was nothing to delete.
func DeleteKey() (bool, error) {
	err := keyringDelete(Service, Account)
	if errors.Is(err, keyring.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Mask shows only the prefix and last four characters of a key.
func Mask(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	prefix := ""
	if i := strings.Index(key, "_"); i > 0 && i <= 4 {
		prefix = key[:i+1]
	}
	return prefix + "…" + key[len(key)-4:]
}

// BaseURL validates and normalises a base URL. Plain http is only allowed for
// loopback hosts (local mock servers); anything else must be https.
func BaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrNoURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid Kogentic base URL %q", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", fmt.Errorf("refusing plain http to %s: use https (http is allowed only for localhost)", host)
		}
	default:
		return "", fmt.Errorf("invalid Kogentic base URL scheme %q", u.Scheme)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// Client talks to the Kogentic API.
type Client struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
}

// NewClient builds a client with a short timeout that never follows
// redirects. A redirect could move the request to plain http, another port or
// another host, and Go would resend the Authorization header and the report
// body there. The API has no reason to redirect, so any 3xx is an error.
func NewClient(base, key string) *Client {
	return &Client{BaseURL: base, Key: key, HTTP: &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: noRedirects,
	}}
}

// noRedirects stops the client at the first response, so a 3xx is returned
// to do() unfollowed.
func noRedirects(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// Me is the GET /v1/me response.
type Me struct {
	User      string   `json:"user"`
	Workspace string   `json:"workspace"`
	Scopes    []string `json:"scopes"`
}

// ReportResponse is the POST /v1/rules-lint/reports response.
type ReportResponse struct {
	ReportID string `json:"report_id"`
	URL      string `json:"url"`
}

func (c *Client) do(ctx context.Context, method, p string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+p, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%s %s: the API key was rejected (HTTP %d)", method, p, resp.StatusCode)
	}
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		return fmt.Errorf("%s %s: the server answered with a redirect (HTTP %d); redirects are never followed, so nothing was sent onward. Check the Kogentic base URL", method, p, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s: HTTP %d", method, p, resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%s %s: bad response: %w", method, p, err)
		}
	}
	return nil
}

// Me validates the key.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var m Me
	if err := c.do(ctx, http.MethodGet, "/v1/me", nil, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SendReport uploads a findings report.
func (c *Client) SendReport(ctx context.Context, r *Report) (*ReportResponse, error) {
	var out ReportResponse
	if err := c.do(ctx, http.MethodPost, "/v1/rules-lint/reports", r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Report is exactly what --send-to-kogentic uploads. It never contains file
// contents: only rule IDs, paths, line numbers and (unless redacted) short
// snippets of at most 200 characters.
type Report struct {
	Tool     string          `json:"tool"`
	Version  string          `json:"version"`
	Repo     RepoInfo        `json:"repo"`
	Findings []ReportFinding `json:"findings"`
	Summary  ReportSummary   `json:"summary"`
}

// RepoInfo identifies the repository without sending its remote URL.
type RepoInfo struct {
	Name       string `json:"name"`
	RemoteHash string `json:"remote_hash,omitempty"`
	SHA        string `json:"sha,omitempty"`
}

// ReportFinding is one finding in the upload.
type ReportFinding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Snippet  string `json:"snippet,omitempty"`
}

// ReportSummary is the upload summary.
type ReportSummary struct {
	Errors         int `json:"errors"`
	Warnings       int `json:"warnings"`
	AlwaysOnTokens int `json:"always_on_tokens"`
}

// BuildReport converts a lint result into the upload payload.
func BuildReport(res *engine.Result, version string, repo RepoInfo, redact bool) *Report {
	e, w, _ := res.Counts()
	r := &Report{Tool: "rules-lint", Version: version, Repo: repo, Findings: []ReportFinding{},
		Summary: ReportSummary{Errors: e, Warnings: w, AlwaysOnTokens: res.AlwaysOnTokens}}
	for _, f := range res.Findings {
		rf := ReportFinding{Rule: f.RuleID, Severity: f.Severity.String(), File: f.Path, Line: f.Line}
		if !redact {
			rf.Snippet = core.Snip(f.Snippet)
		}
		r.Findings = append(r.Findings, rf)
	}
	return r
}

// RepoInfoFor reads the repo name, a SHA-256 of the origin URL and HEAD from
// local git. Missing git is fine: fields stay empty. Nothing leaves the
// machine here.
func RepoInfoFor(root string) RepoInfo {
	abs, _ := filepath.Abs(root)
	info := RepoInfo{Name: filepath.Base(abs)}
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", abs}, args...)...)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	if top := git("rev-parse", "--show-toplevel"); top != "" {
		info.Name = filepath.Base(top)
	}
	if remote := git("remote", "get-url", "origin"); remote != "" {
		sum := sha256.Sum256([]byte(remote))
		info.RemoteHash = hex.EncodeToString(sum[:])
	}
	info.SHA = git("rev-parse", "HEAD")
	return info
}
