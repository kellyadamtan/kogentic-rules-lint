package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/config"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
)

// TestS002BudgetBoundary pins the budget comparison: exactly at the budget is
// fine, one token over is a finding. Tokens are estimated as ceil(bytes/4).
func TestS002BudgetBoundary(t *testing.T) {
	const budget = 50
	cases := []struct {
		name  string
		bytes int
		want  int
	}{
		{"one token under", budget*4 - 4, 0},
		{"exactly at budget", budget * 4, 0},
		{"one byte over", budget*4 + 1, 1},
		{"one token over", budget*4 + 4, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			line := "- Run `make test` before you finish.\n"
			body := strings.Repeat(line, c.bytes/len(line)+1)[:c.bytes]
			if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := config.Default()
			cfg.Budget.AlwaysOnTokens = budget
			res, err := engine.Run(engine.Options{Root: dir, Config: cfg, Only: []string{"S002"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Findings) != c.want {
				t.Fatalf("%d bytes (~%d tokens, budget %d): %d findings, want %d", c.bytes, res.AlwaysOnTokens, budget, len(res.Findings), c.want)
			}
		})
	}
}
