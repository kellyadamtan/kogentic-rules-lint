package report_test

import (
	"bytes"
	"testing"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/report"
)

// GitHub workflow commands split properties on "," and end them at "::", so
// paths and titles must escape ",", ":" and "%" (and newlines); the message
// only escapes "%" and newlines.
func TestGitHubAnnotationEscaping(t *testing.T) {
	res := &engine.Result{Findings: []core.Finding{{
		RuleID: "V001", RuleName: "vague,instruction:x", Severity: core.SevWarning,
		Path: "docs/a,b:c%d.md", Line: 7, Message: "Too vague, see: 100%\nnext",
	}}}
	var buf bytes.Buffer
	if err := report.Write(&buf, "github", res, "test"); err != nil {
		t.Fatal(err)
	}
	want := "::warning file=docs/a%2Cb%3Ac%25d.md,line=7,title=V001 vague%2Cinstruction%3Ax::Too vague, see: 100%25%0Anext\n"
	if got := buf.String(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
