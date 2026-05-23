# Downloads ffmpeg.exe into portable GetCourseDownloader folder.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$targetDir = Join-Path $root "dist\GetCourseDownloader"
$ffmpegExe = Join-Path $targetDir "ffmpeg.exe"

if (Test-Path $ffmpegExe) {
  Write-Host "Already exists: $ffmpegExe"
  & $ffmpegExe -version | Select-Object -First 1
  exit 0
}

New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
$zip = Join-Path $env:TEMP "ffmpeg-essentials-win64.zip"
$url = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"

Write-Host "Downloading ffmpeg (~90 MB)..."
Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing

Write-Host "Extracting..."
$extractDir = Join-Path $env:TEMP "ffmpeg-essentials-extract"
if (Test-Path $extractDir) { Remove-Item -Recurse -Force $extractDir }
Expand-Archive -Path $zip -DestinationPath $extractDir -Force

$found = Get-ChildItem -Path $extractDir -Filter "ffmpeg.exe" -Recurse | Select-Object -First 1
if (-not $found) {
  throw "ffmpeg.exe not found inside archive"
}

Copy-Item -Force $found.FullName $ffmpegExe
Remove-Item -Force $zip -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force $extractDir -ErrorAction SilentlyContinue

Write-Host "Installed: $ffmpegExe"
& $ffmpegExe -version | Select-Object -First 1
