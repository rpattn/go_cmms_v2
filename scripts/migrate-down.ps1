# scripts/migrate-down.ps1
param(
  [Parameter(Mandatory = $false)]
  [int] $Steps,                      # e.g. -Steps 2  -> down 2

  [Parameter(Mandatory = $false)]
  [int] $ToVersion,                  # e.g. -ToVersion 20250906123456 -> goto version

  [switch] $All                      # e.g. -All -> down to 0
)

$ErrorActionPreference = "Stop"

if (-not $env:DATABASE_URL) {
  Write-Error "DATABASE_URL is not set. Run:  . ./scripts/env.ps1"
}

# Resolve to absolute path, then convert to forward slashes
$rawPath  = Resolve-Path (Join-Path $PSScriptRoot "..\database\schema")
$pathUnix = ($rawPath.Path -replace '\\','/')

function Invoke-Migrate {
  param([string]$cmd, [string]$arg = $null)

  $args = @("-path", "$pathUnix", "-database", "$env:DATABASE_URL", $cmd)
  if ($null -ne $arg -and $arg -ne "") { $args += $arg }

  Write-Host "migrate $($args -join ' ')"
  & migrate @args
  if ($LASTEXITCODE -ne 0) {
    throw "migrate exited with code $LASTEXITCODE"
  }
}

# Validate mutually exclusive options
$provided = @()
if ($PSBoundParameters.ContainsKey('Steps'))     { $provided += 'Steps' }
if ($PSBoundParameters.ContainsKey('ToVersion')) { $provided += 'ToVersion' }
if ($All.IsPresent)                              { $provided += 'All' }

if ($provided.Count -eq 0) {
  Write-Error "Provide exactly one of: -Steps <N>, -ToVersion <version>, or -All."
}

if ($provided.Count -gt 1) {
  Write-Error "Options are mutually exclusive. Use only one of: -Steps, -ToVersion, -All."
}

# Execute the chosen mode
if ($PSBoundParameters.ContainsKey('ToVersion')) {
  if ($ToVersion -lt 0) { Write-Error "-ToVersion must be a non-negative integer." }
  Write-Host "Migrating to version $ToVersion from $pathUnix"
  Invoke-Migrate "goto" "$ToVersion"
}
elseif ($PSBoundParameters.ContainsKey('Steps')) {
  if ($Steps -lt 1) { Write-Error "-Steps must be >= 1." }
  Write-Host "Rolling back $Steps step(s) from $pathUnix"
  Invoke-Migrate "down" "$Steps"
}
elseif ($All.IsPresent) {
  Write-Host "Rolling back ALL migrations to version 0 from $pathUnix"
  Invoke-Migrate "down"
}


# Down 2 migrations
# .\scripts\migrate-down.ps1 -Steps 2

# Go directly to version 20250906120000 (up or down as needed)
# .\scripts\migrate-down.ps1 -ToVersion 20250906120000

# Roll back everything to version 0
# .\scripts\migrate-down.ps1 -All