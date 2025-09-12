# scripts/sqlc-generate.ps1
# Generates Go code into internal/db/gen using db/sqlc.yaml.
# Usage:  .\scripts\sqlc-generate.ps1

$ErrorActionPreference = "Stop"
Push-Location (Join-Path $PSScriptRoot "..\database")
try {
  sqlc generate
} finally {
  Pop-Location
}
