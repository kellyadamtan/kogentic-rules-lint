package engine

import (
	"os/exec"
	"strings"
	"testing"
)

// TestLinterPathIsOffline guards the "no network, no telemetry" promise: the
// packages on the lint path must not depend on net/http.
func TestLinterPathIsOffline(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}
	for _, pkg := range []string{"../discover", "../rules", "../engine", "../report", "../patterns", "../config"} {
		out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", pkg, err, out)
		}
		for _, dep := range strings.Fields(string(out)) {
			if dep == "net/http" || dep == "net" {
				t.Errorf("%s depends on %s; the linter path must stay offline", pkg, dep)
			}
		}
	}
}
