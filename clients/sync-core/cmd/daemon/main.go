// Package main provides a headless daemon for NithronSync.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"nithronos/clients/sync-core/config"
	"nithronos/clients/sync-core/engine"
)

var (
	Version   = "1.0.0"
	BuildTime = "unknown"
	Commit    = "unknown"
)

func main() {
	// Parse flags
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		debugMode   = flag.Bool("debug", false, "Enable debug logging")
		configPath  = flag.String("config", "", "Path to configuration file")
		serverURL   = flag.String("server", "", "Server URL (for initial setup)")
		deviceToken = flag.String("token", "", "Device token (for initial setup)")
		syncFolder  = flag.String("folder", "", "Sync folder path (for initial setup)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("NithronSync Daemon v%s\n", Version)
		fmt.Printf("Build time: %s\n", BuildTime)
		fmt.Printf("Commit: %s\n", Commit)
		os.Exit(0)
	}

	// Set up logging
	logLevel := zerolog.InfoLevel
	if *debugMode {
		logLevel = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Set up log output
	logDir, err := config.GetLogDir()
	if err == nil {
		logFile, err := os.OpenFile(
			logDir+"/nithron-sync-daemon.log",
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err == nil {
			multi := zerolog.MultiLevelWriter(os.Stderr, logFile)
			log.Logger = zerolog.New(multi).With().Timestamp().Logger()
		}
	}

	// Load or create configuration
	var cfg *config.Config
	if *configPath != "" {
		cfg, err = config.LoadFrom(*configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Handle initial setup via command line
	if *serverURL != "" && *deviceToken != "" {
		log.Info().Msg("Setting up from command line arguments...")
		
		folder := *syncFolder
		if folder == "" {
			folder, _ = config.GetDefaultSyncFolder()
		}

		if err := cfg.Update(func(c *config.Config) {
			c.ServerURL = *serverURL
			c.DeviceToken = *deviceToken
			c.SyncFolder = folder
			c.SyncEnabled = true
		}); err != nil {
			log.Fatal().Err(err).Msg("Failed to save configuration")
		}

		if err := os.MkdirAll(folder, 0755); err != nil {
			log.Fatal().Err(err).Msg("Failed to create sync folder")
		}

		log.Info().Str("folder", folder).Msg("Configuration saved")
	}

	// Check if configured
	if !cfg.IsConfigured() {
		log.Error().Msg("NithronSync is not configured")
		log.Info().Msg("Run with --server and --token to set up, or use the desktop app")
		os.Exit(1)
	}

	// Create sync engine
	eng, err := engine.New(cfg, log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create sync engine")
	}

	// Set up callbacks
	eng.SetCallbacks(
		func(state engine.State) {
			log.Info().Str("state", state.String()).Msg("State changed")
		},
		func(progress engine.Progress) {
			if progress.CurrentFile != "" {
				log.Debug().
					Str("file", progress.CurrentFile).
					Int64("pending_up", progress.PendingUploads).
					Int64("pending_down", progress.PendingDownloads).
					Msg("Progress")
			}
		},
		func(err error) {
			log.Error().Err(err).Msg("Sync error")
		},
		func(conflict engine.Conflict) {
			log.Warn().
				Str("share", conflict.ShareID).
				Str("path", conflict.Path).
				Msg("Conflict detected")
		},
	)

	// Start engine
	log.Info().
		Str("server", cfg.ServerURL).
		Str("folder", cfg.SyncFolder).
		Msg("Starting NithronSync daemon")

	if err := eng.Start(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start sync engine")
	}

	// Wait for shutdown signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Received shutdown signal")
		cancel()
	}()

	<-ctx.Done()

	// Graceful shutdown
	log.Info().Msg("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		eng.Stop()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Shutdown complete")
	case <-shutdownCtx.Done():
		log.Warn().Msg("Shutdown timed out")
	}
}

