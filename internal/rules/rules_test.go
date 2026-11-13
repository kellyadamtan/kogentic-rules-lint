package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/docs"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/config"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/rules"
)

const fixtures = "../../testdata/rules"

func run(t *testing.T, dir, id string) []string {
	t.Helper()
	cfg, _, err := config.Load(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	res, err := engine.Run(engine.Options{Root: dir, Config: cfg, Only: []string{id}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Findings {
		if f.RuleID != id {
			t.Errorf("rule %s produced a finding for %s", id, f.RuleID)
		}
		if f.Message == "" || f.Suggestion == "" {
			t.Errorf("%s at %s:%d has an empty message or suggestion", id, f.Path, f.Line)
		}
		got = append(got, fmt.Sprintf("%s:%d", f.Path, f.Line))
	}
	sort.Strings(got)
	return got
}

// TestFixtures runs every rule against testdata/rules/<ID>/bad (findings
// must match bad/.expect exactly) and good (no findings).
func TestFixtures(t *testing.T) {
	for _, r := range rules.All() {
		r := r
		t.Run(r.ID, func(t *testing.T) {
			bad := filepath.Join(fixtures, r.ID, "bad")
			good := filepath.Join(fixtures, r.ID, "good")
			b, err := os.ReadFile(filepath.Join(bad, ".expect"))
			if err != nil {
				t.Fatalf("missing fixture: %v", err)
			}
			want := strings.Fields(string(b))
			sort.Strings(want)
			got := run(t, bad, r.ID)
			if strings.Join(got, " ") != strings.Join(want, " ") {
				t.Errorf("bad fixture findings\n got: %v\nwant: %v", got, want)
			}
			if g := run(t, good, r.ID); len(g) != 0 {
				t.Errorf("good fixture should be clean, got %v", g)
			}
		})
	}
}

func TestEveryRuleHasDocsAndMetadata(t *testing.T) {
	seenNames := map[string]bool{}
	for _, r := range rules.All() {
		if _, ok := docs.Rule(r.ID); !ok {
			t.Errorf("%s has no docs/rules/%s.md", r.ID, r.ID)
		}
		if r.Name == "" || r.Summary == "" || r.Check == nil {
			t.Errorf("%s is missing metadata", r.ID)
		}
		if seenNames[r.Name] {
			t.Errorf("duplicate rule name %s", r.Name)
		}
		seenNames[r.Name] = true
	}
	if n := len(rules.All()); n != 18 {
		t.Errorf("expected 18 rules in v0.1, got %d", n)
	}
}

func TestSimilarity(t *testing.T) {
	if s := rules.Similarity("run the tests before every commit", "run the tests before every commit"); s != 1 {
		t.Errorf("identical = %v", s)
	}
	if s := rules.Similarity("run the tests before every commit", "never deploy on fridays at all"); s != 0 {
		t.Errorf("disjoint = %v", s)
	}
	s := rules.Similarity("keep pull requests small and focused", "keep pull requests small and very focused")
	if s >= rules.DuplicateThreshold {
		t.Errorf("one inserted word should drop below threshold, got %v", s)
	}
}
