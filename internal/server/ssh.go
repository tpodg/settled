package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHServer struct {
	name           string
	address        string
	user           User
	knownHostsPath string
	opts           SSHOptions
}

type SSHOptions struct {
	UseAgent         *bool
	HandshakeTimeout time.Duration
}

const defaultSSHHandshakeTimeout = 15 * time.Second

func NewSSHServer(name, address string, user User, knownHostsPath string, opts SSHOptions) *SSHServer {
	return &SSHServer{
		name:           name,
		address:        address,
		user:           user,
		knownHostsPath: knownHostsPath,
		opts:           opts,
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

	// Prefer explicit key material before falling back to the agent.
	if s.user.SSHKey != "" {
		expandedPath, err := expandPath(s.user.SSHKey)
		if err != nil {
			return "", fmt.Errorf("failed to expand ssh key path %q: %w", s.user.SSHKey, err)
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

	if s.useAgent() {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if agentConn, err := net.Dial("unix", sock); err == nil {
				authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
				defer agentConn.Close()
			}
		}
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
		User:            s.user.Name,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	// Use a dialer that supports context
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	if err := applyHandshakeDeadline(ctx, conn, s.handshakeTimeout()); err != nil {
		conn.Close()
		return "", err
	}
	handshakeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-handshakeDone:
		}
	}()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		close(handshakeDone)
		conn.Close()
		return "", fmt.Errorf("failed to establish ssh connection to %s: %w", addr, err)
	}
	close(handshakeDone)
	if err := clearDeadline(conn); err != nil {
		sshConn.Close()
		return "", err
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

	commandToRun := command
	if s.user.SudoPassword != "" && strings.HasPrefix(command, "sudo -n ") {
		commandToRun = "sudo -S -p '' " + strings.TrimPrefix(command, "sudo -n ")
		session.Stdin = strings.NewReader(s.user.SudoPassword + "\n")
	}

	output, err := session.CombinedOutput(commandToRun)
	if err != nil {
		return string(output), fmt.Errorf("command %q failed: %w", commandToRun, err)
	}

	return string(output), nil
}

func (s *SSHServer) useAgent() bool {
	if s.opts.UseAgent == nil {
		return true
	}
	return *s.opts.UseAgent
}

func (s *SSHServer) handshakeTimeout() time.Duration {
	if s.opts.HandshakeTimeout > 0 {
		return s.opts.HandshakeTimeout
	}
	return defaultSSHHandshakeTimeout
}

func applyHandshakeDeadline(ctx context.Context, conn net.Conn, timeout time.Duration) error {
	deadline, ok := handshakeDeadline(ctx, timeout)
	if !ok {
		return nil
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set ssh handshake deadline: %w", err)
	}
	return nil
}

func clearDeadline(conn net.Conn) error {
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return fmt.Errorf("clear ssh handshake deadline: %w", err)
	}
	return nil
}

func handshakeDeadline(ctx context.Context, timeout time.Duration) (time.Time, bool) {
	var deadline time.Time
	now := time.Now()
	if timeout > 0 {
		deadline = now.Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
	}
	if deadline.IsZero() {
		return time.Time{}, false
	}
	return deadline, true
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
