package engine

import (
	"os/exec"
	"strings"
	"testing"
)

// forbidden are packages the lint path must never depend on, directly or
// through a dependency: networking, URL handling and running other programs.
var forbidden = map[string]bool{"net": true, "net/http": true, "net/url": true, "os/exec": true}

// TestLinterPathIsOffline guards the "no network, no telemetry" promise: the
// packages on the lint path must not depend on net, net/http, net/url or
// os/exec.
func TestLinterPathIsOffline(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}
	for _, pkg := range []string{"../discover", "../rules", "../engine", "../report", "../patterns", "../config", "../core"} {
		out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", pkg, err, out)
		}
		for _, dep := range strings.Fields(string(out)) {
			if forbidden[dep] {
				t.Errorf("%s depends on %s; the linter path must stay offline and must not run other programs", pkg, dep)
			}
		}
	}
}
