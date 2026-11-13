// Package discover finds agent rules files in a repository and parses them.
package discover

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	ignore "github.com/sabhiram/go-gitignore"
	"gopkg.in/yaml.v3"

	"github.com/kellyadamtan/kogentic-rules-lint/internal/core"
)

// Result is the output of Discover.
type Result struct {
	Files     []*core.File
	RepoFiles []string
	// ReadErrors lists paths that could not be read (unreadable rules files,
	// imports, .gitignore files or directories). Lint results are incomplete
	// when it is not empty.
	ReadErrors []ReadError
}

// ReadError is one path that could not be read.
type ReadError struct {
	Path string // slash-separated, relative to the root
	Err  error
}

func (e ReadError) Error() string {
	err := e.Err
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	return e.Path + ": " + err.Error()
}

// alwaysSkip are directory names never walked.
var alwaysSkip = map[string]bool{".git": true, "node_modules": true}

// Discover walks root, honouring .gitignore files (root and nested), and
// returns the rules files it recognises plus the list of all non-ignored files.
func Discover(root string, extra, ignoreGlobs []string) (*Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, errors.New(root + " is not a directory")
	}

	matchers := map[string]*ignore.GitIgnore{} // dir (slash rel, "." for root) -> matcher
	ignored := func(rel string, isDir bool) bool {
		for _, g := range ignoreGlobs {
			if ok, _ := doublestar.Match(g, rel); ok {
				return true
			}
			if isDir {
				if ok, _ := doublestar.Match(g, rel+"/x"); ok {
					return true
				}
			}
		}
		dir := path.Dir(rel)
		for {
			if m, ok := matchers[dir]; ok {
				sub := rel
				if dir != "." {
					sub = strings.TrimPrefix(rel, dir+"/")
				}
				if isDir {
					sub += "/"
				}
				if m.MatchesPath(sub) {
					return true
				}
			}
			if dir == "." || dir == "/" || dir == "" {
				break
			}
			dir = path.Dir(dir)
		}
		return false
	}
	res := &Result{}
	readErr := func(rel string, err error) {
		res.ReadErrors = append(res.ReadErrors, ReadError{Path: rel, Err: err})
	}
	loadIgnore := func(relDir, absDir string) {
		p := filepath.Join(absDir, ".gitignore")
		b, err := os.ReadFile(p)
		if err == nil {
			matchers[relDir] = ignore.CompileIgnoreLines(strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")...)
		} else if !errors.Is(err, fs.ErrNotExist) {
			readErr(path.Join(relDir, ".gitignore"), err)
		}
	}

	var candidates []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if p == root {
				return werr
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			// An unreadable directory also failed its .gitignore read just
			// before; report the directory once.
			if n := len(res.ReadErrors); n > 0 && res.ReadErrors[n-1].Path == path.Join(rel, ".gitignore") {
				res.ReadErrors = res.ReadErrors[:n-1]
			}
			readErr(rel, werr) // unreadable entry: report, keep walking
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "." {
				loadIgnore(".", p)
				return nil
			}
			if alwaysSkip[d.Name()] || ignored(rel, true) {
				return filepath.SkipDir
			}
			loadIgnore(rel, p)
			return nil
		}
		if !d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0 {
			return nil
		}
		if ignored(rel, false) {
			return nil
		}
		res.RepoFiles = append(res.RepoFiles, rel)
		if _, _, ok := Classify(rel); ok || matchAny(extra, rel) {
			candidates = append(candidates, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(res.RepoFiles)

	seen := map[string]bool{}
	for _, rel := range candidates {
		tool, kind, ok := Classify(rel)
		if !ok {
			tool, kind = "extra", core.KindExtra
		}
		f, err := Load(root, rel, tool, kind)
		if err != nil {
			readErr(rel, err)
			continue
		}
		seen[rel] = true
		res.Files = append(res.Files, f)
	}

	// Follow @imports from CLAUDE.md files one level deep.
	var imported []*core.File
	for _, f := range res.Files {
		if f.Kind != core.KindClaudeMemory {
			continue
		}
		for _, im := range f.Imports {
			if !im.Exists || im.External || seen[im.Resolved] || strings.HasPrefix(im.Resolved, "../") {
				continue
			}
			nf, err := Load(root, im.Resolved, "claude", core.KindClaudeImport)
			if err != nil {
				seen[im.Resolved] = true
				readErr(im.Resolved, err)
				continue
			}
			nf.ImportedBy = f.Path
			nf.AlwaysOn = f.AlwaysOn
			seen[im.Resolved] = true
			imported = append(imported, nf)
		}
	}
	res.Files = append(res.Files, imported...)
	sort.SliceStable(res.Files, func(i, j int) bool { return res.Files[i].Path < res.Files[j].Path })
	sort.SliceStable(res.ReadErrors, func(i, j int) bool { return res.ReadErrors[i].Path < res.ReadErrors[j].Path })
	return res, nil
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if ok, _ := doublestar.Match(g, rel); ok {
			return true
		}
	}
	return false
}

var rooRe = regexp.MustCompile(`^\.roo/rules(-[A-Za-z0-9_-]+)?/`)

// Classify maps a slash path relative to the repo root to a tool and kind.
func Classify(rel string) (tool string, kind core.Kind, ok bool) {
	base := path.Base(rel)
	switch {
	case rel == "CLAUDE.md" || rel == "CLAUDE.local.md" || rel == ".claude/CLAUDE.md":
		return "claude", core.KindClaudeMemory, true
	case strings.HasPrefix(rel, ".claude/rules/") && strings.HasSuffix(rel, ".md"):
		return "claude", core.KindClaudeRule, true
	case base == "AGENTS.md":
		return "agents", core.KindAgents, true
	case rel == "GEMINI.md":
		return "gemini", core.KindGemini, true
	case strings.HasPrefix(rel, ".cursor/rules/") && strings.HasSuffix(rel, ".mdc"):
		return "cursor", core.KindCursorRule, true
	case strings.HasPrefix(rel, ".cursor/rules/") && strings.HasSuffix(rel, ".md"):
		return "cursor", core.KindCursorRuleMD, true
	case rel == ".cursorrules":
		return "cursor", core.KindCursorLegacy, true
	case rel == ".github/copilot-instructions.md":
		return "copilot", core.KindCopilotRepo, true
	case strings.HasPrefix(rel, ".github/instructions/") && strings.HasSuffix(rel, ".instructions.md"):
		return "copilot", core.KindCopilotInstr, true
	case rel == ".windsurfrules":
		return "windsurf", core.KindWindsurfLegacy, true
	case path.Dir(rel) == ".windsurf/rules" && strings.HasSuffix(rel, ".md"):
		return "windsurf", core.KindWindsurfRule, true
	case rel == ".clinerules" || (strings.HasPrefix(rel, ".clinerules/") && (strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".txt"))):
		return "cline", core.KindCline, true
	case rooRe.MatchString(rel) && (strings.HasSuffix(rel, ".md") || strings.HasSuffix(rel, ".txt")):
		return "roo", core.KindRoo, true
	}
	return "", "", false
}

