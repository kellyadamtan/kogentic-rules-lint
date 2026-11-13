// Package cli wires the commands together.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kellyadamtan/kogentic-rules-lint/docs"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/config"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/connector"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/harden"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/report"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/rules"
)

// Exit codes.
const (
	ExitOK       = 0 // no findings at the failing level
	ExitFindings = 1 // errors found (or warnings with --strict)
	ExitUsage    = 2 // bad flags, arguments or config
	ExitRuntime  = 3 // I/O failure, or --send-to-kogentic failed
)

// Build information, set by main via ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func usageErr(format string, a ...any) error {
	return &exitError{ExitUsage, fmt.Errorf(format, a...)}
}

func runtimeErr(err error) error { return &exitError{ExitRuntime, err} }

// newClient builds the Kogentic client. Tests replace it only to trust a
// local test TLS certificate; the redirect policy always comes from
// connector.NewClient.
var newClient = connector.NewClient

// isTerminal is replaceable in tests.
var isTerminal = func(r io.Reader) bool {
	if f, ok := r.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}

type app struct {
	in       io.Reader
	out, err io.Writer
	code     int // exit code chosen by a successful run (e.g. findings)
}

type lintOpts struct {
	format, configPath string
	strict             bool
	only, ignore       []string
	send, yes, redact  bool
	kogenticURL        string
}

func splitIDs(v []string) []string {
	var out []string
	for _, s := range v {
		for _, p := range strings.Split(s, ",") {
			if p = strings.ToUpper(strings.TrimSpace(p)); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func addLintFlags(cmd *cobra.Command, o *lintOpts) {
	f := cmd.Flags()
	f.StringVar(&o.format, "format", "text", "output format: text, json, sarif or github")
	f.BoolVar(&o.strict, "strict", false, "exit 1 on warnings too")
	f.StringSliceVar(&o.only, "rule", nil, "run only these rule IDs (comma-separated)")
	f.StringSliceVar(&o.ignore, "ignore", nil, "skip these rule IDs (comma-separated)")
	f.StringVar(&o.configPath, "config", "", "config file (default <path>/"+config.FileName+")")
	f.BoolVar(&o.send, "send-to-kogentic", false, "opt-in: upload the findings report to Kogentic (shows it and asks first)")
	f.BoolVar(&o.yes, "yes", false, "with --send-to-kogentic: do not ask for confirmation (for CI)")
	f.BoolVar(&o.redact, "redact-snippets", false, "with --send-to-kogentic: strip text snippets from the upload")
	f.StringVar(&o.kogenticURL, "kogentic-url", "", "Kogentic API base URL (or "+connector.EnvURL+"); no default")
}

// Execute runs the CLI and returns the process exit code.
func Execute(args []string, in io.Reader, out, errw io.Writer) int {
	a := &app{in: in, out: out, err: errw}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errw)
	err := root.Execute()
	if err == nil {
		return a.code
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			fmt.Fprintln(errw, "error:", ee.err)
		}
		return ee.code
	}
	// cobra flag/arg errors
	fmt.Fprintln(errw, "error:", err)
	return ExitUsage
}

func (a *app) rootCmd() *cobra.Command {
	o := &lintOpts{}
	root := &cobra.Command{
		Use:   "kogentic-rules-lint [path]",
		Short: "Lint coding-agent rules files (CLAUDE.md, AGENTS.md, .cursor/rules, Copilot, Windsurf, Cline, Roo, Gemini).",
		Long: "kogentic-rules-lint finds vague, mis-scoped, duplicate and contradictory instructions in every\n" +
			"coding agent's rules files, and turns corrections you keep repeating into hooks.\n" +
			"Runs offline. No account, no telemetry.",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.lint(args, o)
		},
	}
	addLintFlags(root, o)
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usageErr("%v", err) })

	lo := &lintOpts{}
	lint := &cobra.Command{
		Use:   "lint [path]",
		Short: "Lint the rules files under path (default .)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.lint(args, lo)
		},
	}
	addLintFlags(lint, lo)

	root.AddCommand(lint, a.explainCmd(), a.initCmd(), a.hardenCmd(), a.loginCmd(), a.logoutCmd(), a.whoamiCmd(), a.versionCmd())
	root.CompletionOptions.DisableDefaultCmd = true
	return root
}

