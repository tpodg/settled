package users_test

import (
	"testing"

	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/users"
)

func TestUsersSpecBuild(t *testing.T) {
	overrides := map[string]any{
		"users": map[string]any{
			"alice": map[string]any{
				"sudo":          true,
				"sudo_nopasswd": true,
				"groups":        []string{"sudo"},
				"authorized_keys": []string{
					"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... alice@example",
				},
			},
		},
	}

	tasks, unknown, err := task.PlanTasks(overrides, []task.Spec{users.Spec()})
	if err != nil {
		t.Fatalf("PlanTasks failed: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown keys: %v", unknown)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Name() != "user: alice" {
		t.Fatalf("expected task name 'user: alice', got %q", tasks[0].Name())
	}
}

func TestUsersSpecBuildEmptyUser(t *testing.T) {
	overrides := map[string]any{
		"users": map[string]any{
			" ": map[string]any{
				"sudo": true,
			},
		},
	}

	_, _, err := task.PlanTasks(overrides, []task.Spec{users.Spec()})
	if err == nil {
		t.Fatal("expected error for empty user name, got nil")
	}
}
