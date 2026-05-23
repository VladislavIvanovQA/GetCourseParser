# Downloads portable Go into tools/go (for build without system install).
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$goDir = Join-Path $root "tools\go"
$zip = Join-Path $root "tools\go.zip"
$ver = "go1.22.10.windows-amd64.zip"

if (Test-Path (Join-Path $goDir "bin\go.exe")) {
  & (Join-Path $goDir "bin\go.exe") version
  exit 0
}

New-Item -ItemType Directory -Force -Path (Join-Path $root "tools") | Out-Null
Write-Host "Downloading $ver ..."
Invoke-WebRequest -Uri "https://go.dev/dl/$ver" -OutFile $zip -UseBasicParsing
Write-Host "Extracting (may take a minute)..."
tar -xf $zip -C (Join-Path $root "tools")
$extracted = Join-Path $root "tools\go"
if (-not (Test-Path (Join-Path $extracted "bin\go.exe"))) {
  throw "Extract failed: go.exe not found"
}
& (Join-Path $extracted "bin\go.exe") version
