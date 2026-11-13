package discover

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

var (
	headingRe   = regexp.MustCompile(`^#{1,6}\s+(.*?)\s*#*\s*$`)
	listItemRe  = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)
	checkboxRe  = regexp.MustCompile(`^\[[ xX]\]\s+`)
	directiveRe = regexp.MustCompile(`<!--\s*rules-lint-(disable-next-line|disable|enable)\b([^>]*?)-->`)
	ruleIDRe    = regexp.MustCompile(`[A-Z]\d{3}`)
)

// Extract splits a markdown body (starting at 1-based line start) into
// instructions (list items and sentences) and inline directives. Code fences,
// headings, tables and HTML comments are not instructions.
func Extract(lines []string, start int) ([]core.Instruction, []core.Directive) {
	var ins []core.Instruction
	var dirs []core.Directive
	section, sectionLine := "", 0
	inFence, inComment := false, false
	lastWasItem := false
	var pendingNext []int // indexes in dirs of disable-next-line waiting for a target

	for i := start - 1; i >= 0 && i < len(lines); i++ {
		ln := i + 1
		raw := lines[i]
		t := strings.TrimSpace(raw)

		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			lastWasItem = false
			continue
		}
		if inFence {
			continue
		}
		for _, m := range directiveRe.FindAllStringSubmatch(raw, -1) {
			d := core.Directive{Line: ln, Kind: m[1], Rules: ruleIDRe.FindAllString(m[2], -1)}
			rest := strings.TrimSpace(directiveRe.ReplaceAllString(raw, ""))
			if d.Kind == "disable" && rest != "" {
				d.Kind, d.Target = "disable-line", ln
			}
			if d.Kind == "disable-next-line" {
				d.Kind = "disable-line"
				pendingNext = append(pendingNext, len(dirs))
			}
			dirs = append(dirs, d)
		}
		// Strip HTML comments (possibly multi-line).
		if inComment {
			if idx := strings.Index(t, "-->"); idx >= 0 {
				inComment = false
				t = strings.TrimSpace(t[idx+3:])
			} else {
				continue
			}
		}
		for {
			s := strings.Index(t, "<!--")
			if s < 0 {
				break
			}
			e := strings.Index(t[s:], "-->")
			if e < 0 {
				t = strings.TrimSpace(t[:s])
				inComment = true
				break
			}
			t = strings.TrimSpace(t[:s] + " " + t[s+e+3:])
		}
		if t == "" {
			lastWasItem = false
			continue
		}
		for _, k := range pendingNext {
			dirs[k].Target = ln
		}
		pendingNext = nil

		if m := headingRe.FindStringSubmatch(t); m != nil {
			section, sectionLine = m[1], ln
			lastWasItem = false
			continue
		}
		if strings.HasPrefix(t, "|") || t == "---" || t == "***" {
			lastWasItem = false
			continue
		}
		t = strings.TrimSpace(strings.TrimLeft(t, ">"))
		if m := listItemRe.FindStringSubmatch(t); m != nil {
			text := checkboxRe.ReplaceAllString(strings.TrimSpace(m[1]), "")
			ins = append(ins, newInstruction(text, ln, section, sectionLine))
			lastWasItem = true
			continue
		}
		if lastWasItem && len(raw) > 0 && (raw[0] == ' ' || raw[0] == '\t') && len(ins) > 0 {
			last := &ins[len(ins)-1]
			last.Text += " " + t
			last.Norm = Normalize(last.Text)
			continue
		}
		lastWasItem = false
		for _, s := range splitSentences(t) {
			ins = append(ins, newInstruction(s, ln, section, sectionLine))
		}
	}
	return ins, dirs
}

func newInstruction(text string, line int, section string, sectionLine int) core.Instruction {
	return core.Instruction{Text: text, Norm: Normalize(text), Line: line, Section: section, SectionLine: sectionLine}
}

// Normalize lower-cases text, strips emphasis markers and collapses spaces.
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("**", "", "__", "", "’", "'", "‘", "'", "“", "\"", "”", "\"").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// splitSentences splits on . ! ? followed by whitespace and an upper-case
// letter, a backtick or a quote. "e.g. foo" stays together.
func splitSentences(s string) []string {
	var out []string
	r := []rune(s)
	startIdx := 0
	for i := 0; i < len(r)-2; i++ {
		if r[i] != '.' && r[i] != '!' && r[i] != '?' {
			continue
		}
		if !unicode.IsSpace(r[i+1]) {
			continue
		}
		j := i + 1
		for j < len(r) && unicode.IsSpace(r[j]) {
			j++
		}
		if j >= len(r) {
			break
		}
		if unicode.IsUpper(r[j]) || r[j] == '`' || r[j] == '"' {
			if seg := strings.TrimSpace(string(r[startIdx : i+1])); seg != "" {
				out = append(out, seg)
			}
			startIdx = j
			i = j - 1
		}
	}
	if seg := strings.TrimSpace(string(r[startIdx:])); seg != "" {
		out = append(out, seg)
	}
	return out
}
