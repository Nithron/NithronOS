# Build script for Windows
# Requires: Go, Node.js, Wails CLI

$ErrorActionPreference = "Stop"

Write-Host "Building NithronSync for Windows..." -ForegroundColor Cyan

# Check prerequisites
if (!(Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed" -ForegroundColor Red
    exit 1
}

if (!(Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Node.js is not installed" -ForegroundColor Red
    exit 1
}

if (!(Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "Installing Wails CLI..." -ForegroundColor Yellow
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
}

# Install frontend dependencies
Write-Host "Installing frontend dependencies..." -ForegroundColor Yellow
Push-Location frontend
npm install
Pop-Location

# Build
Write-Host "Building application..." -ForegroundColor Yellow
wails build -platform windows/amd64 -o NithronSync.exe

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful!" -ForegroundColor Green
    Write-Host "Output: build\bin\NithronSync.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# Create installer (optional, requires NSIS)
if (Get-Command makensis -ErrorAction SilentlyContinue) {
    Write-Host "Creating installer..." -ForegroundColor Yellow
    # makensis installer/windows/installer.nsi
}

