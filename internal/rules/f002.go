package rules

import (
	"fmt"
	"sort"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

func isStringOrList(v any) bool {
	switch t := v.(type) {
	case string:
		return true
	case []any:
		for _, e := range t {
			if _, ok := e.(string); !ok {
				return false
			}
		}
		return true
	}
	return false
}

func init() {
	r := &core.Rule{ID: "F002", Name: "bad-frontmatter", Default: core.SevError,
		Summary: "Invalid YAML, unknown keys or wrong types in rule frontmatter."}
	r.Check = func(c *core.Context) []core.Finding {
		var out []core.Finding
		for _, f := range c.Files {
			known, ok := knownKeys[f.Kind]
			if !ok || !f.FM.Present {
				continue
			}
			if f.FM.Err != nil {
				out = append(out, finding(r, f, f.FM.StartLine, "Frontmatter is not valid YAML: "+f.FM.Err.Error(), "Fix the YAML between the --- lines; quote values that start with * or contain ': '.", f.FM.Raw))
				continue
			}
			keys := make([]string, 0, len(f.FM.Data))
			for k := range f.FM.Data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := f.FM.Data[k]
				line := f.FM.Line(k)
				if !known[k] {
					if _, vendor := vendorKeys[k]; !vendor {
						out = append(out, finding(r, f, line, fmt.Sprintf("Unknown frontmatter key %q; it is ignored.", k), "Remove it or fix the spelling.", k))
					}
					continue
				}
				bad := ""
				switch k {
				case "description", "applyTo", "excludeAgent":
					if _, ok := v.(string); !ok {
						bad = "a string"
					}
				case "alwaysApply":
					if _, ok := v.(bool); !ok {
						bad = "true or false (unquoted)"
					}
				case "globs", "paths":
					if !isStringOrList(v) {
						bad = "a string or a list of strings"
					}
				case "trigger":
					if s, ok := v.(string); !ok || !windsurfTriggers[s] {
						bad = "one of always_on, manual, model_decision, glob"
					}
				}
				if bad != "" {
					out = append(out, finding(r, f, line, fmt.Sprintf("Frontmatter key %q should be %s.", k, bad), "Fix the value type.", fmt.Sprint(v)))
				}
			}
		}
		return out
	}
	register(r)
}
