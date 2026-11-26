package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"nos/installer/internal/installer"
)

var (
	version = "1.0.0"
	commit  = "unknown"
)

func main() {
	log.SetOutput(os.Stdout)

	var rootCmd = &cobra.Command{
		Use:   "nos-installer",
		Short: "nOS guided installer",
		Long:  `nOS installer creates a fresh installation with Btrfs subvolumes and proper system configuration.`,
		Run: func(cmd *cobra.Command, args []string) {
			runInstaller()
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("nos-installer %s (commit: %s)\n", version, commit)
		},
	}

	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runInstaller() {
	// Ensure we're running as root
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: installer must be run as root\n")
		os.Exit(1)
	}

	// Create and run the installer
	inst := installer.New()
	if err := inst.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✓ Installation completed successfully!")
	fmt.Println("Please remove installation media and reboot.")
}
