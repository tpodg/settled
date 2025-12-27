package cli

import (
	"github.com/spf13/cobra"
	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/task"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure one or more servers",
	Long:  `Apply hardening and configuration steps to the specified servers.`,
	Run: func(cmd *cobra.Command, args []string) {
		settleApp := getApp(cmd)
		settleApp.Logger.Info("Starting configuration process")

		if len(settleApp.Config.Servers) == 0 {
			settleApp.Logger.Warn("No servers provided for configuration")
			return
		}

		settleApp.Logger.Info("Configuring servers", "count", len(settleApp.Config.Servers))

		// Initialize the task runner
		runner := task.NewRunner(settleApp.Logger)

		for _, s := range settleApp.Config.Servers {
			settleApp.Logger.Info("Configuring server", "name", s.Name, "address", s.Address)

			srv := server.NewSSHServer(s.Name, s.Address, s.User, s.SSHKey, s.KnownHostsPath)

			tasks, unknown, err := task.PlanTasks(s.Tasks, task.Builtins())
			if err != nil {
				settleApp.Logger.Error("Failed to plan tasks", "server", s.Name, "error", err)
				continue
			}

			if len(unknown) > 0 {
				settleApp.Logger.Warn("Ignoring unknown task keys", "server", s.Name, "keys", unknown)
			}

			if len(tasks) == 0 {
				settleApp.Logger.Info("No tasks to apply for server", "name", s.Name)
				continue
			}

			configurator := task.NewTaskConfigurator(runner, tasks...)

			if err := configurator.Configure(cmd.Context(), srv); err != nil {
				settleApp.Logger.Error("Failed to configure server", "name", s.Name, "error", err)
				continue
			}

			settleApp.Logger.Info("Server configured successfully", "name", s.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)
}
