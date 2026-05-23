# Packages a portable folder into a release zip (Windows CI / local).
param(
  [Parameter(Mandatory = $true)]
  [string]$ZipPath,
  [string]$SourceDir = ""
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if (-not $SourceDir) {
  $SourceDir = Join-Path $root "dist\GetCourseDownloader"
}
$parent = Split-Path -Parent $SourceDir
$folderName = Split-Path -Leaf $SourceDir
if (-not (Test-Path (Join-Path $SourceDir "GetCourseDownloader.exe"))) {
  throw "Missing GetCourseDownloader.exe in: $SourceDir"
}
Copy-Item -Force (Join-Path $root "scripts\RELEASE.txt") (Join-Path $SourceDir "RELEASE.txt")
if (Test-Path $ZipPath) { Remove-Item -Force $ZipPath }
$staging = Join-Path $env:TEMP "gc-release-stage-$(New-Guid)"
New-Item -ItemType Directory -Force -Path (Join-Path $staging $folderName) | Out-Null
Copy-Item -Recurse -Force $SourceDir (Join-Path $staging $folderName)
Compress-Archive -Path (Join-Path $staging $folderName) -DestinationPath $ZipPath
Remove-Item -Recurse -Force $staging
Write-Host "Created: $ZipPath"
