package rootlogin

import (
	"context"
	"fmt"
	"strings"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/taskutil"
)

const (
	sshdConfigPath = "/etc/ssh/sshd_config"
)

type Config struct {
	Disable bool `yaml:"disable"`
}

func Spec() task.Spec {
	return task.SpecFor("root_login", "root_login.yaml", buildTasks)
}

func buildTasks(cfg Config) ([]task.Task, error) {
	if !cfg.Disable {
		return nil, nil
	}
	return []task.Task{&DisableRootLoginTask{}}, nil
}

type DisableRootLoginTask struct{}

func (t *DisableRootLoginTask) Name() string {
	return "disable root login"
}

func (t *DisableRootLoginTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
	isRoot, err := isLoggedInAsRoot(ctx, s)
	if err != nil {
		return false, err
	}
	if isRoot {
		taskutil.Warnf("skipping %s task because connected as root.", t.Name())
		return false, nil
	}

	prefix, err := taskutil.SudoPrefix(ctx, s)
	if err != nil {
		return false, err
	}

	output, missing, err := taskutil.ReadFileIfExists(ctx, s, prefix, sshdConfigPath)
	if err != nil {
		return false, err
	}
	if missing {
		return false, fmt.Errorf("sshd config %s not found", sshdConfigPath)
	}

	disabled, err := rootLoginDisabled(output)
	if err != nil {
		return false, err
	}
	return !disabled, nil
}

func (t *DisableRootLoginTask) Execute(ctx context.Context, s server.Server) error {
	prefix, err := taskutil.SudoPrefix(ctx, s)
	if err != nil {
		return err
	}

	script, err := t.renderScript()
	if err != nil {
		return err
	}

	cmd := prefix + "sh -c " + strutil.ShellEscape(script)
	if _, err := s.Execute(ctx, cmd); err != nil {
		return err
	}
	return nil
}

type rootLoginScriptData struct {
	ConfigPath string
}

func (t *DisableRootLoginTask) scriptData() rootLoginScriptData {
	return rootLoginScriptData{
		ConfigPath: sshdConfigPath,
	}
}

func (t *DisableRootLoginTask) renderScript() (string, error) {
	var buf strings.Builder
	if err := rootLoginScriptTemplates.ExecuteTemplate(&buf, "main", t.scriptData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func isLoggedInAsRoot(ctx context.Context, s server.Server) (bool, error) {
	output, err := s.Execute(ctx, "id -un")
	if err != nil {
		return false, fmt.Errorf("check login user: %w", err)
	}
	return strings.TrimSpace(output) == "root", nil
}

func rootLoginDisabled(output string) (bool, error) {
	settings, err := taskutil.ParseKeyValueSettings(output)
	if err != nil {
		return false, err
	}
	return settings["permitrootlogin"] == "no", nil
}
