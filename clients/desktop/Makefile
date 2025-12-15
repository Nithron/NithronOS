.PHONY: all build build-windows build-linux build-darwin dev clean install-deps

# Version
VERSION := 1.0.0
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.Commit=$(COMMIT)"

all: build

install-deps:
	@echo "Installing Wails..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
	@echo "Installing frontend dependencies..."
	cd frontend && npm install

dev:
	wails dev

build:
	wails build

build-windows:
	@echo "Building for Windows..."
	wails build -platform windows/amd64 -o NithronSync.exe
	@echo "Windows build complete: build/bin/NithronSync.exe"

build-linux:
	@echo "Building for Linux..."
	wails build -platform linux/amd64 -o NithronSync
	@echo "Linux build complete: build/bin/NithronSync"

build-darwin:
	@echo "Building for macOS..."
	wails build -platform darwin/universal -o NithronSync.app
	@echo "macOS build complete: build/bin/NithronSync.app"

build-all: build-windows build-linux build-darwin

# Create distribution packages
dist-windows: build-windows
	@echo "Creating Windows installer..."
	@mkdir -p dist
	# Use NSIS or similar to create installer
	# makensis installer/windows/installer.nsi

dist-linux: build-linux
	@echo "Creating Linux packages..."
	@mkdir -p dist
	# Create AppImage
	# Create .deb package

dist-darwin: build-darwin
	@echo "Creating macOS DMG..."
	@mkdir -p dist
	# Create DMG
	# create-dmg --volname "NithronSync" ...

clean:
	rm -rf build/bin
	rm -rf dist
	rm -rf frontend/dist
	rm -rf frontend/node_modules

