package users_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/taskutil"
	"github.com/tpodg/settled/internal/task/users"
	"github.com/tpodg/settled/internal/testutils"
	tasktests "github.com/tpodg/settled/internal/testutils/task"
)

func TestUsersTask_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	srv := server.NewSSHServer("users-integration", sshC.Address, sshC.User, sshC.KeyPath, sshC.KnownHostsPath, server.SSHOptions{})
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runner := task.NewRunner(logger)

	t.Run("creates user with sudo and keys", func(t *testing.T) {
		key1 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockKey1 alice@example"
		key2 := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockKey2 alice@example"
		overrides := map[string]any{
			"users": map[string]any{
				"alice": map[string]any{
					"sudo":            true,
					"sudo_nopasswd":   true,
					"groups":          []string{"developers", "sudo"},
					"authorized_keys": []string{key1, key2},
				},
			},
		}

		tasks := tasktests.PlanTasks(t, overrides, users.Spec())
		if err := runner.Run(ctx, srv, tasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserExists(t, ctx, srv, "alice")
		assertUserInGroups(t, ctx, srv, "alice", []string{"developers", "sudo"})
		assertSudoersFile(t, ctx, srv, "alice", "alice ALL=(ALL) NOPASSWD:ALL")
		assertAuthorizedKeys(t, ctx, srv, "alice", []string{key1, key2})
		tasktests.AssertTasksSatisfied(t, ctx, srv, tasks)
	})

	t.Run("updates groups and keys without sudo", func(t *testing.T) {
		baseOverrides := map[string]any{
			"users": map[string]any{
				"bob": map[string]any{
					"groups": []string{"developers"},
				},
			},
		}

		tasks := tasktests.PlanTasks(t, baseOverrides, users.Spec())
		if err := runner.Run(ctx, srv, tasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserExists(t, ctx, srv, "bob")
		assertUserInGroups(t, ctx, srv, "bob", []string{"developers"})
		assertNoSudoersFile(t, ctx, srv, "bob")
		assertNoAuthorizedKeys(t, ctx, srv, "bob")
		tasktests.AssertTasksSatisfied(t, ctx, srv, tasks)

		key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockKey3 bob@example"
		updatedOverrides := map[string]any{
			"users": map[string]any{
				"bob": map[string]any{
					"groups":          []string{"developers", "ops"},
					"authorized_keys": []string{key},
				},
			},
		}

		updatedTasks := tasktests.PlanTasks(t, updatedOverrides, users.Spec())
		tasktests.AssertTasksNeedExecution(t, ctx, srv, updatedTasks)
		if err := runner.Run(ctx, srv, updatedTasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserInGroups(t, ctx, srv, "bob", []string{"developers", "ops"})
		assertAuthorizedKeys(t, ctx, srv, "bob", []string{key})
		assertNoSudoersFile(t, ctx, srv, "bob")
		tasktests.AssertTasksSatisfied(t, ctx, srv, updatedTasks)
	})
}

func assertUserExists(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	output := tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("id -u %s", name))
	if strings.TrimSpace(output) == "" {
		t.Fatalf("expected user %q to exist, got empty id output", name)
	}
}

func assertUserInGroups(t *testing.T, ctx context.Context, srv server.Server, name string, groups []string) {
	t.Helper()

	output := tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("id -nG %s", name))
	seen := make(map[string]bool)
	for _, group := range strings.Fields(output) {
		seen[group] = true
	}
	for _, group := range groups {
		if !seen[group] {
			t.Fatalf("expected user %q to be in group %q, got %q", name, group, strings.TrimSpace(output))
		}
	}
}

func assertSudoersFile(t *testing.T, ctx context.Context, srv server.Server, name, line string) {
	t.Helper()

	path := fmt.Sprintf("/etc/sudoers.d/settled-%s", taskutil.SanitizeFilename(name, "user"))
	output := tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("cat %s", path))
	if strings.TrimSpace(output) != line {
		t.Fatalf("expected sudoers line %q, got %q", line, strings.TrimSpace(output))
	}
	perm := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s", path)))
	if perm != "440" {
		t.Fatalf("expected sudoers perms 440, got %q", perm)
	}
}

func assertNoSudoersFile(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	path := fmt.Sprintf("/etc/sudoers.d/settled-%s", taskutil.SanitizeFilename(name, "user"))
	tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("test ! -f %s", path))
}

func assertAuthorizedKeys(t *testing.T, ctx context.Context, srv server.Server, name string, keys []string) {
	t.Helper()

	home := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("getent passwd %s | cut -d: -f6", name)))
	if home == "" {
		t.Fatalf("expected home directory for %q, got empty", name)
	}
	authFile := fmt.Sprintf("%s/.ssh/authorized_keys", home)
	output := tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("cat %s", authFile))
	lines, err := taskutil.LineSet(output)
	if err != nil {
		t.Fatalf("scan authorized_keys: %v", err)
	}
	for _, key := range keys {
		if _, ok := lines[key]; !ok {
			t.Fatalf("expected authorized_keys to contain %q", key)
		}
	}
	if len(lines) != len(keys) {
		t.Fatalf("expected %d authorized_keys entries, got %d", len(keys), len(lines))
	}
	perm := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s", authFile)))
	if perm != "600" {
		t.Fatalf("expected authorized_keys perms 600, got %q", perm)
	}
	dirPerm := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s/.ssh", home)))
	if dirPerm != "700" {
		t.Fatalf("expected .ssh perms 700, got %q", dirPerm)
	}
	owner := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("stat -c %%U:%%G %s/.ssh", home)))
	expectedOwner := fmt.Sprintf("%s:%s", name, name)
	if owner != expectedOwner {
		t.Fatalf("expected .ssh owner %q, got %q", expectedOwner, owner)
	}
}

func assertNoAuthorizedKeys(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	home := strings.TrimSpace(tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("getent passwd %s | cut -d: -f6", name)))
	if home == "" {
		t.Fatalf("expected home directory for %q, got empty", name)
	}
	authFile := fmt.Sprintf("%s/.ssh/authorized_keys", home)
	tasktests.RunCommand(t, ctx, srv, fmt.Sprintf("test ! -f %s", authFile))
}
