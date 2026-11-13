// Package report writes lint results as text, json, sarif or github
// workflow annotations.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
	"github.com/kellyadamtan/kogentic-rules-lint/internal/engine"
)

// Tool name and project URL used in reports.
const (
	ToolName = "kogentic-rules-lint"
	RepoURL  = "https://github.com/kellyadamtan/kogentic-rules-lint"
)

// Formats lists the supported output formats.
var Formats = []string{"text", "json", "sarif", "github"}

// DocURL is the documentation page of a rule.
func DocURL(id string) string { return RepoURL + "/blob/main/docs/rules/" + id + ".md" }

// Write renders res in format to w.
func Write(w io.Writer, format string, res *engine.Result, version string) error {
	switch format {
	case "text", "":
		return writeText(w, res)
	case "json":
		return writeJSON(w, res, version)
	case "sarif":
		return writeSARIF(w, res, version)
	case "github":
		return writeGitHub(w, res)
	}
	return fmt.Errorf("unknown format %q (use %s)", format, strings.Join(Formats, ", "))
}

func writeText(w io.Writer, res *engine.Result) error {
	if len(res.Files) == 0 {
		_, err := fmt.Fprintln(w, "No agent rules files found.")
		return err
	}
	for _, f := range res.Findings {
		fmt.Fprintf(w, "%s:%d: %s %s %s: %s\n", f.Path, f.Line, f.Severity, f.RuleID, f.RuleName, f.Message)
		if f.Suggestion != "" {
			fmt.Fprintf(w, "    fix: %s\n", f.Suggestion)
		}
	}
	e, wn, in := res.Counts()
	if len(res.Findings) > 0 {
		fmt.Fprintln(w)
	}
	_, err := fmt.Fprintf(w, "%d %s, %d %s, %d info in %d rules %s (always-on: ~%d tokens)\n",
		e, plural(e, "error"), wn, plural(wn, "warning"), in, len(res.Files), plural(len(res.Files), "file"), res.AlwaysOnTokens)
	return err
}

func plural(n int, s string) string {
	if n == 1 {
		return s
	}
	return s + "s"
}

type jsonFinding struct {
	core.Finding
	SeverityS string `json:"severity"`
}

type jsonFile struct {
	Path     string `json:"path"`
	Tool     string `json:"tool"`
	Kind     string `json:"kind"`
	AlwaysOn bool   `json:"always_on"`
	Tokens   int    `json:"tokens"`
	Imported string `json:"imported_by,omitempty"`
}

type jsonReport struct {
	Tool     string        `json:"tool"`
	Version  string        `json:"version"`
	Summary  jsonSummary   `json:"summary"`
	Files    []jsonFile    `json:"files"`
	Findings []jsonFinding `json:"findings"`
}

type jsonSummary struct {
	Files          int `json:"files"`
	Errors         int `json:"errors"`
	Warnings       int `json:"warnings"`
	Info           int `json:"info"`
	AlwaysOnTokens int `json:"always_on_tokens"`
}

func writeJSON(w io.Writer, res *engine.Result, version string) error {
	e, wn, in := res.Counts()
	rep := jsonReport{Tool: ToolName, Version: version, Files: []jsonFile{}, Findings: []jsonFinding{},
		Summary: jsonSummary{Files: len(res.Files), Errors: e, Warnings: wn, Info: in, AlwaysOnTokens: res.AlwaysOnTokens}}
	for _, f := range res.Files {
		rep.Files = append(rep.Files, jsonFile{f.Path, f.Tool, string(f.Kind), f.AlwaysOn, f.Tokens(), f.ImportedBy})
	}
	for _, f := range res.Findings {
		rep.Findings = append(rep.Findings, jsonFinding{f, f.Severity.String()})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(rep)
}

func sarifLevel(s core.Severity) string {
	switch s {
	case core.SevError:
		return "error"
	case core.SevWarning:
		return "warning"
	}
	return "note"
}

func writeSARIF(w io.Writer, res *engine.Result, version string) error {
	type msg struct {
		Text string `json:"text"`
	}
	type rule struct {
		ID               string         `json:"id"`
		Name             string         `json:"name"`
		ShortDescription msg            `json:"shortDescription"`
		HelpURI          string         `json:"helpUri"`
		DefaultConfig    map[string]any `json:"defaultConfiguration"`
	}
	index := map[string]int{}
	rulesOut := []rule{}
	for i, r := range res.Rules {
		index[r.ID] = i
		rulesOut = append(rulesOut, rule{r.ID, r.Name, msg{r.Summary}, DocURL(r.ID), map[string]any{"level": sarifLevel(r.Default)}})
	}
	results := []map[string]any{}
	for _, f := range res.Findings {
		text := f.Message
		if f.Suggestion != "" {
			text += " Fix: " + f.Suggestion
		}
		results = append(results, map[string]any{
			"ruleId":    f.RuleID,
			"ruleIndex": index[f.RuleID],
			"level":     sarifLevel(f.Severity),
			"message":   msg{text},
			"locations": []any{map[string]any{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{"uri": f.Path, "uriBaseId": "%SRCROOT%"},
					"region":           map[string]any{"startLine": f.Line},
				},
			}},
		})
	}
	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []any{map[string]any{
			"tool": map[string]any{"driver": map[string]any{
				"name": ToolName, "version": version, "informationUri": RepoURL, "rules": rulesOut,
			}},
			"results": results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

func ghEscapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func ghEscapeProp(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

func writeGitHub(w io.Writer, res *engine.Result) error {
	for _, f := range res.Findings {
		level := "notice"
		switch f.Severity {
		case core.SevError:
			level = "error"
		case core.SevWarning:
			level = "warning"
		}
		text := f.Message
		if f.Suggestion != "" {
			text += " Fix: " + f.Suggestion
		}
		if _, err := fmt.Fprintf(w, "::%s file=%s,line=%d,title=%s::%s\n", level, ghEscapeProp(f.Path), f.Line, ghEscapeProp(f.RuleID+" "+f.RuleName), ghEscapeData(text)); err != nil {
			return err
		}
	}
	return nil
}
