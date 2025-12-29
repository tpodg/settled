package sshpasswordauth_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
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
	if settings["passwordauthentication"] != "no" {
		t.Fatalf("expected PasswordAuthentication to be %q, got %q", "no", settings["passwordauthentication"])
	}
	if settings["kbdinteractiveauthentication"] != "no" {
		t.Fatalf("expected KbdInteractiveAuthentication to be %q, got %q", "no", settings["kbdinteractiveauthentication"])
	}
	if settings["challengeresponseauthentication"] != "no" {
		t.Fatalf("expected ChallengeResponseAuthentication to be %q, got %q", "no", settings["challengeresponseauthentication"])
	}

	tasktests.AssertTasksSatisfied(t, ctx, srv, tasks)
}

func readSSHDSettings(t *testing.T, ctx context.Context, srv server.Server) map[string]string {
	t.Helper()

	prefix, err := taskutil.SudoPrefix(ctx, srv)
	if err != nil {
		t.Fatalf("SudoPrefix failed: %v", err)
	}

	output, err := srv.Execute(ctx, prefix+"cat /etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("read sshd_config failed: %v", err)
	}

	settings, err := taskutil.ParseKeyValueSettings(output)
	if err != nil {
		t.Fatalf("scan sshd_config failed: %v", err)
	}

	return settings
}
