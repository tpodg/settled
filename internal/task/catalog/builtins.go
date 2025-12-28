package catalog

import (
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/rootlogin"
	"github.com/tpodg/settled/internal/task/users"
)

// Builtins returns the built-in task specifications.
func Builtins() []task.Spec {
	return []task.Spec{
		users.Spec(),
		rootlogin.Spec(),
	}
}
