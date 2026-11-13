// Package config loads .kogentic-rules-lint.yaml.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName is the default config file name, looked up in the lint root.
const FileName = ".kogentic-rules-lint.yaml"

// Config is the user configuration.
type Config struct {
	Version int               `yaml:"version"`
	Rules   map[string]string `yaml:"rules"`
	Budget  struct {
		AlwaysOnTokens int `yaml:"always_on_tokens"`
	} `yaml:"budget"`
	Paths struct {
		Extra  []string `yaml:"extra"`
		Ignore []string `yaml:"ignore"`
	} `yaml:"paths"`
}

// Default is the configuration used when no file exists.
func Default() *Config {
	c := &Config{Version: 1, Rules: map[string]string{}}
	c.Budget.AlwaysOnTokens = 2000
	return c
}

// DefaultYAML is what `init` writes.
const DefaultYAML = `# kogentic-rules-lint configuration
# Docs: https://github.com/kellyadamtan/kogentic-rules-lint#configuration
version: 1

# Per-rule severity: off, info, warning or error.
rules:
  # V003: warning
  # E001: info

# Estimated always-on tokens allowed per tool (S002).
budget:
  always_on_tokens: 2000

paths:
  # Extra files to lint as rules files (globs, relative to the repo root).
  extra: []
  # Paths to skip, in addition to .gitignore.
  ignore: []
`

// Load reads the config from explicit, or from root/FileName if explicit is
// empty. A missing default file is not an error.
func Load(root, explicit string) (*Config, string, error) {
	p := explicit
	if p == "" {
		p = filepath.Join(root, FileName)
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			return Default(), "", nil
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, p, err
	}
	c := Default()
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return nil, p, fmt.Errorf("%s: %w", p, err)
	}
	if c.Version != 1 {
		return nil, p, fmt.Errorf("%s: unsupported version %d (expected 1)", p, c.Version)
	}
	if c.Rules == nil {
		c.Rules = map[string]string{}
	}
	return c, p, nil
}
