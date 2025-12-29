package sshpasswordauth_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/sshd"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/sshpasswordauth"
	"github.com/tpodg/settled/internal/task/taskutil"
	"github.com/tpodg/settled/internal/testutils"
	tasktests "github.com/tpodg/settled/internal/testutils/task"
)

func TestSSHPasswordAuthTask_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready.
	time.Sleep(2 * time.Second)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runner := task.NewRunner(logger)

	srv := server.NewSSHServer("sshpwauth-nonroot", sshC.Address, "testuser", sshC.KeyPath, sshC.KnownHostsPath, server.SSHOptions{})
	tasks := tasktests.PlanTasks(t, map[string]any{}, sshpasswordauth.Spec())

	tasktests.AssertTasksNeedExecution(t, ctx, srv, tasks)
	if err := runner.Run(ctx, srv, tasks...); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	tasktests.WaitForLogin(t, ctx, srv, "testuser")

	settings := readSSHDSettings(t, ctx, srv)
	passwordKey := strings.ToLower(sshd.KeyPasswordAuthentication)
	kbdKey := strings.ToLower(sshd.KeyKbdInteractiveAuth)
	challengeKey := strings.ToLower(sshd.KeyChallengeResponseAuth)

	if settings[passwordKey] != sshd.ValueNo {
		t.Fatalf("expected PasswordAuthentication to be %q, got %q", sshd.ValueNo, settings[passwordKey])
	}
	if settings[kbdKey] != sshd.ValueNo {
		t.Fatalf("expected KbdInteractiveAuthentication to be %q, got %q", sshd.ValueNo, settings[kbdKey])
	}
	if settings[challengeKey] != sshd.ValueNo {
		t.Fatalf("expected ChallengeResponseAuthentication to be %q, got %q", sshd.ValueNo, settings[challengeKey])
	}

	tasktests.AssertTasksSatisfied(t, ctx, srv, tasks)
}

func readSSHDSettings(t *testing.T, ctx context.Context, srv server.Server) map[string]string {
	t.Helper()

	_, output, err := sshd.ReadConfig(ctx, srv)
	if err != nil {
		t.Fatalf("read sshd_config failed: %v", err)
	}

	settings, err := taskutil.ParseKeyValueSettings(output)
	if err != nil {
		t.Fatalf("scan sshd_config failed: %v", err)
	}

	return settings
}
