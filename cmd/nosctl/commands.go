package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// newLoginCmd creates the login command
func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with nOS",
		Long:  `Authenticate with nOS using an API token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Print("Enter API token: ")
			var inputToken string
			if _, err := fmt.Scanln(&inputToken); err != nil {
				return fmt.Errorf("read token: %w", err)
			}

			if inputToken == "" {
				return fmt.Errorf("token cannot be empty")
			}

			// Test the token
			client := newAPIClient(baseURL, inputToken)
			if err := client.testConnection(); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Save to config
			viper.Set("token", inputToken)
			viper.Set("url", baseURL)

			configPath := filepath.Join(os.Getenv("HOME"), ".config", "nos", "cli.yaml")
			if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
				return fmt.Errorf("failed to create config dir: %w", err)
			}

			if err := viper.WriteConfigAs(configPath); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			// The file holds the API token: keep it private to the user.
			if err := os.Chmod(configPath, 0o600); err != nil {
				return fmt.Errorf("failed to restrict config permissions: %w", err)
			}

			fmt.Println("✓ Authentication successful")
			fmt.Printf("✓ Configuration saved to %s\n", configPath)
			return nil
		},
	}

	return cmd
}

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show system status",
		Long:  `Display the current status of the nOS system.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := newAPIClient(baseURL, token)

			status, err := client.getSystemStatus()
			if err != nil {
				return err
			}

			if outputJSON {
				printJSON(status)
			} else {
				fmt.Printf("System Status\n")
				fmt.Printf("=============\n")
				fmt.Printf("Version:     %s\n", status.Version)
				fmt.Printf("Uptime:      %s\n", status.Uptime)
				fmt.Printf("Load:        %.2f, %.2f, %.2f\n", status.Load1, status.Load5, status.Load15)
				fmt.Printf("CPU:         %.1f%%\n", status.CPUUsage)
				fmt.Printf("Memory:      %s / %s (%.1f%%)\n",
					formatBytes(status.MemoryUsed),
					formatBytes(status.MemoryTotal),
					status.MemoryPercent)
				fmt.Printf("Storage:     %s / %s (%.1f%%)\n",
					formatBytes(status.StorageUsed),
					formatBytes(status.StorageTotal),
					status.StoragePercent)
				fmt.Printf("\nServices:\n")
				for _, service := range status.Services {
					mark := "✗"
					if service.Active {
						mark = "✓"
					}
					fmt.Printf("  %s %s - %s\n", mark, service.Name, service.State)
				}
			}

			return nil
		},
	}

	return cmd
}

// newSystemCmd creates the system command group
func newSystemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "System management commands",
		Long:  `Commands for managing system configuration and information.`,
	}

	// Add subcommands
	cmd.AddCommand(
		&cobra.Command{
			Use:   "info",
			Short: "Show system information",
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)
				info, err := client.getSystemInfo()
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(info)
				} else {
					fmt.Printf("System Information\n")
					fmt.Printf("==================\n")
					fmt.Printf("Hostname:    %s\n", info.Hostname)
					fmt.Printf("OS:          %s\n", info.OS)
					fmt.Printf("Kernel:      %s\n", info.Kernel)
					fmt.Printf("Arch:        %s\n", info.Arch)
					fmt.Printf("CPUs:        %d\n", info.CPUs)
					fmt.Printf("Memory:      %s\n", formatBytes(info.Memory))
					fmt.Printf("NOS Version: %s\n", info.NOSVersion)
				}

				return nil
			},
		},
	)

	return cmd
}

// newStorageCmd creates the storage command group
func newStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Storage management commands",
		Long:  `Commands for managing storage pools and snapshots.`,
	}

	// Snapshots subcommand
	snapshotsCmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage snapshots",
	}

	snapshotsCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List snapshots",
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)
				snapshots, err := client.listSnapshots()
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(snapshots)
				} else {
					headers := []string{"ID", "Subvolume", "Created", "Size"}
					rows := [][]string{}
					for _, snap := range snapshots {
						rows = append(rows, []string{
							snap.ID[:8],
							snap.Subvolume,
							snap.CreatedAt,
							formatBytes(snap.Size),
						})
					}
					printTable(headers, rows)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "create [subvolume]",
			Short: "Create a snapshot",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				tag, _ := cmd.Flags().GetString("tag")

				job, err := client.createSnapshot(args, tag)
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(job)
				} else {
					fmt.Printf("✓ Snapshot creation started\n")
					fmt.Printf("  Job ID: %s\n", job.ID)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [id]",
			Short: "Delete a snapshot",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				if err := client.deleteSnapshot(args[0]); err != nil {
					return err
				}

				fmt.Printf("✓ Snapshot deleted\n")
				return nil
			},
		},
	)

	snapshotsCmd.Flags().StringP("tag", "t", "", "snapshot tag")

	cmd.AddCommand(snapshotsCmd)

	return cmd
}

