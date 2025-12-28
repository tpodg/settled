package server

import (
	"context"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/testutils"
)

func TestSSHServer_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// 3. Test SSHServer
	s := NewSSHServer("test-container", sshC.Address, sshC.User, sshC.KeyPath, sshC.KnownHostsPath, SSHOptions{})

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
