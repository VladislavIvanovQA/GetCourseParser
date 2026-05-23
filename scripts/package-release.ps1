# Packages dist/GetCourseDownloader into a release zip (Windows CI / local).
param(
  [Parameter(Mandatory = $true)]
  [string]$ZipPath
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$src = Join-Path $root "dist\GetCourseDownloader"
if (-not (Test-Path (Join-Path $src "GetCourseDownloader.exe"))) {
  throw "Build first: .\scripts\build.ps1"
}
Copy-Item -Force (Join-Path $root "scripts\RELEASE.txt") (Join-Path $src "RELEASE.txt")
if (Test-Path $ZipPath) { Remove-Item -Force $ZipPath }
Compress-Archive -Path $src -DestinationPath $ZipPath
Write-Host "Created: $ZipPath"