// newAppsCmd creates the apps command group
func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Application management commands",
		Long:  `Commands for managing applications.`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed applications",
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)
				apps, err := client.listApps()
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(apps)
				} else {
					headers := []string{"ID", "Name", "Version", "Status", "Health"}
					rows := [][]string{}
					for _, app := range apps {
						rows = append(rows, []string{
							app.ID,
							app.Name,
							app.Version,
							app.Status,
							app.Health,
						})
					}
					printTable(headers, rows)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "install [id]",
			Short: "Install an application",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				// Read params from file or stdin
				paramsFile, _ := cmd.Flags().GetString("params")
				params := make(map[string]interface{})

				if paramsFile != "" {
					data, err := os.ReadFile(paramsFile)
					if err != nil {
						return err
					}
					if err := yaml.Unmarshal(data, &params); err != nil {
						return err
					}
				}

				if err := client.installApp(args[0], params); err != nil {
					return err
				}

				fmt.Printf("✓ Application installation started\n")
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall [id]",
			Short: "Uninstall an application",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				keepData, _ := cmd.Flags().GetBool("keep-data")

				if err := client.uninstallApp(args[0], keepData); err != nil {
					return err
				}

				fmt.Printf("✓ Application uninstalled\n")
				return nil
			},
		},
		&cobra.Command{
			Use:   "restart [id]",
			Short: "Restart an application",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				if err := client.restartApp(args[0]); err != nil {
					return err
				}

				fmt.Printf("✓ Application restarted\n")
				return nil
			},
		},
	)

	cmd.Flags().String("params", "", "parameters file (YAML)")
	cmd.Flags().Bool("keep-data", false, "keep application data when uninstalling")

	return cmd
}

// newBackupsCmd creates the backups command group
func newBackupsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backups",
		Short: "Backup management commands",
		Long:  `Commands for managing backups and restoration.`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "run [schedule-id]",
			Short: "Run a backup schedule",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				job, err := client.runBackup(args[0])
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(job)
				} else {
					fmt.Printf("✓ Backup started\n")
					fmt.Printf("  Job ID: %s\n", job.ID)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "restore",
			Short: "Restore from backup",
			RunE: func(cmd *cobra.Command, args []string) error {
				sourceType, _ := cmd.Flags().GetString("source-type")
				sourceID, _ := cmd.Flags().GetString("source-id")
				restoreType, _ := cmd.Flags().GetString("restore-type")
				targetPath, _ := cmd.Flags().GetString("target")

				if sourceType == "" || sourceID == "" || restoreType == "" || targetPath == "" {
					return fmt.Errorf("all flags are required: --source-type, --source-id, --restore-type, --target")
				}

				client := newAPIClient(baseURL, token)

				job, err := client.restore(sourceType, sourceID, restoreType, targetPath)
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(job)
				} else {
					fmt.Printf("✓ Restore started\n")
					fmt.Printf("  Job ID: %s\n", job.ID)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "job-status [job-id]",
			Short: "Check backup job status",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				job, err := client.getBackupJob(args[0])
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(job)
				} else {
					fmt.Printf("Job Status\n")
					fmt.Printf("==========\n")
					fmt.Printf("ID:       %s\n", job.ID)
					fmt.Printf("Type:     %s\n", job.Type)
					fmt.Printf("State:    %s\n", job.State)
					fmt.Printf("Progress: %d%%\n", job.Progress)
					if job.Error != "" {
						fmt.Printf("Error:    %s\n", job.Error)
					}
				}

				return nil
			},
		},
	)

	cmd.Flags().String("source-type", "", "source type (local, ssh, rclone)")
	cmd.Flags().String("source-id", "", "source ID")
	cmd.Flags().String("restore-type", "", "restore type (full, files)")
	cmd.Flags().String("target", "", "target path")

	return cmd
}

// newAlertsCmd creates the alerts command group
func newAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Alert management commands",
		Long:  `Commands for managing alert rules and notifications.`,
	}

	rulesCmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage alert rules",
	}

	rulesCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List alert rules",
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)
				rules, err := client.listAlertRules()
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(rules)
				} else {
					headers := []string{"ID", "Name", "Metric", "Threshold", "Enabled", "Firing"}
					rows := [][]string{}
					for _, rule := range rules {
						enabled := "No"
						if rule.Enabled {
							enabled = "Yes"
						}
						firing := "No"
						if rule.CurrentState.Firing {
							firing = "Yes"
						}
						rows = append(rows, []string{
							rule.ID[:8],
							rule.Name,
							rule.Metric,
							fmt.Sprintf("%s %.1f", rule.Operator, rule.Threshold),
							enabled,
							firing,
						})
					}
					printTable(headers, rows)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "create",
			Short: "Create alert rule",
			RunE: func(cmd *cobra.Command, args []string) error {
				name, _ := cmd.Flags().GetString("name")
				metric, _ := cmd.Flags().GetString("metric")
				operator, _ := cmd.Flags().GetString("operator")
				threshold, _ := cmd.Flags().GetFloat64("threshold")
				duration, _ := cmd.Flags().GetInt("duration")
				severity, _ := cmd.Flags().GetString("severity")

				if name == "" || metric == "" || operator == "" {
					return fmt.Errorf("required flags: --name, --metric, --operator, --threshold")
				}

				client := newAPIClient(baseURL, token)

				rule, err := client.createAlertRule(name, metric, operator, threshold, duration, severity)
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(rule)
				} else {
					fmt.Printf("✓ Alert rule created\n")
					fmt.Printf("  ID: %s\n", rule.ID)
				}

				return nil
			},
		},
	)

	rulesCmd.Flags().String("name", "", "rule name")
	rulesCmd.Flags().String("metric", "", "metric to monitor")
	rulesCmd.Flags().String("operator", ">", "comparison operator")
	rulesCmd.Flags().Float64("threshold", 0, "threshold value")
	rulesCmd.Flags().Int("duration", 60, "duration in seconds")
	rulesCmd.Flags().String("severity", "warning", "severity level")

	cmd.AddCommand(rulesCmd)

	return cmd
}

