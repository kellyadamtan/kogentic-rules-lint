package engine

import (
	"strconv"
	"strings"
)

func fmtInt(i int) string    { return strconv.Itoa(i) }
func join(s []string) string { return strings.Join(s, " ") }
