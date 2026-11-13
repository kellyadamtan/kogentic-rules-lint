// Package engine runs discovery and rules and applies config and inline
// disables.
package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/config"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/discover"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/rules"
)

// Options configures a run.
type Options struct {
	Root   string
	Config *config.Config
	Only   []string // run only these rule IDs
	Ignore []string // skip these rule IDs
}

// Result is the outcome of a run.
type Result struct {
	Root           string
	Files          []*core.File
	Findings       []core.Finding
	Rules          []*core.Rule // rules that ran
	AlwaysOnTokens int
	// ReadErrors lists paths that could not be read. The CLI exits 3 when
	// it is not empty, after printing the findings it has.
	ReadErrors []discover.ReadError
}

// Counts returns the number of findings per severity.
func (r *Result) Counts() (errors, warnings, infos int) {
	for _, f := range r.Findings {
		switch f.Severity {
		case core.SevError:
			errors++
		case core.SevWarning:
			warnings++
		case core.SevInfo:
			infos++
		}
	}
	return
}

// ValidateIDs returns an error naming any unknown rule ID.
func ValidateIDs(ids []string) error {
	var bad []string
	for _, id := range ids {
		if _, ok := rules.Get(id); !ok {
			bad = append(bad, id)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("unknown rule ID(s): %s (run `kogentic-rules-lint explain` for the list)", strings.Join(bad, ", "))
	}
	return nil
}

// Run lints opts.Root.
func Run(opts Options) (*Result, error) {
	cfg := opts.Config
	if cfg == nil {
		cfg = config.Default()
	}
	sev := map[string]core.Severity{}
	for id, v := range cfg.Rules {
		if err := ValidateIDs([]string{id}); err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		s, err := core.ParseSeverity(v)
		if err != nil {
			return nil, fmt.Errorf("config: rule %s: %w", id, err)
		}
		sev[id] = s
	}
	if err := ValidateIDs(append(append([]string{}, opts.Only...), opts.Ignore...)); err != nil {
		return nil, err
	}

	d, err := discover.Discover(opts.Root, cfg.Paths.Extra, cfg.Paths.Ignore)
	if err != nil {
		return nil, err
	}
	ctx := &core.Context{Root: opts.Root, Files: d.Files, RepoFiles: d.RepoFiles, Budget: cfg.Budget.AlwaysOnTokens}
	res := &Result{Root: opts.Root, Files: d.Files, ReadErrors: d.ReadErrors}
	for _, f := range d.Files {
		if f.AlwaysOn {
			res.AlwaysOnTokens += f.Tokens()
		}
	}

	only := set(opts.Only)
	skip := set(opts.Ignore)
	byPath := map[string]*core.File{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	for _, r := range rules.All() {
		if (len(only) > 0 && !only[r.ID]) || skip[r.ID] {
			continue
		}
		level := r.Default
		if s, ok := sev[r.ID]; ok {
			level = s
		}
		if level == core.SevOff {
			continue
		}
		res.Rules = append(res.Rules, r)
		for _, fd := range r.Check(ctx) {
			fd.Severity = level
			if f := byPath[fd.Path]; f != nil && Suppressed(f.Directives, fd.RuleID, fd.Line) {
				continue
			}
			res.Findings = append(res.Findings, fd)
		}
	}
	sort.SliceStable(res.Findings, func(i, j int) bool {
		a, b := res.Findings[i], res.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.RuleID < b.RuleID
	})
	return res, nil
}

func set(ids []string) map[string]bool {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func applies(d core.Directive, id string) bool {
	if len(d.Rules) == 0 {
		return true
	}
	for _, r := range d.Rules {
		if r == id {
			return true
		}
	}
	return false
}

// Suppressed reports whether inline directives disable rule id at line.
func Suppressed(dirs []core.Directive, id string, line int) bool {
	off := false
	for _, d := range dirs {
		if !applies(d, id) {
			continue
		}
		switch d.Kind {
		case "disable-line":
			if d.Target == line {
				return true
			}
		case "disable":
			if d.Line <= line {
				off = true
			}
		case "enable":
			if d.Line <= line {
				off = false
			}
		}
	}
	return off
}
