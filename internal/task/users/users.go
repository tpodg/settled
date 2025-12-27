package users

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/taskutil"
)

type UserConfig struct {
	Sudo           bool     `yaml:"sudo"`
	SudoNoPassword bool     `yaml:"sudo_nopasswd"`
	Groups         []string `yaml:"groups"`
	AuthorizedKeys []string `yaml:"authorized_keys"`
}

type Config map[string]UserConfig

// Spec defines the user management task spec.
func Spec() task.Spec {
	return task.SpecFor("users", "users.yaml", buildUsersTasks)
}

func buildUsersTasks(cfg Config) ([]task.Task, error) {
	if len(cfg) == 0 {
		return nil, nil
	}

	users := make([]string, 0, len(cfg))
	for name := range cfg {
		users = append(users, name)
	}
	sort.Strings(users)

	tasks := make([]task.Task, 0, len(cfg))
	for _, name := range users {
		if err := validateUserName(name); err != nil {
			return nil, err
		}
		userCfg := cfg[name]
		userCfg.Groups = strutil.CleanList(userCfg.Groups)
		userCfg.AuthorizedKeys = strutil.CleanList(userCfg.AuthorizedKeys)
		if err := validateGroupNames(userCfg.Groups); err != nil {
			return nil, err
		}
		tasks = append(tasks, &UserTask{
			name:   name,
			config: userCfg,
		})
	}
	return tasks, nil
}

type UserTask struct {
	name   string
	config UserConfig
}

func (t *UserTask) Name() string {
	return fmt.Sprintf("user: %s", t.name)
}

func (t *UserTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
	entry, err := lookupUser(ctx, s, t.name)
	if err != nil {
		return false, err
	}
	if entry == nil {
		return true, nil
	}

	if needs, err := t.needsGroupUpdate(ctx, s); err != nil || needs {
		return needs, err
	}

	prefix := ""
	if t.config.Sudo || len(t.config.AuthorizedKeys) > 0 {
		var err error
		prefix, err = taskutil.SudoPrefix(ctx, s)
		if err != nil {
			return false, err
		}
	}

	if needs, err := t.needsSudoersUpdate(ctx, s, prefix); err != nil || needs {
		return needs, err
	}

	if needs, err := t.needsAuthorizedKeysUpdate(ctx, s, prefix, entry.home); err != nil || needs {
		return needs, err
	}

	return false, nil
}

func (t *UserTask) Execute(ctx context.Context, s server.Server) error {
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

func (t *UserTask) needsGroupUpdate(ctx context.Context, s server.Server) (bool, error) {
	if len(t.config.Groups) == 0 {
		return false, nil
	}

	groups, err := lookupGroups(ctx, s, t.name)
	if err != nil {
		return false, err
	}
	for _, group := range t.config.Groups {
		if !groups[group] {
			return true, nil
		}
	}
	return false, nil
}

func (t *UserTask) needsSudoersUpdate(ctx context.Context, s server.Server, prefix string) (bool, error) {
	if !t.config.Sudo {
		return false, nil
	}

	ok, err := t.sudoersMatches(ctx, s, prefix)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

func (t *UserTask) needsAuthorizedKeysUpdate(ctx context.Context, s server.Server, prefix, home string) (bool, error) {
	if len(t.config.AuthorizedKeys) == 0 {
		return false, nil
	}

	ok, err := t.authorizedKeysMatch(ctx, s, prefix, home)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

type userScriptData struct {
	Name           string
	Groups         []string
	Sudo           bool
	SudoersFile    string
	SudoersLine    string
	AuthorizedKeys []string
}

func (t *UserTask) scriptData() userScriptData {
	return userScriptData{
		Name:           t.name,
		Groups:         t.config.Groups,
		Sudo:           t.config.Sudo,
		SudoersFile:    t.sudoersFile(),
		SudoersLine:    t.sudoersLine(),
		AuthorizedKeys: t.config.AuthorizedKeys,
	}
}

func (t *UserTask) renderScript() (string, error) {
	var buf strings.Builder
	if err := userScriptTemplates.ExecuteTemplate(&buf, "main", t.scriptData()); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

type userEntry struct {
	home string
}

func lookupUser(ctx context.Context, s server.Server, name string) (*userEntry, error) {
	output, err := s.Execute(ctx, fmt.Sprintf("getent passwd %s", strutil.ShellEscape(name)))
	if err != nil {
		if strings.TrimSpace(output) == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup user %q: %w", name, err)
	}
	line := strings.TrimSpace(output)
	if line == "" {
		return nil, nil
	}
	fields := strings.Split(line, ":")
	if len(fields) < 6 {
		return nil, fmt.Errorf("unexpected passwd entry for %q: %s", name, line)
	}
	return &userEntry{
		home: fields[5],
	}, nil
}

func lookupGroups(ctx context.Context, s server.Server, name string) (map[string]bool, error) {
	output, err := s.Execute(ctx, fmt.Sprintf("id -nG %s", strutil.ShellEscape(name)))
	if err != nil {
		return nil, fmt.Errorf("lookup groups for %q: %w", name, err)
	}
	groups := make(map[string]bool)
	for _, group := range strings.Fields(output) {
		groups[group] = true
	}
	return groups, nil
}

func (t *UserTask) sudoersFile() string {
	return fmt.Sprintf("/etc/sudoers.d/settled-%s", taskutil.SanitizeFilename(t.name, "user"))
}

func (t *UserTask) sudoersLine() string {
	if t.config.SudoNoPassword {
		return fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", t.name)
	}
	return fmt.Sprintf("%s ALL=(ALL) ALL", t.name)
}

func (t *UserTask) sudoersMatches(ctx context.Context, s server.Server, prefix string) (bool, error) {
	output, missing, err := taskutil.ReadFileIfExists(ctx, s, prefix, t.sudoersFile())
	if err != nil {
		return false, fmt.Errorf("read sudoers for %q: %w", t.name, err)
	}
	if missing {
		return false, nil
	}

	ok, err := taskutil.HasExactLine(output, t.sudoersLine())
	if err != nil {
		return false, fmt.Errorf("scan sudoers for %q: %w", t.name, err)
	}
	return ok, nil
}

func (t *UserTask) authorizedKeysMatch(ctx context.Context, s server.Server, prefix, home string) (bool, error) {
	if strings.TrimSpace(home) == "" {
		return false, fmt.Errorf("empty home directory for %q", t.name)
	}

	authFile := fmt.Sprintf("%s/.ssh/authorized_keys", home)
	output, missing, err := taskutil.ReadFileIfExists(ctx, s, prefix, authFile)
	if err != nil {
		return false, fmt.Errorf("read authorized_keys for %q: %w", t.name, err)
	}
	if missing {
		return false, nil
	}

	keys, err := taskutil.LineSet(output)
	if err != nil {
		return false, fmt.Errorf("scan authorized_keys for %q: %w", t.name, err)
	}
	for _, key := range t.config.AuthorizedKeys {
		if _, ok := keys[key]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func validateUserName(name string) error {
	return taskutil.ValidateIdentifier("user", name)
}

func validateGroupNames(groups []string) error {
	for _, group := range groups {
		if err := taskutil.ValidateIdentifier("group", group); err != nil {
			return err
		}
	}
	return nil
}
