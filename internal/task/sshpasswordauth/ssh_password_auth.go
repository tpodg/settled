package sshpasswordauth

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
	return task.SpecFor("ssh_password_auth", "ssh_password_auth.yaml", buildTasks)
}

func buildTasks(cfg Config) ([]task.Task, error) {
	if !cfg.Disable {
		return nil, nil
	}
	return []task.Task{&DisableSSHPasswordAuthTask{}}, nil
}

type DisableSSHPasswordAuthTask struct{}

func (t *DisableSSHPasswordAuthTask) Name() string {
	return "disable ssh password authentication"
}

func (t *DisableSSHPasswordAuthTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
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

	disabled, err := sshPasswordAuthDisabled(output)
	if err != nil {
		return false, err
	}
	return !disabled, nil
}

func (t *DisableSSHPasswordAuthTask) Execute(ctx context.Context, s server.Server) error {
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

type sshPasswordAuthScriptData struct {
	ConfigPath string
}

func (t *DisableSSHPasswordAuthTask) scriptData() sshPasswordAuthScriptData {
	return sshPasswordAuthScriptData{
		ConfigPath: sshdConfigPath,
	}
}

func (t *DisableSSHPasswordAuthTask) renderScript() (string, error) {
	var buf strings.Builder
	if err := sshPasswordAuthScriptTemplates.ExecuteTemplate(&buf, "main", t.scriptData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func sshPasswordAuthDisabled(output string) (bool, error) {
	settings, err := taskutil.ParseKeyValueSettings(output)
	if err != nil {
		return false, err
	}

	passwordSetting := settings["passwordauthentication"]
	kbdSetting := settings["kbdinteractiveauthentication"]
	challengeSetting := settings["challengeresponseauthentication"]

	if passwordSetting != "no" {
		return false, nil
	}
	if kbdSetting == "" && challengeSetting == "" {
		return false, nil
	}
	if kbdSetting != "" && kbdSetting != "no" {
		return false, nil
	}
	if challengeSetting != "" && challengeSetting != "no" {
		return false, nil
	}
	return true, nil
}
