package task

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/tpodg/settled/internal/server"
)

// Runner is responsible for executing tasks on a server.
type Runner struct {
	logger *slog.Logger
}

// NewRunner creates a new Runner with the given logger.
func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{
		logger: logger,
	}
}

// Run executes a list of tasks on a server.
// For each task, it first checks if it needs execution.
func (r *Runner) Run(ctx context.Context, s server.Server, tasks ...Task) error {
	for _, t := range tasks {
		name := t.Name()
		r.logger.Info("Processing task", "task", name, "server", s.ID())

		needsExec, err := t.NeedsExecution(ctx, s)
		if err != nil {
			return fmt.Errorf("failed to check if task %q needs execution: %w", name, err)
		}

		if !needsExec {
			r.logger.Info("Task is already satisfied", "task", name, "server", s.ID())
			continue
		}

		r.logger.Info("Applying task", "task", name, "server", s.ID())
		if err := t.Execute(ctx, s); err != nil {
			return fmt.Errorf("failed to execute task %q: %w", name, err)
		}

		r.logger.Info("Task applied successfully", "task", name, "server", s.ID())
	}

	return nil
}
