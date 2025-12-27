package testutils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHContainer struct {
	Container      testcontainers.Container
	Address        string
	User           string
	KeyPath        string
	KnownHostsPath string
}

const (
	defaultSSHStartupTimeout = 60 * time.Second
	defaultSSHPort           = "22"
	legacySSHPort            = "2222"
)

func SetupSSHContainer(t *testing.T, ctx context.Context) *SSHContainer {
	t.Helper()

	// 1. Generate SSH key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_rsa")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(privateKeyPEM), 0600)
	if err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	// Generate public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to create public key: %v", err)
	}
	pubKeyStr := string(ssh.MarshalAuthorizedKey(pub))

	// 2. Start SSH container
	image := os.Getenv("SETTLED_TEST_SSH_IMAGE")
	sshPort := defaultSSHPort
	if image != "" {
		if envPort := os.Getenv("SETTLED_TEST_SSH_PORT"); envPort != "" {
			sshPort = envPort
		} else {
			sshPort = legacySSHPort
		}
	}

	portSpec := sshPort + "/tcp"
	natPort := nat.Port(portSpec)

	req := testcontainers.ContainerRequest{
		ExposedPorts: []string{portSpec},
		Env: map[string]string{
			"PUBLIC_KEY": pubKeyStr,
			"USER_NAME":  "testuser",
		},
		WaitingFor: wait.ForListeningPort(natPort).WithStartupTimeout(defaultSSHStartupTimeout),
	}

	if image == "" {
		contextDir, err := sshImageContext()
		if err != nil {
			t.Fatalf("failed to resolve ssh image context: %v", err)
		}
		req.FromDockerfile = testcontainers.FromDockerfile{
			Context:    contextDir,
			Dockerfile: "Dockerfile",
		}
	} else {
		req.Image = image
	}

	sshContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	host, err := sshContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := sshContainer.MappedPort(ctx, natPort)
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	address := fmt.Sprintf("%s:%s", host, port.Port())

	hostKey, err := fetchHostKey(ctx, address)
	if err != nil {
		t.Fatalf("failed to fetch host key: %v", err)
	}

	knownHostsPath := filepath.Join(tmpDir, "known_hosts")
	knownHostsLine := knownhosts.Line([]string{address}, hostKey)
	if err := os.WriteFile(knownHostsPath, []byte(knownHostsLine+"\n"), 0600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	return &SSHContainer{
		Container:      sshContainer,
		Address:        address,
		User:           "root",
		KeyPath:        keyPath,
		KnownHostsPath: knownHostsPath,
	}
}

func sshImageContext() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("caller info unavailable")
	}
	return filepath.Join(filepath.Dir(filename), "ssh_ubuntu"), nil
}

func fetchHostKey(ctx context.Context, address string) (ssh.PublicKey, error) {
	var hostKey ssh.PublicKey
	config := &ssh.ClientConfig{
		User: "testuser",
		Auth: []ssh.AuthMethod{
			ssh.Password("invalid"),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			hostKey = key
			return nil
		},
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}
	defer conn.Close()

	_, _, _, err = ssh.NewClientConn(conn, address, config)
	if hostKey == nil {
		if err != nil {
			return nil, fmt.Errorf("failed to capture host key: %w", err)
		}
		return nil, errors.New("failed to capture host key")
	}
	return hostKey, nil
}
