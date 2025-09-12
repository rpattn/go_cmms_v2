# scripts/run.ps1
# Runs the Go server from cmd/server.
# Usage:  .\scripts\run.ps1
# Tip: ensure config.yaml exists at repo root (or use env overrides).

$ErrorActionPreference = "Stop"
Write-Host "Starting server ..."
# run from repo root so config.yaml at root is found
go run ./cmd/server
