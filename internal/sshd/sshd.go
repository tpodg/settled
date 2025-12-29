package sshd

import (
	"context"
	"fmt"
	"strings"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task/taskutil"
)

const (
	DefaultConfigPath         = "/etc/ssh/sshd_config"
	KeyPermitRootLogin        = "PermitRootLogin"
	KeyPasswordAuthentication = "PasswordAuthentication"
	KeyKbdInteractiveAuth     = "KbdInteractiveAuthentication"
	KeyChallengeResponseAuth  = "ChallengeResponseAuthentication"
	ValueNo                   = "no"
)

var configPaths = []string{
	DefaultConfigPath,
}

func ReadConfig(ctx context.Context, s server.Server) (string, string, error) {
	prefix, err := taskutil.SudoPrefix(ctx, s)
	if err != nil {
		return "", "", err
	}

	for _, path := range configPaths {
		output, missing, err := taskutil.ReadFileIfExists(ctx, s, prefix, path)
		if err != nil {
			return "", "", err
		}
		if missing {
			continue
		}
		return path, output, nil
	}
	return "", "", fmt.Errorf("sshd config not found (checked: %s)", strings.Join(configPaths, ", "))
}
