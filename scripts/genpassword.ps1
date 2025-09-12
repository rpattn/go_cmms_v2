$ErrorActionPreference = "Stop"
Write-Host "Generating Hash ..."
# run from repo root so config.yaml at root is found
$env:SEED_PASSWORD='5Se_w:EXCvW8RU2'; go run .\scripts\genhash.go