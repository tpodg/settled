package cli

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/testutils"
)

func TestVerifyServers_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	servers := []server.Server{
		server.NewSSHServer("integration-server", sshC.Address, sshC.User, sshC.KeyPath, sshC.KnownHostsPath),
	}

	verifyServers(logger, servers)

	output := buf.String()
	if !strings.Contains(output, "Verification successful") {
		t.Errorf("expected logs to contain 'Verification successful', got:\n%s", output)
	}
	if !strings.Contains(output, "server=integration-server") {
		t.Errorf("expected logs to contain 'server=integration-server', got:\n%s", output)
	}
}
