package report_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/report"
)

var update = flag.Bool("update", false, "rewrite golden files")

const example = "../../examples/messy-repo"

func render(t *testing.T, format string) []byte {
	t.Helper()
	res, err := engine.Run(engine.Options{Root: example})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := report.Write(&buf, format, res, "0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestGolden compares every output format for examples/messy-repo against
// testdata/golden. Run `go test ./internal/report -update` after an
// intentional change and review the diff.
func TestGolden(t *testing.T) {
	ext := map[string]string{"text": "txt", "json": "json", "sarif": "sarif", "github": "github.txt"}
	for _, format := range report.Formats {
		t.Run(format, func(t *testing.T) {
			got := render(t, format)
			golden := filepath.Join("../../testdata/golden", "messy-repo."+ext[format])
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update to create)", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s output differs from %s; run with -update and review the diff\n--- got ---\n%s", format, golden, got)
			}
		})
	}
}

func TestJSONShape(t *testing.T) {
	var doc struct {
		Tool     string           `json:"tool"`
		Version  string           `json:"version"`
		Summary  map[string]int   `json:"summary"`
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(render(t, "json"), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Tool != "kogentic-rules-lint" || doc.Version != "0.0.0-test" || len(doc.Findings) == 0 {
		t.Fatalf("bad doc: %+v", doc)
	}
	if doc.Summary["errors"]+doc.Summary["warnings"]+doc.Summary["info"] != len(doc.Findings) {
		t.Error("summary counts do not add up")
	}
	for _, f := range doc.Findings {
		for _, k := range []string{"rule", "name", "severity", "file", "line", "message"} {
			if _, ok := f[k]; !ok {
				t.Errorf("finding missing %q: %v", k, f)
			}
		}
	}
}

func TestSARIFShape(t *testing.T) {
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID      string `json:"id"`
						HelpURI string `json:"helpUri"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				RuleIndex int    `json:"ruleIndex"`
				Level     string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(render(t, "sarif"), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		t.Fatalf("bad sarif: %+v", doc)
	}
	run := doc.Runs[0]
	if len(run.Tool.Driver.Rules) != 18 {
		t.Errorf("want 18 rules in driver, got %d", len(run.Tool.Driver.Rules))
	}
	for _, r := range run.Results {
		if run.Tool.Driver.Rules[r.RuleIndex].ID != r.RuleID {
			t.Errorf("ruleIndex mismatch for %s", r.RuleID)
		}
		if r.Level != "error" && r.Level != "warning" && r.Level != "note" {
			t.Errorf("bad level %q", r.Level)
		}
	}
	if !strings.HasSuffix(run.Tool.Driver.Rules[0].HelpURI, "/docs/rules/C001.md") {
		t.Errorf("helpUri: %s", run.Tool.Driver.Rules[0].HelpURI)
	}
}

func TestUnknownFormatAndEmpty(t *testing.T) {
	res := &engine.Result{}
	if err := report.Write(&bytes.Buffer{}, "xml", res, "x"); err == nil {
		t.Error("expected error for unknown format")
	}
	var buf bytes.Buffer
	_ = report.Write(&buf, "text", res, "x")
	if !strings.Contains(buf.String(), "No agent rules files found.") {
		t.Errorf("empty text output: %q", buf.String())
	}
	buf.Reset()
	_ = report.Write(&buf, "json", res, "x")
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("empty json should have an empty findings array: %s", buf.String())
	}
}
