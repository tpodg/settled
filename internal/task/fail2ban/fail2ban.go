package fail2ban

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/taskutil"
)

const (
	TaskKey            = "fail2ban"
	defaultJailConfig  = "/etc/fail2ban/jail.d/settled.conf"
	continuationIndent = "          "
)

const (
	fail2banServiceName = "fail2ban"
	fail2banClientCmd   = "fail2ban-client"
	scriptOutputYes     = "yes"
	scriptOutputNo      = "no"
)

const (
	jailKeyEnabled  = "enabled"
	jailKeyFilter   = "filter"
	jailKeyPort     = "port"
	jailKeyProtocol = "protocol"
	jailKeyLogPath  = "logpath"
	jailKeyBackend  = "backend"
	jailKeyMaxRetry = "maxretry"
	jailKeyFindTime = "findtime"
	jailKeyBanTime  = "bantime"
	jailKeyAction   = "action"
	jailKeyIgnoreIP = "ignoreip"
)

const (
	ruleFieldFilter   = jailKeyFilter
	ruleFieldPort     = jailKeyPort
	ruleFieldProtocol = jailKeyProtocol
	ruleFieldLogPath  = jailKeyLogPath
	ruleFieldBackend  = jailKeyBackend
	ruleFieldAction   = jailKeyAction
	ruleFieldMaxRetry = "max_retry"
	ruleFieldFindTime = "find_time"
	ruleFieldBanTime  = "ban_time"
	ruleFieldIgnoreIP = "ignore_ip"
)

type Config struct {
	Rules map[string]Rule `yaml:"rules"`
}

type Rule struct {
	Enabled  *bool          `yaml:"enabled"`
	Filter   string         `yaml:"filter"`
	Port     string         `yaml:"port"`
	Protocol string         `yaml:"protocol"`
	LogPath  StringList     `yaml:"logpath"`
	Backend  string         `yaml:"backend"`
	MaxRetry *int           `yaml:"max_retry"`
	FindTime *time.Duration `yaml:"find_time"`
	BanTime  *time.Duration `yaml:"ban_time"`
	Action   StringList     `yaml:"action"`
	IgnoreIP StringList     `yaml:"ignore_ip"`
	Options  map[string]any `yaml:"options"`
}

type StringList []string

func (s *StringList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var list []string
	if err := unmarshal(&list); err == nil {
		*s = list
		return nil
	}

	var single string
	if err := unmarshal(&single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			*s = nil
			return nil
		}
		*s = []string{single}
		return nil
	}

	return fmt.Errorf("expected string or list")
}

func Spec() task.Spec {
	return task.SpecFor(TaskKey, "fail2ban.yaml", buildTasks)
}

func buildTasks(cfg Config) ([]task.Task, error) {
	rules, err := normalizeRules(cfg.Rules)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}

	content, err := renderJailConfig(rules)
	if err != nil {
		return nil, err
	}

	return []task.Task{&Fail2banTask{
		configPath:    defaultJailConfig,
		configContent: content,
	}}, nil
}

type Fail2banTask struct {
	configPath    string
	configContent string
}

func (t *Fail2banTask) Name() string {
	return "configure fail2ban"
}

func (t *Fail2banTask) NeedsExecution(ctx context.Context, s server.Server) (bool, error) {
	installed, err := fail2banInstalled(ctx, s)
	if err != nil {
		return false, err
	}
	if !installed {
		return true, nil
	}

	prefix, err := taskutil.SudoPrefix(ctx, s)
	if err != nil {
		return false, err
	}

	output, missing, err := taskutil.ReadFileIfExists(ctx, s, prefix, t.configPath)
	if err != nil {
		return false, err
	}
	if missing {
		return true, nil
	}
	if !configMatches(output, t.configContent) {
		return true, nil
	}

	ready, err := fail2banServiceReady(ctx, s, prefix)
	if err != nil {
		return false, err
	}
	return !ready, nil
}

func (t *Fail2banTask) Execute(ctx context.Context, s server.Server) error {
	prefix, err := taskutil.SudoPrefix(ctx, s)
	if err != nil {
		return err
	}

	script, err := t.renderScript()
	if err != nil {
		return err
	}

	if _, err := runScript(ctx, s, prefix, script); err != nil {
		return err
	}
	return nil
}

type fail2banScriptData struct {
	ConfigPath    string
	ConfigContent string
	ServiceName   string
	ClientCmd     string
	ResultYes     string
	ResultNo      string
}

func (t *Fail2banTask) renderScript() (string, error) {
	data := newFail2banScriptData()
	data.ConfigPath = t.configPath
	data.ConfigContent = t.configContent
	return renderFail2banScript("main", data)
}

func newFail2banScriptData() fail2banScriptData {
	return fail2banScriptData{
		ServiceName: fail2banServiceName,
		ClientCmd:   fail2banClientCmd,
		ResultYes:   scriptOutputYes,
		ResultNo:    scriptOutputNo,
	}
}

