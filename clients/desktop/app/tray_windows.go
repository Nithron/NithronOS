//go:build windows

// The tray icon, on Windows only: getlantern/systray runs its own UI loop,
// which works beside Wails' on Windows but not on macOS (both need the main
// thread) or Linux (two GTK main loops in different threads).
package app

import (
	"encoding/binary"

	"github.com/getlantern/systray"
)

// HasTray says whether closing the window can hide it to a tray.
const HasTray = true

// TrayManager manages the system tray icon.
type TrayManager struct {
	app      *App
	quitChan chan struct{}
}

// NewTrayManager creates a new tray manager.
func NewTrayManager(app *App) *TrayManager {
	return &TrayManager{
		app:      app,
		quitChan: make(chan struct{}),
	}
}

// Run starts the system tray.
func (t *TrayManager) Run() {
	systray.Run(t.onReady, t.onExit)
}

// Stop stops the system tray.
func (t *TrayManager) Stop() {
	close(t.quitChan)
	systray.Quit()
}

func (t *TrayManager) onReady() {
	// Set icon (using embedded icon from main.go)
	systray.SetIcon(pngICO(Icon))
	systray.SetTitle("NithronSync")
	systray.SetTooltip("NithronSync - File Synchronization")

	// Menu items
	mOpen := systray.AddMenuItem("Open NithronSync", "Open the main window")
	systray.AddSeparator()

	mOpenFolder := systray.AddMenuItem("Open Sync Folder", "Open the sync folder in file manager")
	mOpenWeb := systray.AddMenuItem("Open Web UI", "Open nOS in browser")
	systray.AddSeparator()

	mStatus := systray.AddMenuItem("Status: Idle", "Current sync status")
	mStatus.Disable()
	systray.AddSeparator()

	mPause := systray.AddMenuItem("Pause Sync", "Pause synchronization")
	mResume := systray.AddMenuItem("Resume Sync", "Resume synchronization")
	mResume.Hide()
	mSyncNow := systray.AddMenuItem("Sync Now", "Trigger immediate sync")
	systray.AddSeparator()

	mSettings := systray.AddMenuItem("Settings", "Open settings")
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit NithronSync")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-t.quitChan:
				return

			case <-mOpen.ClickedCh:
				t.app.ShowWindow()

			case <-mOpenFolder.ClickedCh:
				if err := t.app.OpenSyncFolder(); err != nil {
					t.app.logger.Error().Err(err).Msg("Open sync folder")
				}

			case <-mOpenWeb.ClickedCh:
				if err := t.app.OpenWebUI(); err != nil {
					t.app.logger.Error().Err(err).Msg("Open web UI")
				}

			case <-mPause.ClickedCh:
				t.app.PauseSync()
				mPause.Hide()
				mResume.Show()
				mStatus.SetTitle("Status: Paused")
				systray.SetTooltip("NithronSync - Paused")

			case <-mResume.ClickedCh:
				t.app.ResumeSync()
				mResume.Hide()
				mPause.Show()
				mStatus.SetTitle("Status: Syncing")
				systray.SetTooltip("NithronSync - Syncing")

			case <-mSyncNow.ClickedCh:
				t.app.SyncNow()
				mStatus.SetTitle("Status: Syncing...")

			case <-mSettings.ClickedCh:
				t.app.ShowWindow()
				// TODO: Navigate to settings

			case <-mQuit.ClickedCh:
				t.app.Quit()
				return
			}
		}
	}()
}

func (t *TrayManager) onExit() {
	// Cleanup
}

// UpdateStatus updates the tray status.
func (t *TrayManager) UpdateStatus(status string) {
	systray.SetTooltip("NithronSync - " + status)
}

// pngICO wraps a PNG in an ICO container (one image; Windows Vista and
// later read PNG-compressed icons), which is what the Windows tray takes.
// (The tray was given a PNG placeholder, which Windows doesn't show.)
func pngICO(png []byte) []byte {
	if len(png) < 24 {
		return nil
	}
	dim := func(b []byte) byte { // 256 and up are written as 0
		v := binary.BigEndian.Uint32(b)
		if v >= 256 {
			return 0
		}
		return byte(v)
	}
	ico := []byte{0, 0, 1, 0, 1, 0, // reserved, type icon, one image
		dim(png[16:20]), dim(png[20:24]), 0, 0, // width, height (from IHDR), colors, reserved
		1, 0, 32, 0} // planes, bits per pixel
	ico = binary.LittleEndian.AppendUint32(ico, uint32(len(png)))
	ico = binary.LittleEndian.AppendUint32(ico, 6+16)
	return append(ico, png...)
}
