package task

import (
	"context"

	"github.com/tpodg/settled/internal/server"
)

// TaskConfigurator implements the server.Configurator interface using a list of tasks.
type TaskConfigurator struct {
	tasks  []Task
	runner *Runner
}

// NewTaskConfigurator creates a new TaskConfigurator with the given runner and tasks.
func NewTaskConfigurator(runner *Runner, tasks ...Task) *TaskConfigurator {
	return &TaskConfigurator{
		tasks:  tasks,
		runner: runner,
	}
}

// Configure applies the tasks to the server using the runner.
func (tc *TaskConfigurator) Configure(ctx context.Context, s server.Server) error {
	return tc.runner.Run(ctx, s, tc.tasks...)
}