func renderFail2banScript(templateName string, data fail2banScriptData) (string, error) {
	var buf strings.Builder
	if err := fail2banScriptTemplates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func runScript(ctx context.Context, s server.Server, prefix, script string) (string, error) {
	cmd := prefix + "sh -c " + strutil.ShellEscape(script)
	return s.Execute(ctx, cmd)
}

type jailConfigWriter struct {
	buf      *strings.Builder
	ruleName string
}

func (w jailConfigWriter) writeKeyValue(key, value string) {
	fmt.Fprintf(w.buf, "%s = %s\n", key, value)
}

func (w jailConfigWriter) writeString(key, value string) {
	if value == "" {
		return
	}
	w.writeKeyValue(key, value)
}

func (w jailConfigWriter) writeList(key string, values []string) {
	if len(values) == 0 {
		return
	}
	writeListSetting(w.buf, key, values)
}

func (w jailConfigWriter) writeInt(key string, value *int) {
	if value == nil {
		return
	}
	fmt.Fprintf(w.buf, "%s = %d\n", key, *value)
}

func (w jailConfigWriter) writeDuration(key, field string, value *time.Duration) error {
	if value == nil {
		return nil
	}
	seconds, err := durationSeconds(*value)
	if err != nil {
		return ruleErrorf(w.ruleName, "%s: %w", field, err)
	}
	fmt.Fprintf(w.buf, "%s = %d\n", key, seconds)
	return nil
}

func (w jailConfigWriter) writeJoin(key string, values []string) {
	if len(values) == 0 {
		return
	}
	w.writeKeyValue(key, strings.Join(values, " "))
}

type jailRule struct {
	Name     string
	Enabled  bool
	Filter   string
	Port     string
	Protocol string
	LogPath  []string
	Backend  string
	MaxRetry *int
	FindTime *time.Duration
	BanTime  *time.Duration
	Action   []string
	IgnoreIP []string
	Options  []jailOption
}

type jailOption struct {
	Key   string
	Value string
}

var reservedOptionKeys = map[string]struct{}{
	jailKeyEnabled:  {},
	jailKeyFilter:   {},
	jailKeyPort:     {},
	jailKeyProtocol: {},
	jailKeyLogPath:  {},
	jailKeyBackend:  {},
	jailKeyMaxRetry: {},
	jailKeyFindTime: {},
	jailKeyBanTime:  {},
	jailKeyAction:   {},
	jailKeyIgnoreIP: {},
}

func normalizeRules(raw map[string]Rule) ([]jailRule, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	rules := make([]jailRule, 0, len(raw))
	for name, rule := range raw {
		normalized, err := normalizeRule(name, rule)
		if err != nil {
			return nil, err
		}
		rules = append(rules, normalized)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})
	return rules, nil
}

func normalizeRule(name string, rule Rule) (jailRule, error) {
	if err := taskutil.ValidateIdentifier("fail2ban rule", name); err != nil {
		return jailRule{}, err
	}

	normalized := jailRule{
		Name:     name,
		Enabled:  ruleEnabled(rule),
		Filter:   strings.TrimSpace(rule.Filter),
		Port:     strings.TrimSpace(rule.Port),
		Protocol: strings.TrimSpace(rule.Protocol),
		Backend:  strings.TrimSpace(rule.Backend),
		LogPath:  strutil.CleanList([]string(rule.LogPath)),
		Action:   strutil.CleanList([]string(rule.Action)),
		IgnoreIP: strutil.CleanList([]string(rule.IgnoreIP)),
		MaxRetry: rule.MaxRetry,
		FindTime: rule.FindTime,
		BanTime:  rule.BanTime,
	}

	if normalized.Filter == "" {
		normalized.Filter = name
	}

	if err := validateRuleValues(name, normalized); err != nil {
		return jailRule{}, err
	}

	options, err := normalizeOptions(name, rule.Options)
	if err != nil {
		return jailRule{}, err
	}
	normalized.Options = options

	return normalized, nil
}

func ruleEnabled(rule Rule) bool {
	if rule.Enabled == nil {
		return true
	}
	return *rule.Enabled
}

func ruleErrorf(ruleName, format string, args ...any) error {
	return fmt.Errorf("fail2ban rule %q "+format, append([]any{ruleName}, args...)...)
}

