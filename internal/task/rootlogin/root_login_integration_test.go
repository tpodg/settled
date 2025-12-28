package rootlogin_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/rootlogin"
	"github.com/tpodg/settled/internal/task/taskutil"
	"github.com/tpodg/settled/internal/testutils"
	tasktests "github.com/tpodg/settled/internal/testutils/task"
)

func TestRootLoginTask_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runner := task.NewRunner(logger)

	t.Run("skips when logged in as root", func(t *testing.T) {
		srv := server.NewSSHServer("rootlogin-root", sshC.Address, "root", sshC.KeyPath, sshC.KnownHostsPath)
		tasks := tasktests.PlanTasks(t, map[string]any{}, rootlogin.Spec())

		before := permitRootLoginValue(t, ctx, srv)
		output := tasktests.CaptureStdout(t, func() {
			if err := runner.Run(ctx, srv, tasks...); err != nil {
				t.Fatalf("Run failed: %v", err)
			}
		})
		if !strings.Contains(output, "\x1b[33mWARN: skipping disable root login task because connected as root.\x1b[0m") {
			t.Fatalf("expected warn output with color, got %q", strings.TrimSpace(output))
		}

		after := permitRootLoginValue(t, ctx, srv)
		if after != before {
			t.Fatalf("expected PermitRootLogin to remain %q, got %q", before, after)
		}
	})

	t.Run("disables when logged in as non-root", func(t *testing.T) {
		srv := server.NewSSHServer("rootlogin-nonroot", sshC.Address, "testuser", sshC.KeyPath, sshC.KnownHostsPath)
		tasks := tasktests.PlanTasks(t, map[string]any{}, rootlogin.Spec())

		assertRootLoginWorks(t, ctx, sshC, "root-login-before")
		if err := runner.Run(ctx, srv, tasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		tasktests.WaitForLogin(t, ctx, srv, "testuser")
		assertRootLoginBlocked(t, ctx, sshC, "root-login-after")
		after := permitRootLoginValue(t, ctx, srv)
		if after != "no" {
			t.Fatalf("expected PermitRootLogin to be %q, got %q", "no", after)
		}
		tasktests.AssertTasksSatisfied(t, ctx, srv, tasks)
	})
}

func permitRootLoginValue(t *testing.T, ctx context.Context, srv server.Server) string {
	t.Helper()

	prefix, err := taskutil.SudoPrefix(ctx, srv)
	if err != nil {
		t.Fatalf("SudoPrefix failed: %v", err)
	}

	output, err := srv.Execute(ctx, prefix+"cat /etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("read sshd_config failed: %v", err)
	}

	setting := ""
	if err := taskutil.ScanLines(output, func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			return
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return
		}
		if strings.EqualFold(fields[0], "PermitRootLogin") {
			setting = strings.ToLower(fields[1])
		}
	}); err != nil {
		t.Fatalf("scan sshd_config failed: %v", err)
	}

	if setting == "" {
		t.Fatalf("PermitRootLogin not found in sshd_config")
	}
	return setting
}

func assertRootLoginWorks(t *testing.T, ctx context.Context, sshC *testutils.SSHContainer, name string) {
	t.Helper()

	srv := server.NewSSHServer(name, sshC.Address, "root", sshC.KeyPath, sshC.KnownHostsPath)
	tasktests.WaitForLogin(t, ctx, srv, "root")
}

func assertRootLoginBlocked(t *testing.T, ctx context.Context, sshC *testutils.SSHContainer, name string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		srv := server.NewSSHServer(name, sshC.Address, "root", sshC.KeyPath, sshC.KnownHostsPath)
		_, err := srv.Execute(ctx, "id -un")
		if err != nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected root login to be blocked, but it still succeeds")
		}
		time.Sleep(200 * time.Millisecond)
	}
}
