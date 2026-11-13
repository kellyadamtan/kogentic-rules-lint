package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

// DuplicateThreshold is the shingle (word-bigram Jaccard) similarity at or
// above which two instructions are near-duplicates.
const DuplicateThreshold = 0.85

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func words(norm string) []string { return strings.Fields(nonWord.ReplaceAllString(norm, " ")) }

func shingles(ws []string) map[string]bool {
	s := map[string]bool{}
	if len(ws) == 1 {
		s[ws[0]] = true
	}
	for i := 0; i+1 < len(ws); i++ {
		s[ws[i]+" "+ws[i+1]] = true
	}
	return s
}

// Similarity is the Jaccard similarity of the word-bigram sets of a and b.
func Similarity(a, b string) float64 {
	sa, sb := shingles(words(a)), shingles(words(b))
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for k := range sa {
		if sb[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(sa)+len(sb)-inter)
}

func init() {
	r := &core.Rule{ID: "C001", Name: "duplicate", Default: core.SevWarning,
		Summary: "Near-duplicate instructions within or across files."}
	r.Check = func(c *core.Context) []core.Finding {
		type item struct {
			f  *core.File
			in core.Instruction
			ws string
		}
		var items []item
		for _, f := range instructionFiles(c) {
			for _, in := range f.Instructions {
				ws := words(in.Norm)
				if len(ws) < 4 {
					continue
				}
				items = append(items, item{f, in, strings.Join(ws, " ")})
			}
		}
		var out []core.Finding
		for j := 1; j < len(items); j++ {
			for i := 0; i < j; i++ {
				a, b := items[i], items[j]
				if a.f == b.f && a.in.Line == b.in.Line {
					continue
				}
				sim := 1.0
				if a.ws != b.ws {
					sim = Similarity(a.ws, b.ws)
				}
				if sim < DuplicateThreshold {
					continue
				}
				keep := a.f.Path
				if a.f == b.f {
					keep = fmt.Sprintf("line %d", a.in.Line)
				}
				out = append(out, finding(r, b.f, b.in.Line,
					fmt.Sprintf("%s duplicates %s:%d (similarity %.2f).", quote(b.in.Text), a.f.Path, a.in.Line, sim),
					"Keep one copy ("+keep+") and delete this one, or import the shared file instead of copying it.", b.in.Text))
				break
			}
		}
		return out
	}
	register(r)
}
