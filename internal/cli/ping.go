package cli

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tpodg/settled/internal/server"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Verify connection to servers",
	Long:  `Try to connect to all configured servers and execute a simple command to verify accessibility.`,
	Run: func(cmd *cobra.Command, args []string) {
		settleApp := getApp(cmd)
		settleApp.Logger.Info("Starting connection verification")

		if len(settleApp.Config.Servers) == 0 {
			settleApp.Logger.Warn("No servers configured")
			return
		}

		servers := make([]server.Server, 0, len(settleApp.Config.Servers))
		for _, sCfg := range settleApp.Config.Servers {
			servers = append(servers, server.NewSSHServer(sCfg.Name, sCfg.Address, server.User{
				Name:         sCfg.User.Name,
				SSHKey:       sCfg.User.SSHKey,
				SudoPassword: sCfg.User.SudoPassword,
			}, sCfg.KnownHostsPath, server.SSHOptions{
				UseAgent:         sCfg.UseAgent,
				HandshakeTimeout: sCfg.HandshakeTimeout,
			}))
		}

		verifyServers(settleApp.Logger, servers)
	},
}

func verifyServers(logger *slog.Logger, servers []server.Server) {
	for _, srv := range servers {
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			logger.Info("Checking server", "name", srv.ID(), "address", srv.Address())
			output, err := srv.Execute(ctx, "echo 'pong'")

			if err != nil {
				logger.Error("Verification failed", "server", srv.ID(), "error", err)
				return
			}

			if strings.TrimSpace(output) == "pong" {
				logger.Info("Verification successful", "server", srv.ID())
			} else {
				logger.Warn("Verification partially successful (unexpected output)", "server", srv.ID(), "output", strings.TrimSpace(output))
			}
		}()
	}
}

func init() {
	rootCmd.AddCommand(pingCmd)
}
