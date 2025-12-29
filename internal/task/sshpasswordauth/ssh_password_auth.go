package sshpasswordauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/sshd"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/taskutil"
)

var (
	passwordAuthKeyLower          = strings.ToLower(sshd.KeyPasswordAuthentication)
	kbdInteractiveAuthKeyLower    = strings.ToLower(sshd.KeyKbdInteractiveAuth)
	challengeResponseAuthKeyLower = strings.ToLower(sshd.KeyChallengeResponseAuth)
)

type Config struct {
	Disable bool `yaml:"disable"`
}

const TaskKey = "ssh_password_auth"

func Spec() task.Spec {
	return task.SpecFor(TaskKey, "ssh_password_auth.yaml", buildTasks)
}

func buildTasks(cfg Config) ([]task.Task, error) {
	if !cfg.Disable {
		return nil, nil
	}
	return []task.Task{&DisableSSHPasswordAuthTask{}}, nil
}

type DisableSSHPasswordAuthTask struct {
	configPath string
}

func (t *DisableSSHPasswordAuthTask) Name() string {
	return "disable ssh password authentication"
}

func (t *DisableSSHPasswordAuthTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
	path, output, err := sshd.ReadConfig(ctx, s)
	if err != nil {
		return false, err
	}
	t.configPath = path

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

	if _, err := t.resolveConfigPath(ctx, s); err != nil {
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
	Settings   []sshSetting
}

type sshSetting struct {
	Key   string
	Value string
}

func (t *DisableSSHPasswordAuthTask) scriptData() sshPasswordAuthScriptData {
	configPath := t.configPath
	if configPath == "" {
		configPath = sshd.DefaultConfigPath
	}
	return sshPasswordAuthScriptData{
		ConfigPath: configPath,
		Settings: []sshSetting{
			{Key: sshd.KeyPasswordAuthentication, Value: sshd.ValueNo},
			{Key: sshd.KeyKbdInteractiveAuth, Value: sshd.ValueNo},
			{Key: sshd.KeyChallengeResponseAuth, Value: sshd.ValueNo},
		},
	}
}

func (t *DisableSSHPasswordAuthTask) renderScript() (string, error) {
	var buf strings.Builder
	if err := sshPasswordAuthScriptTemplates.ExecuteTemplate(&buf, "main", t.scriptData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func (t *DisableSSHPasswordAuthTask) resolveConfigPath(ctx context.Context, s server.Server) (string, error) {
	if t.configPath != "" {
		return t.configPath, nil
	}
	path, _, err := sshd.ReadConfig(ctx, s)
	if err != nil {
		return "", err
	}
	t.configPath = path
	return path, nil
}

func sshPasswordAuthDisabled(output string) (bool, error) {
	settings, err := taskutil.ParseKeyValueSettings(output)
	if err != nil {
		return false, err
	}

	passwordSetting := settings[passwordAuthKeyLower]
	kbdSetting := settings[kbdInteractiveAuthKeyLower]
	challengeSetting := settings[challengeResponseAuthKeyLower]

	if passwordSetting != sshd.ValueNo {
		return false, nil
	}
	if kbdSetting == "" && challengeSetting == "" {
		return false, nil
	}
	if kbdSetting != "" && kbdSetting != sshd.ValueNo {
		return false, nil
	}
	if challengeSetting != "" && challengeSetting != sshd.ValueNo {
		return false, nil
	}
	return true, nil
}
