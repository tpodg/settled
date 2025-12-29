package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tpodg/settled/internal/server"
	"github.com/tpodg/settled/internal/strutil"
	"github.com/tpodg/settled/internal/task"
	"github.com/tpodg/settled/internal/task/users"
)

var (
	bootstrapUser           string
	bootstrapLoginUser      string
	bootstrapGroup          string
	bootstrapSudoNoPasswd   bool
	bootstrapAuthorizedKeys []string
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Create the initial sudo user on servers",
	Long:  "Create the initial sudo user using the configured login user.",
	Run: func(cmd *cobra.Command, args []string) {
		settleApp := getApp(cmd)
		settleApp.Logger.Info("Starting bootstrap process")

		if len(settleApp.Config.Servers) == 0 {
			settleApp.Logger.Warn("No servers provided for bootstrap")
			return
		}

		newUser := strings.TrimSpace(bootstrapUser)
		if newUser == "" {
			settleApp.Logger.Error("Bootstrap user is required")
			return
		}

		group := strings.TrimSpace(bootstrapGroup)
		loginUser := strings.TrimSpace(bootstrapLoginUser)
		if loginUser == "" {
			loginUser = "root"
		}
		runner := task.NewRunner(settleApp.Logger)

		for _, s := range settleApp.Config.Servers {
			settleApp.Logger.Info("Bootstrapping server", "name", s.Name, "address", s.Address)

			srv := server.NewSSHServer(s.Name, s.Address, loginUser, s.SSHKey, s.KnownHostsPath, server.SSHOptions{
				UseAgent:         s.UseAgent,
				HandshakeTimeout: s.HandshakeTimeout,
			})

			keys, err := resolveBootstrapKeys(cmd.Context(), srv, bootstrapAuthorizedKeys, loginUser)
			if err != nil {
				settleApp.Logger.Error("Failed to resolve authorized keys", "server", s.Name, "error", err)
				continue
			}

			userCfg := map[string]any{
				"sudo": true,
			}
			if bootstrapSudoNoPasswd {
				userCfg["sudo_nopasswd"] = true
			}
			if group != "" {
				userCfg["groups"] = []string{group}
			}
			if len(keys) > 0 {
				userCfg["authorized_keys"] = keys
			}

			overrides := map[string]any{
				users.TaskKey: map[string]any{
					newUser: userCfg,
				},
			}

			tasks, unknown, err := task.PlanTasks(overrides, []task.Spec{users.Spec()})
			if err != nil {
				settleApp.Logger.Error("Failed to plan bootstrap tasks", "server", s.Name, "error", err)
				continue
			}

			if len(unknown) > 0 {
				settleApp.Logger.Warn("Ignoring unknown bootstrap task keys", "server", s.Name, "keys", unknown)
			}

			if len(tasks) == 0 {
				settleApp.Logger.Info("No bootstrap tasks to apply for server", "name", s.Name)
				continue
			}

			configurator := task.NewTaskConfigurator(runner, tasks...)

			if err := configurator.Configure(cmd.Context(), srv); err != nil {
				settleApp.Logger.Error("Failed to bootstrap server", "name", s.Name, "error", err)
				continue
			}

			settleApp.Logger.Info("Server bootstrapped successfully", "name", s.Name)
		}
	},
}

func resolveBootstrapKeys(ctx context.Context, srv server.Server, provided []string, loginUser string) ([]string, error) {
	keys := strutil.CleanList(provided)
	if len(keys) > 0 {
		return keys, nil
	}

	userEsc := strutil.ShellEscape(loginUser)
	script := fmt.Sprintf("set -e; home=$(getent passwd %s | cut -d: -f6); if [ -z \"$home\" ]; then home=/root; fi; cat \"$home/%s/%s\"", userEsc, users.SSHDirName, users.AuthorizedKeysFileName)
	output, err := srv.Execute(ctx, "sh -c "+strutil.ShellEscape(script))
	if err != nil {
		return nil, fmt.Errorf("read authorized_keys for %s (use --authorized-key to override): %w", loginUser, err)
	}

	keys = strutil.CleanList(strings.Split(output, "\n"))
	if len(keys) == 0 {
		return nil, fmt.Errorf("authorized_keys for %s is empty", loginUser)
	}
	return keys, nil
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapUser, "user", "", "Username for the new sudo user (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapLoginUser, "login-user", "root", "SSH user to run bootstrap (default root)")
	bootstrapCmd.Flags().StringVar(&bootstrapGroup, "group", "sudo", "Additional group to assign to the new user (optional)")
	bootstrapCmd.Flags().BoolVar(&bootstrapSudoNoPasswd, "sudo-nopasswd", false, "Allow passwordless sudo")
	bootstrapCmd.Flags().StringSliceVar(&bootstrapAuthorizedKeys, "authorized-key", nil, "Authorized SSH public key for the new user (repeatable; defaults to login user's keys)")
	cobra.CheckErr(bootstrapCmd.MarkFlagRequired("user"))
	rootCmd.AddCommand(bootstrapCmd)
}