// Load reads and parses one rules file.
func Load(root, rel, tool string, kind core.Kind) (*core.File, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	f := &core.File{Path: rel, Abs: abs, Tool: tool, Kind: kind, Size: len(b)}
	f.Lines = strings.Split(text, "\n")
	if n := len(f.Lines); n > 0 && f.Lines[n-1] == "" {
		f.Lines = f.Lines[:n-1]
	}
	f.FM, f.BodyStart = parseFrontmatter(f.Lines, kind)
	f.Instructions, f.Directives = Extract(f.Lines, f.BodyStart)
	f.AlwaysOn = alwaysOn(f)
	if kind == core.KindClaudeMemory {
		f.Imports = findImports(root, f)
	}
	return f, nil
}

func alwaysOn(f *core.File) bool {
	switch f.Kind {
	case core.KindClaudeMemory, core.KindGemini, core.KindCursorLegacy, core.KindCopilotRepo,
		core.KindWindsurfLegacy, core.KindCline, core.KindRoo:
		return true
	case core.KindAgents:
		return f.Path == "AGENTS.md"
	case core.KindCursorRule:
		v, _ := f.FM.Data["alwaysApply"].(bool)
		return v
	case core.KindClaudeRule:
		_, has := f.FM.Data["paths"]
		return f.FM.Err == nil && !has
	case core.KindCopilotInstr:
		v, _ := f.FM.Data["applyTo"].(string)
		v = strings.TrimSpace(v)
		return v == "**" || v == "**/*"
	case core.KindWindsurfRule:
		v, _ := f.FM.Data["trigger"].(string)
		return v == "always_on"
	}
	return false
}

var unquotedGlobs = regexp.MustCompile(`^(globs:\s*)([^"'\[\s].*)$`)

func parseFrontmatter(lines []string, kind core.Kind) (core.Frontmatter, int) {
	fm := core.Frontmatter{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, 1
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return fm, 1
	}
	fm.Present = true
	fm.StartLine = 1
	fm.EndLine = end + 1
	raw := lines[1:end]
	fm.Raw = strings.Join(raw, "\n")
	src := raw
	if kind == core.KindCursorRule || kind == core.KindWindsurfRule {
		// Cursor and Windsurf accept unquoted globs such as `globs: *.tsx`,
		// which is not valid YAML. Quote them before parsing.
		src = make([]string, len(raw))
		for i, l := range raw {
			if m := unquotedGlobs.FindStringSubmatch(l); m != nil {
				l = m[1] + "'" + strings.ReplaceAll(strings.TrimSpace(m[2]), "'", "''") + "'"
			}
			src[i] = l
		}
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(src, "\n")), &node); err != nil {
		fm.Err = err
		return fm, end + 2
	}
	fm.Data = map[string]any{}
	fm.KeyLines = map[string]int{}
	if len(node.Content) == 0 {
		return fm, end + 2
	}
	doc := node.Content[0]
	if doc.Kind != yaml.MappingNode {
		fm.Err = errors.New("frontmatter is not a key: value mapping")
		return fm, end + 2
	}
	if err := doc.Decode(&fm.Data); err != nil {
		fm.Err = err
		return fm, end + 2
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		fm.KeyLines[doc.Content[i].Value] = doc.Content[i].Line + 1
	}
	return fm, end + 2
}

var importRe = regexp.MustCompile(`(?:^|\s)@([A-Za-z0-9_.~/\-]+)`)
var codeSpanRe = regexp.MustCompile("`[^`]*`")

func findImports(root string, f *core.File) []core.Import {
	var out []core.Import
	inFence := false
	for i := f.BodyStart - 1; i < len(f.Lines); i++ {
		line := f.Lines[i]
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		line = codeSpanRe.ReplaceAllString(line, "")
		for _, m := range importRe.FindAllStringSubmatch(line, -1) {
			tok := strings.TrimRight(m[1], ".,;:)")
			if !strings.ContainsAny(tok, "./") || tok == "" {
				continue
			}
			im := core.Import{Raw: "@" + tok, Line: i + 1}
			if strings.HasPrefix(tok, "~") || strings.HasPrefix(tok, "/") {
				im.External = true
				out = append(out, im)
				continue
			}
			im.Resolved = path.Clean(path.Join(path.Dir(f.Path), tok))
			if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(im.Resolved))); err == nil && !st.IsDir() {
				im.Exists = true
			}
			out = append(out, im)
		}
	}
	return out
}
