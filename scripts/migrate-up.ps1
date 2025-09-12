# scripts/migrate-up.ps1
$ErrorActionPreference = "Stop"
if (-not $env:DATABASE_URL) { Write-Error "DATABASE_URL is not set. Run:  . ./scripts/env.ps1" }

# Resolve to absolute path, then convert to forward slashes
$rawPath   = Resolve-Path (Join-Path $PSScriptRoot "..\database\schema")
$pathUnix  = ($rawPath.Path -replace '\\','/')

Write-Host "Running migrations UP from $pathUnix"
migrate -path "$pathUnix" -database "$env:DATABASE_URL" up
