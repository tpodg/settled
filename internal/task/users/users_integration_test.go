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
)

func TestUsersTask_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	srv := server.NewSSHServer("users-integration", sshC.Address, sshC.User, sshC.KeyPath, sshC.KnownHostsPath)
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

		tasks := planUserTasks(t, overrides)
		if err := runner.Run(ctx, srv, tasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserExists(t, ctx, srv, "alice")
		assertUserInGroups(t, ctx, srv, "alice", []string{"developers", "sudo"})
		assertSudoersFile(t, ctx, srv, "alice", "alice ALL=(ALL) NOPASSWD:ALL")
		assertAuthorizedKeys(t, ctx, srv, "alice", []string{key1, key2})
		assertTasksSatisfied(t, ctx, srv, tasks)
	})

	t.Run("updates groups and keys without sudo", func(t *testing.T) {
		baseOverrides := map[string]any{
			"users": map[string]any{
				"bob": map[string]any{
					"groups": []string{"developers"},
				},
			},
		}

		tasks := planUserTasks(t, baseOverrides)
		if err := runner.Run(ctx, srv, tasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserExists(t, ctx, srv, "bob")
		assertUserInGroups(t, ctx, srv, "bob", []string{"developers"})
		assertNoSudoersFile(t, ctx, srv, "bob")
		assertNoAuthorizedKeys(t, ctx, srv, "bob")
		assertTasksSatisfied(t, ctx, srv, tasks)

		key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockKey3 bob@example"
		updatedOverrides := map[string]any{
			"users": map[string]any{
				"bob": map[string]any{
					"groups":          []string{"developers", "ops"},
					"authorized_keys": []string{key},
				},
			},
		}

		updatedTasks := planUserTasks(t, updatedOverrides)
		assertTasksNeedExecution(t, ctx, srv, updatedTasks)
		if err := runner.Run(ctx, srv, updatedTasks...); err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		assertUserInGroups(t, ctx, srv, "bob", []string{"developers", "ops"})
		assertAuthorizedKeys(t, ctx, srv, "bob", []string{key})
		assertNoSudoersFile(t, ctx, srv, "bob")
		assertTasksSatisfied(t, ctx, srv, updatedTasks)
	})
}

func planUserTasks(t *testing.T, overrides map[string]any) []task.Task {
	t.Helper()

	tasks, unknown, err := task.PlanTasks(overrides, []task.Spec{users.Spec()})
	if err != nil {
		t.Fatalf("PlanTasks failed: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown keys: %v", unknown)
	}
	if len(tasks) == 0 {
		t.Fatalf("expected at least one task, got %d", len(tasks))
	}
	return tasks
}

func runCommand(t *testing.T, ctx context.Context, srv server.Server, command string) string {
	t.Helper()

	output, err := srv.Execute(ctx, command)
	if err != nil {
		t.Fatalf("command %q failed: %v\nOutput: %s", command, err, output)
	}
	return output
}

func assertUserExists(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	output := runCommand(t, ctx, srv, fmt.Sprintf("id -u %s", name))
	if strings.TrimSpace(output) == "" {
		t.Fatalf("expected user %q to exist, got empty id output", name)
	}
}

func assertUserInGroups(t *testing.T, ctx context.Context, srv server.Server, name string, groups []string) {
	t.Helper()

	output := runCommand(t, ctx, srv, fmt.Sprintf("id -nG %s", name))
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
	output := runCommand(t, ctx, srv, fmt.Sprintf("cat %s", path))
	if strings.TrimSpace(output) != line {
		t.Fatalf("expected sudoers line %q, got %q", line, strings.TrimSpace(output))
	}
	perm := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s", path)))
	if perm != "440" {
		t.Fatalf("expected sudoers perms 440, got %q", perm)
	}
}

func assertNoSudoersFile(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	path := fmt.Sprintf("/etc/sudoers.d/settled-%s", taskutil.SanitizeFilename(name, "user"))
	runCommand(t, ctx, srv, fmt.Sprintf("test ! -f %s", path))
}

func assertAuthorizedKeys(t *testing.T, ctx context.Context, srv server.Server, name string, keys []string) {
	t.Helper()

	home := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("getent passwd %s | cut -d: -f6", name)))
	if home == "" {
		t.Fatalf("expected home directory for %q, got empty", name)
	}
	authFile := fmt.Sprintf("%s/.ssh/authorized_keys", home)
	output := runCommand(t, ctx, srv, fmt.Sprintf("cat %s", authFile))
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
	perm := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s", authFile)))
	if perm != "600" {
		t.Fatalf("expected authorized_keys perms 600, got %q", perm)
	}
	dirPerm := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("stat -c %%a %s/.ssh", home)))
	if dirPerm != "700" {
		t.Fatalf("expected .ssh perms 700, got %q", dirPerm)
	}
	owner := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("stat -c %%U:%%G %s/.ssh", home)))
	expectedOwner := fmt.Sprintf("%s:%s", name, name)
	if owner != expectedOwner {
		t.Fatalf("expected .ssh owner %q, got %q", expectedOwner, owner)
	}
}

func assertNoAuthorizedKeys(t *testing.T, ctx context.Context, srv server.Server, name string) {
	t.Helper()

	home := strings.TrimSpace(runCommand(t, ctx, srv, fmt.Sprintf("getent passwd %s | cut -d: -f6", name)))
	if home == "" {
		t.Fatalf("expected home directory for %q, got empty", name)
	}
	authFile := fmt.Sprintf("%s/.ssh/authorized_keys", home)
	runCommand(t, ctx, srv, fmt.Sprintf("test ! -f %s", authFile))
}

func assertTasksSatisfied(t *testing.T, ctx context.Context, srv server.Server, tasks []task.Task) {
	t.Helper()

	for _, task := range tasks {
		needs, err := task.NeedsExecution(ctx, srv)
		if err != nil {
			t.Fatalf("NeedsExecution failed for %q: %v", task.Name(), err)
		}
		if needs {
			t.Fatalf("expected task %q to be satisfied", task.Name())
		}
	}
}

func assertTasksNeedExecution(t *testing.T, ctx context.Context, srv server.Server, tasks []task.Task) {
	t.Helper()

	for _, task := range tasks {
		needs, err := task.NeedsExecution(ctx, srv)
		if err != nil {
			t.Fatalf("NeedsExecution failed for %q: %v", task.Name(), err)
		}
		if needs {
			return
		}
	}
	t.Fatal("expected at least one task to need execution")
}
