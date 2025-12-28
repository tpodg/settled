package taskutil

import (
	"fmt"
	"os"
)

const (
	warnColor  = "\x1b[33m"
	colorReset = "\x1b[0m"
)

// Warnf prints a formatted warning to stdout with a colored prefix.
func Warnf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "%sWARN: %s%s\n", warnColor, message, colorReset)
}
