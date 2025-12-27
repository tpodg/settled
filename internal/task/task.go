package task

import (
	"context"
	"fmt"
	"sort"

	"github.com/tpodg/settled/internal/server"
)

// Task represents a single unit of work that can be applied to a server.
type Task interface {
	// Name returns a human-readable name for the task.
	Name() string
	// NeedsExecution checks if the task needs to be executed on the server.
	// It should return true if the task should be performed, false if the state is already as desired.
	NeedsExecution(ctx context.Context, s server.Server) (bool, error)
	// Execute performs the task on the server.
	Execute(ctx context.Context, s server.Server) error
}

// Handler is a function that creates one or more tasks from a piece of state.
type Handler func(state any) ([]Task, error)

// Builder ties a state key to a handler.
type Builder struct {
	Key     string
	Handler Handler
}

// CreateTasks creates a list of tasks from the provided config map.
// It returns the list of unknown task keys so callers can decide how to handle them.
func CreateTasks(config map[string]any, builders ...Builder) ([]Task, []string, error) {
	var tasks []Task
	var unknown []string

	handlers := make(map[string]Handler, len(builders))
	order := make([]string, 0, len(builders))
	for _, b := range builders {
		if _, exists := handlers[b.Key]; exists {
			return nil, nil, fmt.Errorf("duplicate task builder key: %s", b.Key)
		}
		handlers[b.Key] = b.Handler
		order = append(order, b.Key)
	}

	for _, key := range order {
		val, ok := config[key]
		if !ok {
			continue
		}
		handler := handlers[key]
		t, err := handler(val)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create tasks for %s: %w", key, err)
		}
		tasks = append(tasks, t...)
	}

	for key := range config {
		if _, ok := handlers[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	return tasks, unknown, nil
}
