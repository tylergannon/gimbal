package effecthelper

import (
	"os"
	"strings"
)

func Pure(value string) string { return strings.TrimSpace(value) }
func Effect()                  { _, _ = os.ReadFile("mutable") }
