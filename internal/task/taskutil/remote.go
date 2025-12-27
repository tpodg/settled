package taskutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
)

const missingFileSentinel = "__SETTLED_MISSING__"

func SudoPrefix(ctx context.Context, s server.Server) (string, error) {
	output, err := s.Execute(ctx, "id -u")
	if err != nil {
		return "", fmt.Errorf("check for root user: %w", err)
	}
	if strings.TrimSpace(output) == "0" {
		return "", nil
	}
	return "sudo -n ", nil
}

func ReadFileIfExists(ctx context.Context, s server.Server, prefix, path string) (string, bool, error) {
	marker := missingFileSentinel + ":" + path
	pathEsc := strutil.ShellEscape(path)
	script := fmt.Sprintf(
		"if [ -f %s ]; then cat %s; else printf '%%s' %s; fi",
		pathEsc,
		pathEsc,
		strutil.ShellEscape(marker),
	)
	cmd := prefix + "sh -c " + strutil.ShellEscape(script)
	output, err := s.Execute(ctx, cmd)
	if err != nil {
		return "", false, fmt.Errorf("read file %q: %w", path, err)
	}
	if strings.TrimSpace(output) == marker {
		return "", true, nil
	}
	return output, false, nil
}
