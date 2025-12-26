package cli

import (
	"github.com/spf13/cobra"
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
		for _, s := range settleApp.Config.Servers {
			settleApp.Logger.Info("Configuring server", "name", s.Name, "address", s.Address)
		}
		// TODO
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)
}
