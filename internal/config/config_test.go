package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	c, p, err := Load(dir, "")
	if err != nil || p != "" || c.Budget.AlwaysOnTokens != 2000 {
		t.Fatalf("default: %v %q %+v", err, p, c)
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, FileName), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(DefaultYAML)
	if c, _, err = Load(dir, ""); err != nil || c.Version != 1 {
		t.Fatalf("DefaultYAML must load: %v", err)
	}
	write("version: 1\nrules:\n  V001: error\nbudget:\n  always_on_tokens: 500\npaths:\n  extra: [\"x/*.md\"]\n  ignore: [\"vendor/**\"]\n")
	c, _, err = Load(dir, "")
	if err != nil || c.Rules["V001"] != "error" || c.Budget.AlwaysOnTokens != 500 || c.Paths.Extra[0] != "x/*.md" || c.Paths.Ignore[0] != "vendor/**" {
		t.Fatalf("custom: %v %+v", err, c)
	}
	write("")
	if _, _, err = Load(dir, ""); err != nil {
		t.Errorf("empty file should be fine: %v", err)
	}
	write("version: 1\nunknown: 1\n")
	if _, _, err = Load(dir, ""); err == nil {
		t.Error("unknown key should fail")
	}
	write("version: 2\n")
	if _, _, err = Load(dir, ""); err == nil {
		t.Error("version 2 should fail")
	}
	if _, _, err = Load(dir, filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("explicit missing file should fail")
	}
}
