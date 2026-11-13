package rules

import "github.com/kellyadamtan/kogentic-rules-lint/internal/core"

// knownKeys lists the frontmatter keys each vendor reads.
var knownKeys = map[core.Kind]map[string]bool{
	core.KindCursorRule:   {"description": true, "globs": true, "alwaysApply": true},
	core.KindClaudeRule:   {"paths": true, "description": true},
	core.KindCopilotInstr: {"applyTo": true, "description": true, "excludeAgent": true},
	core.KindWindsurfRule: {"trigger": true, "globs": true, "description": true},
}

// vendorKeys are keys that belong to some vendor; seeing one in the wrong
// vendor's file is F003, not F002.
var vendorKeys = map[string]string{
	"alwaysApply":  "Cursor",
	"globs":        "Cursor/Windsurf",
	"paths":        "Claude Code",
	"applyTo":      "Copilot",
	"excludeAgent": "Copilot",
	"trigger":      "Windsurf",
	"description":  "",
}

var windsurfTriggers = map[string]bool{"always_on": true, "manual": true, "model_decision": true, "glob": true}
