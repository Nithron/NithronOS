package main

import (
	"embed"
	"os"
	"runtime"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"nithronos/clients/desktop/app"
	"nithronos/clients/sync-core/config"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Set up logging
	logLevel := zerolog.InfoLevel
	if os.Getenv("NITHRON_DEBUG") != "" {
		logLevel = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	// Set up log file
	logDir, err := config.GetLogDir()
	if err == nil {
		logFile, err := os.OpenFile(logDir+"/nithron-sync.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			multi := zerolog.MultiLevelWriter(os.Stderr, logFile)
			log.Logger = zerolog.New(multi).With().Timestamp().Logger()
		}
	}

	// Create app
	application := app.New(log.Logger)

	// Create application with options
	err = wails.Run(&options.App{
		Title:             "NithronSync",
		Width:             1024,
		Height:            768,
		MinWidth:          400,
		MinHeight:         300,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:             nil,
		Logger:           nil,
		LogLevel:         0,
		OnStartup:        application.Startup,
		OnDomReady:       application.DomReady,
		OnBeforeClose:    application.BeforeClose,
		OnShutdown:       application.Shutdown,
		WindowStartState: options.Normal,
		Bind: []interface{}{
			application,
		},
		// Windows specific options
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: false,
			WebviewUserDataPath:               "",
			ZoomFactor:                        1.0,
		},
		// Mac specific options
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameAqua,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "NithronSync",
				Message: "File synchronization for NithronOS\n\nVersion 1.0.0",
				Icon:    icon,
			},
		},
		// Linux specific options
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: false,
		},
	})

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start application")
	}
}

func init() {
	// Set GOMAXPROCS
	if runtime.GOMAXPROCS(0) < 2 {
		runtime.GOMAXPROCS(2)
	}
}

