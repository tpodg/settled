package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tpodg/settled/internal/app"
	"github.com/tpodg/settled/internal/config"
)

type contextKey string

const appKey contextKey = "app"

var rootCmd = &cobra.Command{
	Use:   "settle",
	Short: "Settled is a tool for server configuration and provisioning",
	Long: `Settled helps you prepare your servers for production by automating
basic configuration steps like installing fail2ban, disabling root login, 
and more.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfgFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		settleApp := app.New(cfg)
		ctx := context.WithValue(cmd.Context(), appKey, settleApp)
		cmd.SetContext(ctx)

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("config", "", fmt.Sprintf("config file (default is $HOME/%s)", config.DefaultConfigFileName))
}

func getApp(cmd *cobra.Command) *app.App {
	if a, ok := cmd.Context().Value(appKey).(*app.App); ok {
		return a
	}
	return nil
}