// newTokensCmd creates the tokens command group
func newTokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "API token management",
		Long:  `Commands for managing API tokens.`,
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List API tokens",
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)
				tokens, err := client.listTokens()
				if err != nil {
					return err
				}

				if outputJSON {
					printJSON(tokens)
				} else {
					headers := []string{"ID", "Name", "Type", "Created", "Last Used"}
					rows := [][]string{}
					for _, t := range tokens {
						lastUsed := "Never"
						if t.LastUsedAt != "" {
							lastUsed = t.LastUsedAt
						}
						rows = append(rows, []string{
							t.ID[:8],
							t.Name,
							t.Type,
							t.CreatedAt,
							lastUsed,
						})
					}
					printTable(headers, rows)
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "create [name]",
			Short: "Create API token",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				scopes, _ := cmd.Flags().GetStringSlice("scopes")
				expires, _ := cmd.Flags().GetString("expires")

				if len(scopes) == 0 {
					return fmt.Errorf("at least one scope is required")
				}

				client := newAPIClient(baseURL, token)

				newToken, tokenValue, err := client.createToken(args[0], scopes, expires)
				if err != nil {
					return err
				}

				if outputJSON {
					// Include token value in JSON output
					output := map[string]interface{}{
						"token": newToken,
						"value": tokenValue,
					}
					printJSON(output)
				} else {
					fmt.Printf("✓ Token created\n")
					fmt.Printf("  ID:    %s\n", newToken.ID)
					fmt.Printf("  Name:  %s\n", newToken.Name)
					fmt.Printf("  Token: %s\n", tokenValue)
					fmt.Printf("\n⚠ Save this token now. You won't be able to see it again.\n")
				}

				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [id]",
			Short: "Delete API token",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				if err := client.deleteToken(args[0]); err != nil {
					return err
				}

				fmt.Printf("✓ Token deleted\n")
				return nil
			},
		},
	)

	cmd.Flags().StringSlice("scopes", []string{}, "token scopes")
	cmd.Flags().String("expires", "", "expiration (e.g., 30d, 1y)")

	return cmd
}

// newOpenapiCmd creates the openapi command
func newOpenapiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "OpenAPI specification commands",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "pull [output-file]",
			Short: "Download OpenAPI specification",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				client := newAPIClient(baseURL, token)

				spec, err := client.getOpenAPISpec()
				if err != nil {
					return err
				}

				var output io.Writer = os.Stdout
				if len(args) > 0 {
					file, err := os.Create(args[0])
					if err != nil {
						return err
					}
					defer file.Close()
					output = file
				}

				if _, err := output.Write(spec); err != nil {
					return err
				}

				if len(args) > 0 {
					fmt.Printf("✓ OpenAPI spec saved to %s\n", args[0])
				}

				return nil
			},
		},
	)

	return cmd
}

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show nosctl version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nosctl version %s\n", Version)
			fmt.Printf("  Build time: %s\n", BuildTime)
			fmt.Printf("  Git commit: %s\n", GitCommit)
		},
	}

	return cmd
}

// newCompletionCmd creates the completion command
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate shell completion script for nosctl.

To load completions:

Bash:
  $ source <(nosctl completion bash)
  # To load completions for each session, execute once:
  $ nosctl completion bash > /etc/bash_completion.d/nosctl

Zsh:
  $ source <(nosctl completion zsh)
  # To load completions for each session, execute once:
  $ nosctl completion zsh > "${fpath[1]}/_nosctl"

Fish:
  $ nosctl completion fish | source
  # To load completions for each session, execute once:
  $ nosctl completion fish > ~/.config/fish/completions/nosctl.fish

PowerShell:
  PS> nosctl completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> nosctl completion powershell > nosctl.ps1
  # and source this file from your PowerShell profile.
`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	return cmd
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
