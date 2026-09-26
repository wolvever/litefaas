package stackpack

import (
	"path/filepath"
	"regexp"
	"strings"
)

var nameSafe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// NameFromDir is a resource name derived from the directory basename.
func NameFromDir(dir string) string {
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	base := filepath.Base(abs)
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "app"
	}
	if nameSafe.MatchString(base) {
		return base
	}
	var b strings.Builder
	for i, r := range base {
		switch {
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			if i > 0 {
				b.WriteRune(r)
			}
		default:
			if b.Len() > 0 {
				b.WriteByte('-')
			}
		}
	}
	s := strings.Trim(b.String(), "-._")
	if s == "" || !nameSafe.MatchString(s) {
		return "app"
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
