package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/testutils"
)

func TestSSHServer_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// 3. Test SSHServer
	s := NewSSHServer("test-container", sshC.Address, User{
		Name:   sshC.User,
		SSHKey: sshC.KeyPath,
	}, sshC.KnownHostsPath, SSHOptions{})

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	output, err := s.Execute(ctx, "echo 'hello world'")
	if err != nil {
		t.Fatalf("Execute failed: %v\nOutput: %s", err, output)
	}

	expected := "hello world\n"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestSSHServer_SudoPassword(t *testing.T) {
	ctx := context.Background()
	sudoNoPasswd := false
	sshC := testutils.SetupSSHContainerWithOptions(t, ctx, testutils.SSHContainerOptions{
		UserName:     "sudoer",
		UserPassword: "sudopass",
		SudoNoPasswd: &sudoNoPasswd,
	})
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	baseUser := User{
		Name:   "sudoer",
		SSHKey: sshC.KeyPath,
	}

	s := NewSSHServer("sudo-test", sshC.Address, baseUser, sshC.KnownHostsPath, SSHOptions{})
	if _, err := s.Execute(ctx, "sudo -n id -u"); err == nil {
		t.Fatal("expected sudo without password to fail")
	}

	sWrong := NewSSHServer("sudo-test", sshC.Address, User{
		Name:         baseUser.Name,
		SSHKey:       baseUser.SSHKey,
		SudoPassword: "wrong",
	}, sshC.KnownHostsPath, SSHOptions{})
	if _, err := sWrong.Execute(ctx, "sudo -n id -u"); err == nil {
		t.Fatal("expected sudo with wrong password to fail")
	}

	sOK := NewSSHServer("sudo-test", sshC.Address, User{
		Name:         baseUser.Name,
		SSHKey:       baseUser.SSHKey,
		SudoPassword: "sudopass",
	}, sshC.KnownHostsPath, SSHOptions{})
	output, err := sOK.Execute(ctx, "sudo -n id -u")
	if err != nil {
		t.Fatalf("expected sudo with correct password to succeed: %v", err)
	}
	if strings.TrimSpace(output) != "0" {
		t.Fatalf("expected sudo to return uid 0, got %q", strings.TrimSpace(output))
	}
}
