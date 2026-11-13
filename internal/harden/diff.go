package harden

import (
	"fmt"
	"strings"
)

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// Diff renders a change as a unified diff with the whole file in one hunk.
func Diff(c Change) string {
	a, b := splitLines(c.Old), splitLines(c.New)
	var sb strings.Builder
	if c.Old == "" {
		sb.WriteString("--- /dev/null\n")
	} else {
		sb.WriteString("--- a/" + c.Path + "\n")
	}
	sb.WriteString("+++ b/" + c.Path + "\n")
	if c.Old == c.New {
		sb.WriteString("(no change)\n")
		return sb.String()
	}
	startA, startB := 1, 1
	if len(a) == 0 {
		startA = 0
	}
	if len(b) == 0 {
		startB = 0
	}
	fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", startA, len(a), startB, len(b))
	// LCS table
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			sb.WriteString(" " + a[i] + "\n")
			i++
			j++
		case j < m && (i == n || dp[i][j+1] >= dp[i+1][j]):
			sb.WriteString("+" + b[j] + "\n")
			j++
		default:
			sb.WriteString("-" + a[i] + "\n")
			i++
		}
	}
	return sb.String()
}