func validateRuleValues(name string, rule jailRule) error {
	if rule.MaxRetry != nil && *rule.MaxRetry <= 0 {
		return ruleErrorf(name, "%s must be positive", ruleFieldMaxRetry)
	}
	if err := validateOptionalDuration(name, ruleFieldFindTime, rule.FindTime); err != nil {
		return err
	}
	if err := validateOptionalDuration(name, ruleFieldBanTime, rule.BanTime); err != nil {
		return err
	}
	if err := validateSingleLine(name, ruleFieldFilter, rule.Filter); err != nil {
		return err
	}
	if err := validateSingleLine(name, ruleFieldPort, rule.Port); err != nil {
		return err
	}
	if err := validateSingleLine(name, ruleFieldProtocol, rule.Protocol); err != nil {
		return err
	}
	if err := validateSingleLine(name, ruleFieldBackend, rule.Backend); err != nil {
		return err
	}
	for _, value := range rule.LogPath {
		if err := validateSingleLine(name, ruleFieldLogPath, value); err != nil {
			return err
		}
	}
	for _, value := range rule.Action {
		if err := validateSingleLine(name, ruleFieldAction, value); err != nil {
			return err
		}
	}
	for _, value := range rule.IgnoreIP {
		if err := validateSingleLine(name, ruleFieldIgnoreIP, value); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalDuration(ruleName, field string, value *time.Duration) error {
	if value == nil {
		return nil
	}
	if *value <= 0 {
		return ruleErrorf(ruleName, "%s must be positive", field)
	}
	return nil
}

func validateSingleLine(ruleName, field, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return ruleErrorf(ruleName, "%s cannot contain newlines", field)
	}
	return nil
}

func normalizeOptions(ruleName string, options map[string]any) ([]jailOption, error) {
	if len(options) == 0 {
		return nil, nil
	}
	normalized := make([]jailOption, 0, len(options))
	for key, value := range options {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, ruleErrorf(ruleName, "option key cannot be empty")
		}
		if err := taskutil.ValidateIdentifier("fail2ban option", key); err != nil {
			return nil, err
		}
		if _, exists := reservedOptionKeys[strings.ToLower(key)]; exists {
			return nil, ruleErrorf(ruleName, "option %q conflicts with built-in settings", key)
		}
		valueStr, err := formatOptionValue(value)
		if err != nil {
			return nil, ruleErrorf(ruleName, "option %q: %w", key, err)
		}
		if strings.ContainsAny(valueStr, "\r\n") {
			return nil, ruleErrorf(ruleName, "option %q cannot contain newlines", key)
		}
		normalized = append(normalized, jailOption{
			Key:   key,
			Value: valueStr,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Key < normalized[j].Key
	})
	return normalized, nil
}

func formatOptionValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "", fmt.Errorf("option value cannot be empty")
		}
		return trimmed, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case time.Duration:
		seconds, err := durationSeconds(v)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(seconds, 10), nil
	default:
		return "", fmt.Errorf("unsupported option value type %T", value)
	}
}

func renderJailConfig(rules []jailRule) (string, error) {
	var buf strings.Builder
	buf.WriteString("# Managed by settled. Manual changes may be overwritten.\n")

	for idx, rule := range rules {
		if idx > 0 {
			buf.WriteString("\n")
		}
		writer := jailConfigWriter{
			buf:      &buf,
			ruleName: rule.Name,
		}
		fmt.Fprintf(&buf, "[%s]\n", rule.Name)
		writer.writeKeyValue(jailKeyEnabled, strconv.FormatBool(rule.Enabled))
		writer.writeString(jailKeyFilter, rule.Filter)
		writer.writeString(jailKeyPort, rule.Port)
		writer.writeString(jailKeyProtocol, rule.Protocol)
		writer.writeList(jailKeyLogPath, rule.LogPath)
		writer.writeString(jailKeyBackend, rule.Backend)
		writer.writeInt(jailKeyMaxRetry, rule.MaxRetry)
		if err := writer.writeDuration(jailKeyFindTime, ruleFieldFindTime, rule.FindTime); err != nil {
			return "", err
		}
		if err := writer.writeDuration(jailKeyBanTime, ruleFieldBanTime, rule.BanTime); err != nil {
			return "", err
		}
		writer.writeList(jailKeyAction, rule.Action)
		writer.writeJoin(jailKeyIgnoreIP, rule.IgnoreIP)
		for _, option := range rule.Options {
			writer.writeKeyValue(option.Key, option.Value)
		}
	}

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	return output, nil
}

func writeListSetting(buf *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(buf, "%s = %s\n", key, values[0])
	for _, value := range values[1:] {
		fmt.Fprintf(buf, "%s%s\n", continuationIndent, value)
	}
}

func durationSeconds(value time.Duration) (int64, error) {
	if value <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	if value%time.Second != 0 {
		return 0, fmt.Errorf("duration must be in whole seconds")
	}
	return int64(value / time.Second), nil
}

func configMatches(existing, desired string) bool {
	return strings.TrimSpace(existing) == strings.TrimSpace(desired)
}

func fail2banInstalled(ctx context.Context, s server.Server) (bool, error) {
	script := fmt.Sprintf(
		"if command -v %s >/dev/null 2>&1; then echo %s; else echo %s; fi",
		fail2banClientCmd,
		scriptOutputYes,
		scriptOutputNo,
	)
	output, err := runScript(ctx, s, "", script)
	if err != nil {
		return false, fmt.Errorf("check fail2ban install: %w", err)
	}
	return strings.TrimSpace(output) == scriptOutputYes, nil
}

func fail2banServiceReady(ctx context.Context, s server.Server, prefix string) (bool, error) {
	script, err := renderFail2banScript("service_ready", newFail2banScriptData())
	if err != nil {
		return false, err
	}
	output, err := runScript(ctx, s, prefix, script)
	if err != nil {
		return false, fmt.Errorf("check fail2ban service: %w", err)
	}
	return strings.TrimSpace(output) == scriptOutputYes, nil
}
