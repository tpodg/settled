package fail2ban

import (
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
)

func TestStringListUnmarshal(t *testing.T) {
	type sample struct {
		Values StringList `yaml:"values"`
	}

	t.Run("string", func(t *testing.T) {
		var cfg sample
		if err := yaml.Unmarshal([]byte("values: /var/log/auth.log\n"), &cfg); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(cfg.Values) != 1 || cfg.Values[0] != "/var/log/auth.log" {
			t.Fatalf("unexpected values: %#v", cfg.Values)
		}
	})

	t.Run("list", func(t *testing.T) {
		var cfg sample
		data := "values:\n  - /var/log/auth.log\n  - /var/log/secure\n"
		if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if len(cfg.Values) != 2 || cfg.Values[1] != "/var/log/secure" {
			t.Fatalf("unexpected values: %#v", cfg.Values)
		}
	})
}

func TestRenderJailConfig(t *testing.T) {
	enabled := true
	maxRetry := 5
	findTime := 10 * time.Minute
	banTime := time.Hour

	rules := []jailRule{
		{
			Name:     "sshd",
			Enabled:  enabled,
			Filter:   "sshd",
			Port:     "ssh",
			LogPath:  []string{"/var/log/auth.log"},
			MaxRetry: &maxRetry,
			FindTime: &findTime,
			BanTime:  &banTime,
		},
	}

	config, err := renderJailConfig(rules)
	if err != nil {
		t.Fatalf("renderJailConfig failed: %v", err)
	}
	for _, line := range []string{
		"[sshd]\n",
		"enabled = true\n",
		"filter = sshd\n",
		"port = ssh\n",
		"logpath = /var/log/auth.log\n",
		"maxretry = 5\n",
		"findtime = 600\n",
		"bantime = 3600\n",
	} {
		if !strings.Contains(config, line) {
			t.Fatalf("expected config to contain %q, got:\n%s", line, config)
		}
	}
}

func TestRenderJailConfigMultilineSettings(t *testing.T) {
	rules := []jailRule{
		{
			Name:    "api",
			Enabled: true,
			Filter:  "api",
			LogPath: []string{"/var/log/api.log", "/var/log/api-fail.log"},
			Action:  []string{"iptables[name=api]", "sendmail[name=api]"},
		},
	}

	config, err := renderJailConfig(rules)
	if err != nil {
		t.Fatalf("renderJailConfig failed: %v", err)
	}

	if !strings.Contains(config, "logpath = /var/log/api.log\n"+continuationIndent+"/var/log/api-fail.log\n") {
		t.Fatalf("expected multiline logpath, got:\n%s", config)
	}
	if !strings.Contains(config, "action = iptables[name=api]\n"+continuationIndent+"sendmail[name=api]\n") {
		t.Fatalf("expected multiline action, got:\n%s", config)
	}
}

func TestNormalizeRuleReservedOptions(t *testing.T) {
	_, err := normalizeRule("sshd", Rule{
		Options: map[string]any{
			"maxretry": 2,
		},
	})
	if err == nil {
		t.Fatal("expected error for reserved option key")
	}
}

func TestRenderScript(t *testing.T) {
	task := &Fail2banTask{
		configPath:    defaultJailConfig,
		configContent: "test\n",
	}

	script, err := task.renderScript()
	if err != nil {
		t.Fatalf("renderScript failed: %v", err)
	}
	if script == "" {
		t.Fatal("renderScript returned empty script")
	}
}
