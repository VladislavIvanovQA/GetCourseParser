# Build portable GetCourseDownloader.exe for Windows amd64.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$desktop = Join-Path $root "desktop"
$dist = Join-Path $root "dist"
$portable = Join-Path $dist "GetCourseDownloader"

New-Item -ItemType Directory -Force -Path $portable | Out-Null

$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $go) {
  $localGo = Join-Path $root "tools\go\bin\go.exe"
  if (Test-Path $localGo) { $go = @{ Source = $localGo } }
  else {
    Write-Host "Go not found. Run: .\scripts\setup-go.ps1"
    exit 1
  }
} else {
  $localGo = $go.Source
}

Push-Location $desktop
& $localGo build -ldflags "-s -w" -o (Join-Path $portable "GetCourseDownloader.exe") .
if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
Pop-Location

Copy-Item -Force (Join-Path $root "extension") (Join-Path $portable "extension") -Recurse
New-Item -ItemType Directory -Force -Path (Join-Path $portable "downloads") | Out-Null

if (-not (Test-Path (Join-Path $portable "ffmpeg.exe"))) {
  Write-Host ""
  Write-Host "ffmpeg missing — run: .\scripts\setup-ffmpeg.ps1"
}

Write-Host ""
Write-Host "Built: $portable\GetCourseDownloader.exe"
Write-Host "Chrome: Load unpacked -> $portable\extension"
