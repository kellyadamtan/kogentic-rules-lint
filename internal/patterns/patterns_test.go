package patterns

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		text  string
		kind  Kind
		paths []string
		cmds  []string
	}{
		{"Never edit files in `dist/`.", PathDeny, []string{"dist/"}, nil},
		{"Don't modify generated/ by hand", PathDeny, []string{"generated/"}, nil},
		{"Do not touch `package-lock.json`", PathDeny, []string{"package-lock.json"}, nil},
		{"Don't run `rm -rf`.", CommandDeny, nil, []string{"rm -rf"}},
		{"Never force-push to main.", CommandDeny, nil, []string{"git push --force"}},
		{"Never commit .env files.", SecretDeny, []string{".env"}, nil},
		{"Do not print API keys in logs", SecretDeny, nil, nil},
		{"Always run `make test` before committing.", RunBeforeCommit, nil, []string{"make test"}},
		{"Every change to `src/*.ts` needs a test.", MustHaveTest, []string{"src/*.ts"}, nil},
		{"Write clean code.", None, nil, nil},
		{"Never use any.", None, nil, nil},
		{"Run tests before committing.", None, nil, nil},
	}
	for _, c := range cases {
		p := Classify(c.text)
		if p.Kind != c.kind {
			t.Errorf("Classify(%q) = %v, want %v", c.text, p.Kind, c.kind)
			continue
		}
		if c.paths != nil && !reflect.DeepEqual(p.Paths, c.paths) {
			t.Errorf("Classify(%q).Paths = %v, want %v", c.text, p.Paths, c.paths)
		}
		if c.cmds != nil && !reflect.DeepEqual(p.Commands, c.cmds) {
			t.Errorf("Classify(%q).Commands = %v, want %v", c.text, p.Commands, c.cmds)
		}
	}
}

func TestPathsAndLooksLikePath(t *testing.T) {
	got := Paths("Components in `*.tsx` and src/api/ use `UserCard`, see https://x.test/a/b and `npm test`.")
	want := []string{"*.tsx", "src/api/"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Paths = %v, want %v", got, want)
	}
	for tok, want := range map[string]bool{"src/a.ts": true, ".env": true, "package.json": true, "*.go": true, "npm test": false, "UserCard": false, "--force": false, "https://x/y": false} {
		if LooksLikePath(tok) != want {
			t.Errorf("LooksLikePath(%q) != %v", tok, want)
		}
	}
	if !IsNegative("Don't do it") || IsNegative("Do it") {
		t.Error("IsNegative")
	}
}
