package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tpodg/settled/internal/app"
	"github.com/tpodg/settled/internal/config"
	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/testutils"
)

func TestBootstrapCommand_Integration(t *testing.T) {
	ctx := context.Background()
	sshC := testutils.SetupSSHContainer(t, ctx)
	defer sshC.Container.Terminate(ctx)

	// Wait a bit for the SSH server to be fully ready
	time.Sleep(2 * time.Second)

	origUser := bootstrapUser
	origLogin := bootstrapLoginUser
	origGroup := bootstrapGroup
	origNoPass := bootstrapSudoNoPasswd
	origKeys := bootstrapAuthorizedKeys
	t.Cleanup(func() {
		bootstrapUser = origUser
		bootstrapLoginUser = origLogin
		bootstrapGroup = origGroup
		bootstrapSudoNoPasswd = origNoPass
		bootstrapAuthorizedKeys = origKeys
	})

	bootstrapUser = "bootstrapper"
	bootstrapLoginUser = "root"
	bootstrapGroup = "sudo"
	bootstrapSudoNoPasswd = true
	bootstrapAuthorizedKeys = nil

	cfg := &config.Config{
		Servers: []config.ServerConfig{
			{
				Name:           "bootstrap-test",
				Address:        sshC.Address,
				User:           bootstrapLoginUser,
				SSHKey:         sshC.KeyPath,
				KnownHostsPath: sshC.KnownHostsPath,
			},
		},
	}
	settleApp := app.New(cfg)

	bootstrapCmd.SetContext(context.WithValue(context.Background(), appKey, settleApp))
	bootstrapCmd.Run(bootstrapCmd, nil)

	newSrv := server.NewSSHServer("bootstrap-login", sshC.Address, bootstrapUser, sshC.KeyPath, sshC.KnownHostsPath)
	output, err := newSrv.Execute(ctx, "whoami")
	if err != nil {
		t.Fatalf("failed to login as new user: %v", err)
	}
	if strings.TrimSpace(output) != bootstrapUser {
		t.Fatalf("expected whoami %q, got %q", bootstrapUser, strings.TrimSpace(output))
	}
}
