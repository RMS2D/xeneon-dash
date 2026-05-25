# build.ps1 - cross-compile xeneon-dash.exe and produce a dated zip in dist/
# Usage:
#   .\scripts\build.ps1
#   .\scripts\build.ps1 -DropboxPath "C:\Users\dark\Dropbox\xeneon-dash"

param(
    [string]$DropboxPath = ""
)

$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")

$ts = Get-Date -Format "yyyyMMdd"
$distRoot = "dist"
$stage = Join-Path $distRoot "xeneon-dash-$ts"
$zipPath = Join-Path $distRoot "xeneon-dash-$ts.zip"

Write-Host "[1/4] go build (windows/amd64) ..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o xeneon-dash.exe .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

Write-Host "[2/4] staging $stage ..."
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Path $stage -Force | Out-Null

Copy-Item "xeneon-dash.exe" $stage
Copy-Item "scripts\install-task.ps1" $stage
Copy-Item "scripts\update.ps1" $stage
Copy-Item "scripts\launch-panel.ps1" $stage
Copy-Item "scripts\launch-panel.sh" $stage
Copy-Item "scripts\README.txt" $stage
Copy-Item "scripts\config.example.json" $stage

Write-Host "[3/4] zipping to $zipPath ..."
if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
Compress-Archive -Path "$stage\*" -DestinationPath $zipPath

$size = [math]::Round((Get-Item $zipPath).Length / 1MB, 2)
Write-Host "[4/4] done. $zipPath ($size MB)"

if ($DropboxPath) {
    if (-not (Test-Path $DropboxPath)) {
        New-Item -ItemType Directory -Path $DropboxPath -Force | Out-Null
    }
    Copy-Item -Path $zipPath -Destination $DropboxPath -Force
    Write-Host "Copied to $DropboxPath"
}
