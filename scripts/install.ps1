# scripts/tools-install.ps1
# Installs sqlc and migrate via `go install` and ensures %USERPROFILE%\go\bin is on PATH.

param([switch]$Force)

$ErrorActionPreference = "Stop"

function Ensure-Go {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go is not installed or not on PATH. Install Go from https://go.dev/dl/ and re-run."
  }
}

function GoInstall($pkg) {
  Write-Host "Installing $pkg ..."
  & go install $pkg
}

Ensure-Go

# Install/Update to latest
GoInstall "github.com/sqlc-dev/sqlc/cmd/sqlc@latest"
GoInstall "github.com/golang-migrate/migrate/v4/cmd/migrate@latest"

# Ensure PATH contains %USERPROFILE%\go\bin
$goBin = "$env:USERPROFILE\go\bin"
if ($env:Path -notlike "*$goBin*") {
  Write-Host "Adding $goBin to user PATH"
  [Environment]::SetEnvironmentVariable("Path", $env:Path + ";$goBin", "User")
  Write-Host "PATH updated. Open a NEW PowerShell window to pick it up."
} else {
  Write-Host "Go bin already on PATH."
}

Write-Host "`nInstalled versions:"
try { sqlc version } catch { Write-Host "sqlc not found on PATH (open a new shell?)" }
try { migrate -version } catch { Write-Host "migrate not found on PATH (open a new shell?)" }

# Build the CLI with Postgres DB driver + file source driver
$env:CGO_ENABLED = "0"
go install -tags "postgres,file" github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Make sure %USERPROFILE%\go\bin is on PATH in this session
$goBin = "$env:USERPROFILE\go\bin"
if ($env:Path -notlike "*$goBin*") { $env:Path = "$env:Path;$goBin" }

# Verify
migrate -help | Select-String postgres

Write-Host "`nDone."