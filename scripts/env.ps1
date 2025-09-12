# scripts/env.ps1
# Central place to set env vars used by scripts (migrations). Adjust to your DB credentials.
# Source it in the same PowerShell session:  . ./scripts/env.ps1

# Example local DB:
$env:DATABASE_URL = "postgres://postgres:admin@localhost:5432/db?sslmode=disable"

# Optional: set BASE_URL etc. (server reads config.yaml + env overrides)
# $env:BASE_URL = "http://localhost:8080"
