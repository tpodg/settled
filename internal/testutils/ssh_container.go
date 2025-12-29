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
	"sync"
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

type SSHContainerOptions struct {
	UserName     string
	UserPassword string
	SudoNoPasswd *bool
}

const (
	defaultSSHStartupTimeout = 60 * time.Second
	defaultSSHPort           = "22"
)

var (
	sshImageOnce sync.Once
	sshImageName string
	sshImageErr  error
)

func SetupSSHContainer(t *testing.T, ctx context.Context) *SSHContainer {
	return SetupSSHContainerWithOptions(t, ctx, SSHContainerOptions{})
}

func SetupSSHContainerWithOptions(t *testing.T, ctx context.Context, opts SSHContainerOptions) *SSHContainer {
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
	image, err := ensureSSHImage()
	if err != nil {
		t.Fatalf("failed to build ssh image: %v", err)
	}

	portSpec := defaultSSHPort + "/tcp"
	natPort := nat.Port(portSpec)

	userName := opts.UserName
	if userName == "" {
		userName = "testuser"
	}
	sudoNoPasswd := true
	if opts.SudoNoPasswd != nil {
		sudoNoPasswd = *opts.SudoNoPasswd
	}

	env := map[string]string{
		"PUBLIC_KEY": pubKeyStr,
		"USER_NAME":  userName,
	}
	if opts.UserPassword != "" {
		env["USER_PASSWORD"] = opts.UserPassword
	}
	if sudoNoPasswd {
		env["SUDO_NOPASSWD"] = "1"
	} else {
		env["SUDO_NOPASSWD"] = "0"
	}

	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{portSpec},
		Env:          env,
		WaitingFor:   wait.ForListeningPort(natPort).WithStartupTimeout(defaultSSHStartupTimeout),
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

func ensureSSHImage() (string, error) {
	sshImageOnce.Do(func() {
		contextDir, err := sshImageContext()
		if err != nil {
			sshImageErr = err
			return
		}

		provider, err := testcontainers.NewDockerProvider()
		if err != nil {
			sshImageErr = err
			return
		}

		buildReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    contextDir,
				Dockerfile: "Dockerfile",
			},
		}
		// Use a background context so a test-scoped cancellation does not poison the shared build.
		sshImageName, sshImageErr = provider.BuildImage(context.Background(), &buildReq)
	})

	if sshImageErr != nil {
		return "", sshImageErr
	}
	if sshImageName == "" {
		return "", errors.New("built ssh image tag is empty")
	}
	return sshImageName, nil
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
