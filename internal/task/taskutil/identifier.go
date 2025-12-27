package taskutil

import (
	"fmt"
	"strings"
)

func SanitizeFilename(value, fallback string) string {
	if value == "" {
		return fallback
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if isSafeNameRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	return b.String()
}

func ValidateIdentifier(kind, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s name cannot be empty", kind)
	}
	if trimmed != value {
		return fmt.Errorf("%s name %q has leading/trailing whitespace", kind, value)
	}
	for _, r := range value {
		if !isSafeNameRune(r) {
			return fmt.Errorf("%s name %q contains invalid character %q", kind, value, r)
		}
	}
	return nil
}

func isSafeNameRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		r == '-' || r == '_' || r == '.'
}
