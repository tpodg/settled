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
    ssh_key: /path/to/key
    known_hosts: /path/to/known_hosts
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write temp config file: %v", err)
	}

	t.Run("load from specific file", func(t *testing.T) {
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(cfg.Servers) != 1 {
			t.Fatalf("expected 1 server, got %d", len(cfg.Servers))
		}

		s := cfg.Servers[0]
		if s.Name != "test-server" {
			t.Errorf("expected server name 'test-server', got '%s'", s.Name)
		}
		if s.Address != "1.2.3.4" {
			t.Errorf("expected server address '1.2.3.4', got '%s'", s.Address)
		}
		if s.User != "test-user" {
			t.Errorf("expected server user 'test-user', got '%s'", s.User)
		}
		if s.SSHKey != "/path/to/key" {
			t.Errorf("expected ssh_key '/path/to/key', got '%s'", s.SSHKey)
		}
		if s.KnownHostsPath != "/path/to/known_hosts" {
			t.Errorf("expected known_hosts '/path/to/known_hosts', got '%s'", s.KnownHostsPath)
		}
	})

	t.Run("load from non-existent file", func(t *testing.T) {
		_, err := Load("non-existent-file.yaml")
		if err == nil {
			t.Error("expected error when loading non-existent file, got nil")
		}
	})
}