func (a *app) lint(args []string, o *lintOpts) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return usageErr("%s is not a directory", root)
	}
	valid := false
	for _, f := range report.Formats {
		valid = valid || f == o.format
	}
	if !valid {
		return usageErr("unknown --format %q (use %s)", o.format, strings.Join(report.Formats, ", "))
	}
	if !o.send && (o.yes || o.redact || o.kogenticURL != "") {
		return usageErr("--yes, --redact-snippets and --kogentic-url only apply with --send-to-kogentic")
	}
	only, ignore := splitIDs(o.only), splitIDs(o.ignore)
	if err := engine.ValidateIDs(append(append([]string{}, only...), ignore...)); err != nil {
		return usageErr("%v", err)
	}
	cfg, _, err := config.Load(root, o.configPath)
	if err != nil {
		return usageErr("config: %v", err)
	}

	// Resolve connector settings before linting so a misconfiguration fails fast.
	var base, key string
	if o.send {
		u := o.kogenticURL
		if u == "" {
			u = os.Getenv(connector.EnvURL)
		}
		if base, err = connector.BaseURL(u); err != nil {
			return usageErr("%v", err)
		}
		if key, _, err = connector.ResolveKey(); err != nil {
			return usageErr("%v", err)
		}
	}

	res, err := engine.Run(engine.Options{Root: root, Config: cfg, Only: only, Ignore: ignore})
	if err != nil {
		if strings.HasPrefix(err.Error(), "config:") || strings.HasPrefix(err.Error(), "unknown rule") {
			return usageErr("%v", err)
		}
		return runtimeErr(err)
	}
	if err := report.Write(a.out, o.format, res, Version); err != nil {
		return runtimeErr(err)
	}
	e, w, _ := res.Counts()
	if e > 0 || (o.strict && w > 0) {
		a.code = ExitFindings
	}
	if n := len(res.ReadErrors); n > 0 {
		for _, re := range res.ReadErrors {
			fmt.Fprintln(a.err, "error: cannot read", re.Error())
		}
		if o.send {
			fmt.Fprintln(a.err, "Not sent: the report would be incomplete.")
		}
		return runtimeErr(fmt.Errorf("%d path(s) could not be read; results are incomplete", n))
	}
	if o.send {
		return a.send(res, root, base, key, o)
	}
	return nil
}

func (a *app) send(res *engine.Result, root, base, key string, o *lintOpts) error {
	rep := connector.BuildReport(res, Version, connector.RepoInfoFor(root), o.redact)
	preview, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Fprintf(a.err, "\n--send-to-kogentic will POST this report to %s/v1/rules-lint/reports (key %s):\n%s\n", base, connector.Mask(key), preview)
	if !o.yes {
		if !isTerminal(a.in) {
			return usageErr("not sending: no terminal to confirm on; pass --yes to send non-interactively")
		}
		fmt.Fprint(a.err, "Send it? [y/N] ")
		line, _ := bufio.NewReader(a.in).ReadString('\n')
		if ans := strings.ToLower(strings.TrimSpace(line)); ans != "y" && ans != "yes" {
			fmt.Fprintln(a.err, "Not sent.")
			return nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := newClient(base, key).SendReport(ctx, rep)
	if err != nil {
		return runtimeErr(fmt.Errorf("send to Kogentic failed: %w", err))
	}
	fmt.Fprintf(a.err, "Sent report %s %s\n", resp.ReportID, resp.URL)
	return nil
}

func (a *app) explainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain [RULE_ID]",
		Short: "Print a rule's documentation (or list all rules)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				for _, r := range rules.All() {
					fmt.Fprintf(a.out, "%s  %-30s %-8s %s\n", r.ID, r.Name, r.Default, r.Summary)
				}
				return nil
			}
			id := strings.ToUpper(args[0])
			doc, ok := docs.Rule(id)
			if _, known := rules.Get(id); !ok || !known {
				return usageErr("unknown rule %q (run `kogentic-rules-lint explain` for the list)", args[0])
			}
			fmt.Fprint(a.out, doc)
			return nil
		},
	}
}

