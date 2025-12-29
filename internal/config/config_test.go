package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/task/users"
)

func TestLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settled-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, DefaultConfigFileName)
	configContent := `
servers:
  - name: test-server
    address: 1.2.3.4
    user: test-user
    use_agent: false
    handshake_timeout: 12s
    tasks:
      ssh:
        hardening: true
      users:
        test_user: { sudo: false }
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	t.Run("load tasks config", func(t *testing.T) {
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(cfg.Servers) != 1 {
			t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
		}

		s := cfg.Servers[0]
		if s.UseAgent == nil || *s.UseAgent != false {
			t.Fatalf("expected use_agent=false, got %v", s.UseAgent)
		}
		if s.HandshakeTimeout != 12*time.Second {
			t.Fatalf("expected handshake_timeout=12s, got %s", s.HandshakeTimeout)
		}
		ssh, ok := s.Tasks["ssh"].(map[string]any)
		if !ok || ssh["hardening"] != true {
			t.Fatalf("expected ssh hardening=true, got %+v", s.Tasks["ssh"])
		}

		usersConfig, ok := s.Tasks[users.TaskKey].(map[string]any)
		if !ok {
			t.Fatalf("expected users config map, got %T", s.Tasks[users.TaskKey])
		}
		testUser, ok := usersConfig["test_user"].(map[string]any)
		if !ok || testUser["sudo"] != false {
			t.Fatalf("expected test_user sudo=false, got %+v", usersConfig["test_user"])
		}
	})

	t.Run("load from non-existent file", func(t *testing.T) {
		_, err := Load("non-existent-file.yaml")
		if err == nil {
			t.Error("expected error when loading non-existent file, got nil")
		}
	})
}
