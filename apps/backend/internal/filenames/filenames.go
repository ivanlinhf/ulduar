package filenames

import (
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func Sanitize(filename, fallback string) string {
	name := strings.TrimSpace(filepath.Base(filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fallback
	}

	safe := unsafeChars.ReplaceAllString(name, "-")
	safe = strings.Trim(safe, "-.")
	if safe == "" {
		return fallback
	}

	return safe
}
