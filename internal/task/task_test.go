package task_test

import (
	"context"
	"errors"
	"testing"

	"log/slog"
	"os"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task"
)

type mockServer struct {
	server.Server
}

func (m *mockServer) ID() string { return "mock-server" }

type mockTask struct {
	name           string
	needsExecution bool
	executed       bool
	err            error
}

func (m *mockTask) Name() string { return m.name }
func (m *mockTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
	return m.needsExecution, nil
}
func (m *mockTask) Execute(ctx context.Context, s server.Server) error {
	m.executed = true
	return m.err
}

func TestRunner_Run(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	runner := task.NewRunner(logger)
	s := &mockServer{}

	t.Run("Task needs execution", func(t *testing.T) {
		mt := &mockTask{name: "test-task", needsExecution: true}
		err := runner.Run(context.Background(), s, mt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !mt.executed {
			t.Error("task should have been executed")
		}
	})

	t.Run("Task does not need execution", func(t *testing.T) {
		mt := &mockTask{name: "test-task", needsExecution: false}
		err := runner.Run(context.Background(), s, mt)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if mt.executed {
			t.Error("task should not have been executed")
		}
	})

	t.Run("Task fails", func(t *testing.T) {
		expectedErr := errors.New("execution failed")
		mt := &mockTask{name: "test-task", needsExecution: true, err: expectedErr}
		err := runner.Run(context.Background(), s, mt)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}

type mockConfig struct {
	Name string `yaml:"name"`
}

func TestCreateTasks(t *testing.T) {
	builder := task.BuilderFor("mock", func(cfg mockConfig) ([]task.Task, error) {
		return []task.Task{&mockTask{name: cfg.Name}}, nil
	})

	t.Run("Create tasks from state", func(t *testing.T) {
		state := map[string]any{
			"mock": map[string]any{"name": "test-task"},
		}
		tasks, unknown, err := task.CreateTasks(state, builder)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(unknown) != 0 {
			t.Fatalf("unexpected unknown keys: %v", unknown)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Name() != "test-task" {
			t.Errorf("expected task name 'test-task', got %q", tasks[0].Name())
		}
	})

	t.Run("Create tasks from unknown state", func(t *testing.T) {
		state := map[string]any{"unknown": nil}
		_, unknown, err := task.CreateTasks(state, builder)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(unknown) != 1 || unknown[0] != "unknown" {
			t.Errorf("expected unknown key 'unknown', got %v", unknown)
		}
	})
}