func (a *app) initCmd() *cobra.Command {
	force := false
	c := &cobra.Command{
		Use:   "init [path]",
		Short: "Write " + config.FileName + " with defaults",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			p := filepath.Join(dir, config.FileName)
			if _, err := os.Stat(p); err == nil && !force {
				return usageErr("%s already exists (use --force to overwrite)", p)
			}
			if err := os.WriteFile(p, []byte(config.DefaultYAML), 0o644); err != nil {
				return runtimeErr(err)
			}
			fmt.Fprintln(a.out, "Wrote", p)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite an existing config")
	return c
}

func (a *app) hardenCmd() *cobra.Command {
	var as, target, dir string
	var write, dry bool
	c := &cobra.Command{
		Use:   "harden \"<correction text>\"",
		Short: "Turn a repeated correction into a scoped rule, a hook or a test stub",
		Long: "harden turns a correction you keep making into something enforced, from templates:\n" +
			"path deny-list, command deny-list, secret-file deny, file-pattern-must-have-test and\n" +
			"run-before-commit. It always prints a diff and writes only with --write.\n\n" +
			"Test stubs (--as test) for path deny-lists and must-have-test compare against\n" +
			"BASE_REF (default origin/main) and fail if it is missing: in GitHub Actions, check\n" +
			"out with fetch-depth: 0.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if as != "" && as != "rule" && as != "hook" && as != "test" {
				return usageErr("--as must be rule, hook or test")
			}
			if target != "claude" && target != "cursor" {
				return usageErr("--target must be claude or cursor")
			}
			if write && dry {
				return usageErr("--write and --dry-run cannot be used together")
			}
			changes, p, err := harden.Plan(harden.Options{Root: dir, Text: args[0], As: as, Target: target})
			if err != nil {
				if errors.Is(err, harden.ErrUnsupported) {
					return usageErr("%v", err)
				}
				return runtimeErr(err)
			}
			fmt.Fprintf(a.err, "Template: %s\n", p.Kind)
			for _, ch := range changes {
				fmt.Fprint(a.out, harden.Diff(ch))
			}
			if !write {
				fmt.Fprintln(a.err, "Dry run: nothing written. Re-run with --write to apply.")
				return nil
			}
			if err := harden.Apply(dir, changes); err != nil {
				return runtimeErr(err)
			}
			for _, ch := range changes {
				fmt.Fprintln(a.err, "Wrote", ch.Path)
			}
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&as, "as", "", "rule, hook or test (default: chosen from the correction)")
	f.StringVar(&target, "target", "claude", "claude or cursor")
	f.BoolVar(&dry, "dry-run", false, "print the diff only (the default unless --write)")
	f.BoolVar(&write, "write", false, "write the files")
	f.StringVar(&dir, "dir", ".", "repository root to write into")
	return c
}

func resolveURL(flag string) (string, error) {
	if flag == "" {
		flag = os.Getenv(connector.EnvURL)
	}
	return connector.BaseURL(flag)
}

func (a *app) loginCmd() *cobra.Command {
	var u string
	c := &cobra.Command{
		Use:   "login",
		Short: "Opt-in: store a Kogentic API key in the OS keychain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := resolveURL(u)
			if err != nil {
				return usageErr("%v", err)
			}
			var key string
			if isTerminal(a.in) {
				fmt.Fprint(a.err, "Kogentic API key (input hidden): ")
				f := a.in.(*os.File)
				b, err := term.ReadPassword(int(f.Fd()))
				fmt.Fprintln(a.err)
				if err != nil {
					return runtimeErr(err)
				}
				key = string(b)
			} else {
				line, _ := bufio.NewReader(a.in).ReadString('\n')
				key = line
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return usageErr("no key given")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			me, err := newClient(base, key).Me(ctx)
			if err != nil {
				return runtimeErr(fmt.Errorf("key check failed: %w", err))
			}
			if err := connector.StoreKey(key); err != nil {
				return runtimeErr(fmt.Errorf("could not store the key in the OS keychain (set %s instead): %w", connector.EnvKey, err))
			}
			fmt.Fprintf(a.out, "Logged in as %s (%s). Key %s stored in the OS keychain.\n", me.User, me.Workspace, connector.Mask(key))
			return nil
		},
	}
	c.Flags().StringVar(&u, "kogentic-url", "", "Kogentic API base URL (or "+connector.EnvURL+")")
	return c
}

func (a *app) logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the Kogentic API key from the OS keychain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, err := connector.DeleteKey()
			if err != nil {
				return runtimeErr(err)
			}
			if ok {
				fmt.Fprintln(a.out, "Removed the Kogentic API key from the OS keychain.")
			} else {
				fmt.Fprintln(a.out, "No Kogentic API key was stored.")
			}
			if os.Getenv(connector.EnvKey) != "" {
				fmt.Fprintf(a.out, "Note: %s is still set in your environment.\n", connector.EnvKey)
			}
			return nil
		},
	}
}

func (a *app) whoamiCmd() *cobra.Command {
	var u string
	c := &cobra.Command{
		Use:   "whoami",
		Short: "Show which Kogentic API key is configured (masked)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, src, err := connector.ResolveKey()
			if err != nil {
				return usageErr("%v", err)
			}
			fmt.Fprintf(a.out, "Key %s from %s\n", connector.Mask(key), src)
			if u == "" && os.Getenv(connector.EnvURL) == "" {
				return nil
			}
			base, err := resolveURL(u)
			if err != nil {
				return usageErr("%v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			me, err := newClient(base, key).Me(ctx)
			if err != nil {
				return runtimeErr(err)
			}
			fmt.Fprintf(a.out, "User %s, workspace %s\n", me.User, me.Workspace)
			return nil
		},
	}
	c.Flags().StringVar(&u, "kogentic-url", "", "Kogentic API base URL (or "+connector.EnvURL+")")
	return c
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(a.out, "kogentic-rules-lint %s (commit %s, built %s)\n", Version, Commit, Date)
		},
	}
}
