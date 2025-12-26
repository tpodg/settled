package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHServer struct {
	name           string
	address        string
	user           string
	keyPath        string
	knownHostsPath string
}

func NewSSHServer(name, address, user, keyPath, knownHostsPath string) *SSHServer {
	return &SSHServer{
		name:           name,
		address:        address,
		user:           user,
		keyPath:        keyPath,
		knownHostsPath: knownHostsPath,
	}
}

func (s *SSHServer) ID() string      { return s.name }
func (s *SSHServer) Address() string { return s.address }

func (s *SSHServer) Execute(ctx context.Context, command string) (string, error) {
	addr := s.address
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}

	authMethods := []ssh.AuthMethod{}

	// 1. Try SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if agentConn, err := net.Dial("unix", sock); err == nil {
			authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
			defer agentConn.Close()
		}
	}

	// 2. Try private key if provided
	if s.keyPath != "" {
		expandedPath, err := expandPath(s.keyPath)
		if err != nil {
			return "", fmt.Errorf("failed to expand ssh key path %q: %w", s.keyPath, err)
		}
		key, err := os.ReadFile(expandedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read ssh key %q: %w", expandedPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("failed to parse ssh key %q: %w", expandedPath, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return "", fmt.Errorf("no ssh authentication methods available")
	}

	knownHostsPath, err := resolveKnownHostsPath(s.knownHostsPath)
	if err != nil {
		return "", err
	}
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return "", fmt.Errorf("failed to load known_hosts file %q: %w", knownHostsPath, err)
	}

	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	// Use a dialer that supports context
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return "", fmt.Errorf("failed to establish ssh connection to %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	// Handle context cancellation
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			client.Close()
		case <-done:
		}
	}()
	defer close(done)

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command %q failed: %w", command, err)
	}

	return string(output), nil
}

func resolveKnownHostsPath(path string) (string, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory for known_hosts: %w", err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	return expandPath(path)
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
