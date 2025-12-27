package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settled-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ".settled.yaml")
	configContent := `
servers:
  - name: test-server
    address: 1.2.3.4
    user: test-user
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
		ssh, ok := s.Tasks["ssh"].(map[string]any)
		if !ok || ssh["hardening"] != true {
			t.Fatalf("expected ssh hardening=true, got %+v", s.Tasks["ssh"])
		}

		users, ok := s.Tasks["users"].(map[string]any)
		if !ok {
			t.Fatalf("expected users config map, got %T", s.Tasks["users"])
		}
		testUser, ok := users["test_user"].(map[string]any)
		if !ok || testUser["sudo"] != false {
			t.Fatalf("expected test_user sudo=false, got %+v", users["test_user"])
		}
	})

	t.Run("load from non-existent file", func(t *testing.T) {
		_, err := Load("non-existent-file.yaml")
		if err == nil {
			t.Error("expected error when loading non-existent file, got nil")
		}
	})
}
