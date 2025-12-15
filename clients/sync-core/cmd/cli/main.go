// Package main provides a CLI for NithronSync.
package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"nithronos/clients/sync-core/api"
	"nithronos/clients/sync-core/config"
	"nithronos/clients/sync-core/db"
)

var (
	Version = "1.0.0"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "status":
		cmdStatus()
	case "config":
		cmdConfig(args)
	case "shares":
		cmdShares()
	case "devices":
		cmdDevices()
	case "activity":
		cmdActivity()
	case "sync":
		cmdSync()
	case "pause":
		cmdPause()
	case "resume":
		cmdResume()
	case "version":
		fmt.Printf("NithronSync CLI v%s\n", Version)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`NithronSync CLI

Usage: nithron-sync-cli <command> [arguments]

Commands:
  status      Show sync status
  config      Show or update configuration
  shares      List sync-enabled shares
  devices     List registered devices
  activity    Show recent activity
  sync        Trigger immediate sync
  pause       Pause synchronization
  resume      Resume synchronization
  version     Show version
  help        Show this help

Configuration:
  nithron-sync-cli config                    Show current config
  nithron-sync-cli config server <url>       Set server URL
  nithron-sync-cli config token <token>      Set device token
  nithron-sync-cli config folder <path>      Set sync folder

Examples:
  nithron-sync-cli config server https://nas.local
  nithron-sync-cli config token nos_dt_...
  nithron-sync-cli status`)
}

func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %s\n", err)
		os.Exit(1)
	}
	return cfg
}

func cmdStatus() {
	cfg := loadConfig()

	if !cfg.IsConfigured() {
		fmt.Println("NithronSync is not configured.")
		fmt.Println("Run 'nithron-sync-cli config server <url>' and 'nithron-sync-cli config token <token>' to set up.")
		return
	}

	// Get stats from database
	database, err := db.Open()
	if err != nil {
		fmt.Printf("Error opening database: %s\n", err)
		return
	}
	defer database.Close()

	fmt.Println("NithronSync Status")
	fmt.Println("==================")
	fmt.Printf("Server:      %s\n", cfg.ServerURL)
	fmt.Printf("Sync Folder: %s\n", cfg.SyncFolder)
	fmt.Printf("Enabled:     %t\n", cfg.SyncEnabled)
	fmt.Println()

	// Check server connectivity
	client := api.NewClient(cfg)
	if err := client.HealthCheck(context.Background()); err != nil {
		fmt.Printf("Server Status: ❌ Unreachable (%s)\n", err)
	} else {
		fmt.Println("Server Status: ✅ Connected")
	}
}

func cmdConfig(args []string) {
	cfg := loadConfig()

	if len(args) == 0 {
		// Show config
		fmt.Println("Current Configuration")
		fmt.Println("=====================")
		fmt.Printf("Server URL:       %s\n", cfg.ServerURL)
		fmt.Printf("Sync Folder:      %s\n", cfg.SyncFolder)
		fmt.Printf("Device ID:        %s\n", cfg.DeviceID)
		fmt.Printf("Sync Enabled:     %t\n", cfg.SyncEnabled)
		fmt.Printf("Sync on Metered:  %t\n", cfg.SyncOnMetered)
		fmt.Printf("Bandwidth Limit:  %d KB/s\n", cfg.BandwidthLimitKBps)
		fmt.Printf("Poll Interval:    %d seconds\n", cfg.PollIntervalSecs)
		fmt.Printf("Conflict Policy:  %s\n", cfg.ConflictPolicy)
		fmt.Printf("Debug Logging:    %t\n", cfg.DebugLogging)
		return
	}

	if len(args) < 2 {
		fmt.Println("Usage: nithron-sync-cli config <key> <value>")
		return
	}

	key := args[0]
	value := args[1]

	err := cfg.Update(func(c *config.Config) {
		switch key {
		case "server":
			c.ServerURL = value
		case "token":
			c.DeviceToken = value
		case "folder":
			c.SyncFolder = value
		case "enabled":
			c.SyncEnabled = value == "true" || value == "1" || value == "yes"
		default:
			fmt.Printf("Unknown config key: %s\n", key)
			os.Exit(1)
		}
	})

	if err != nil {
		fmt.Printf("Error saving config: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config updated: %s = %s\n", key, value)
}

func cmdShares() {
	cfg := loadConfig()

	if !cfg.IsConfigured() {
		fmt.Println("NithronSync is not configured.")
		return
	}

	client := api.NewClient(cfg)
	shares, err := client.ListShares(context.Background())
	if err != nil {
		fmt.Printf("Error listing shares: %s\n", err)
		return
	}

	if len(shares) == 0 {
		fmt.Println("No sync-enabled shares found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPATH\tSYNC ENABLED")
	fmt.Fprintln(w, "--\t----\t----\t------------")
	for _, share := range shares {
		enabled := "No"
		if share.SyncEnabled {
			enabled = "Yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", share.ID, share.Name, share.Path, enabled)
	}
	w.Flush()
}

func cmdDevices() {
	cfg := loadConfig()

	if !cfg.IsConfigured() {
		fmt.Println("NithronSync is not configured.")
		return
	}

	// This would require an additional API endpoint to list devices
	// For now, just show the current device info
	fmt.Println("Current Device")
	fmt.Println("==============")
	fmt.Printf("Device ID: %s\n", cfg.DeviceID)
}

func cmdActivity() {
	database, err := db.Open()
	if err != nil {
		fmt.Printf("Error opening database: %s\n", err)
		return
	}
	defer database.Close()

	activities, err := database.GetRecentActivity(20)
	if err != nil {
		fmt.Printf("Error getting activity: %s\n", err)
		return
	}

	if len(activities) == 0 {
		fmt.Println("No recent activity.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tACTION\tSTATUS\tPATH")
	fmt.Fprintln(w, "----\t------\t------\t----")
	for _, a := range activities {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			a.CreatedAt.Format("15:04:05"),
			a.Action,
			a.Status,
			a.Path,
		)
	}
	w.Flush()
}

func cmdSync() {
	fmt.Println("Triggering sync...")
	// This would send a signal to the daemon to sync immediately
	// For now, just print a message
	fmt.Println("Use 'nithron-sync-daemon' to run the sync service.")
}

func cmdPause() {
	fmt.Println("Pausing sync...")
	// This would send a signal to the daemon to pause
	fmt.Println("Use 'nithron-sync-daemon' to control the sync service.")
}

func cmdResume() {
	fmt.Println("Resuming sync...")
	// This would send a signal to the daemon to resume
	fmt.Println("Use 'nithron-sync-daemon' to control the sync service.")
}

