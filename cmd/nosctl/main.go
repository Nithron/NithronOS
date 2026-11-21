package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version info (set by build)
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"

	// Global flags
	cfgFile    string
	baseURL    string
	token      string
	outputJSON bool
	verbose    bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "nosctl",
	Short: "NithronOS command-line interface",
	Long: `nosctl is the command-line interface for NithronOS.

It allows you to manage your NithronOS system from the terminal,
including storage, applications, backups, and more.`,
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/nos/cli.yaml)")
	rootCmd.PersistentFlags().StringVar(&baseURL, "url", "", "NithronOS API URL")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "API token")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	_ = viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))

	// Add commands
	rootCmd.AddCommand(
		newLoginCmd(),
		newStatusCmd(),
		newSystemCmd(),
		newStorageCmd(),
		newAppsCmd(),
		newBackupsCmd(),
		newAlertsCmd(),
		newTokensCmd(),
		newOpenapiCmd(),
		newVersionCmd(),
		newCompletionCmd(),
	)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in default locations
		viper.SetConfigName("cli")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("$HOME/.config/nos")
		viper.AddConfigPath(".")
	}

	// Environment variables
	viper.SetEnvPrefix("NOS")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err == nil && verbose {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}

	// Set defaults
	if baseURL == "" {
		baseURL = viper.GetString("url")
		if baseURL == "" {
			baseURL = "http://localhost:9000"
		}
	}

	if token == "" {
		token = viper.GetString("token")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Helper functions

func printJSON(data interface{}) {
	// Implementation would use json.Marshal
	fmt.Printf("%+v\n", data)
}

func printTable(headers []string, rows [][]string) {
	// Implementation would use a table printer library
	// For now, simple output
	for _, header := range headers {
		fmt.Printf("%-20s", header)
	}
	fmt.Println()

	for _, row := range rows {
		for _, col := range row {
			fmt.Printf("%-20s", col)
		}
		fmt.Println()
	}
}
