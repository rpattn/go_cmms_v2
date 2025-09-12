# scripts/migrate-version.ps1
# Prints current migration version.
# Usage:  .\scripts\migrate-version.ps1

$ErrorActionPreference = "Stop"
if (-not $env:DATABASE_URL) {
  Write-Error "DATABASE_URL is not set. Run:  . ./scripts/env.ps1"
}

$path = Join-Path $PSScriptRoot "..\database\schema"
$pathUnix  = ($path -replace '\\','/')

migrate -path $pathUnix -database $env:DATABASE_URL force 4
# use force N instead of version to fix
