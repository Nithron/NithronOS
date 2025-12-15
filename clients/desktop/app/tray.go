// Package app provides system tray functionality.
package app

import (
	"github.com/getlantern/systray"
)

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
	systray.SetIcon(trayIcon)
	systray.SetTitle("NithronSync")
	systray.SetTooltip("NithronSync - File Synchronization")

	// Menu items
	mOpen := systray.AddMenuItem("Open NithronSync", "Open the main window")
	systray.AddSeparator()
	
	mOpenFolder := systray.AddMenuItem("Open Sync Folder", "Open the sync folder in file manager")
	mOpenWeb := systray.AddMenuItem("Open Web UI", "Open NithronOS in browser")
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
				t.app.OpenSyncFolder()

			case <-mOpenWeb.ClickedCh:
				t.app.OpenWebUI()

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

// trayIcon is a placeholder - will be replaced with actual icon data
var trayIcon = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
	0x01, 0x73, 0x52, 0x47, 0x42, 0x00, 0xAE, 0xCE, 0x1C, 0xE9, 0x00, 0x00,
	0x00, 0x89, 0x49, 0x44, 0x41, 0x54, 0x38, 0x8D, 0x63, 0x64, 0xC0, 0x00,
	0x8C, 0x0C, 0x0C, 0x0C, 0xFF, 0x61, 0x18, 0x24, 0x00, 0x04, 0x10, 0x13,
	0x0C, 0x10, 0x00, 0x01, 0xC4, 0x04, 0x03, 0x04, 0x40, 0x00, 0x31, 0xC1,
	0x00, 0x01, 0x10, 0x40, 0x4C, 0x30, 0x40, 0x00, 0x04, 0x10, 0x13, 0x0C,
	0x10, 0x00, 0x01, 0xC4, 0x04, 0x03, 0x04, 0x40, 0x00, 0x31, 0xC1, 0x00,
	0x01, 0x10, 0x40, 0x4C, 0x30, 0x40, 0x00, 0x04, 0x10, 0x13, 0x0C, 0x10,
	0x00, 0x01, 0xC4, 0x04, 0x03, 0x04, 0x40, 0x00, 0x31, 0xC1, 0x00, 0x01,
	0x10, 0x40, 0x4C, 0x30, 0x40, 0x00, 0x04, 0x10, 0x13, 0x0C, 0x10, 0x00,
	0x01, 0xC4, 0x04, 0x03, 0x04, 0x40, 0x00, 0x31, 0xC1, 0x00, 0x01, 0x10,
	0x40, 0x4C, 0x30, 0x00, 0x00, 0x00, 0xD5, 0x0F, 0x21, 0x5C, 0xD7, 0xDA,
	0x04, 0x3D, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42,
	0x60, 0x82,
}

