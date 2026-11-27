//go:build !windows

package app

// No tray outside Windows (see tray_windows.go); closing the window quits.
type TrayManager struct{}

// NewTrayManager returns nil: there is no tray here.
func NewTrayManager(*App) *TrayManager { return nil }

// Run does nothing.
func (*TrayManager) Run() {}

// Stop does nothing.
func (*TrayManager) Stop() {}

// HasTray says whether closing the window can hide it to a tray.
const HasTray = false
