package task

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task"
)

const (
	loginWaitTimeout  = 5 * time.Second
	loginWaitInterval = 200 * time.Millisecond
)

func PlanTasks(t *testing.T, overrides map[string]any, spec task.Spec) []task.Task {
	t.Helper()

	tasks, unknown, err := task.PlanTasks(overrides, []task.Spec{spec})
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

func RunCommand(t *testing.T, ctx context.Context, srv server.Server, command string) string {
	t.Helper()

	output, err := srv.Execute(ctx, command)
	if err != nil {
		t.Fatalf("command %q failed: %v\nOutput: %s", command, err, output)
	}
	return output
}

func AssertTasksSatisfied(t *testing.T, ctx context.Context, srv server.Server, tasks []task.Task) {
	t.Helper()

	for _, currentTask := range tasks {
		needs, err := currentTask.NeedsExecution(ctx, srv)
		if err != nil {
			t.Fatalf("NeedsExecution failed for %q: %v", currentTask.Name(), err)
		}
		if needs {
			t.Fatalf("expected task %q to be satisfied", currentTask.Name())
		}
	}
}

func AssertTasksNeedExecution(t *testing.T, ctx context.Context, srv server.Server, tasks []task.Task) {
	t.Helper()

	for _, currentTask := range tasks {
		needs, err := currentTask.NeedsExecution(ctx, srv)
		if err != nil {
			t.Fatalf("NeedsExecution failed for %q: %v", currentTask.Name(), err)
		}
		if needs {
			return
		}
	}
	t.Fatal("expected at least one task to need execution")
}

func WaitForLogin(t *testing.T, ctx context.Context, srv server.Server, expectedUser string) {
	t.Helper()

	deadline := time.Now().Add(loginWaitTimeout)
	for {
		output, err := srv.Execute(ctx, "id -un")
		if err == nil {
			user := strings.TrimSpace(output)
			if user != expectedUser {
				t.Fatalf("expected login user %q, got %q", expectedUser, user)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected login for %q to work, last error: %v", expectedUser, err)
		}
		time.Sleep(loginWaitInterval)
	}
}

func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return buf.String()
}
