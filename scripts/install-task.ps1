# install-task.ps1 - first-time install on the old PC.
# Right-click - Run with PowerShell (as Administrator).
# Registers two scheduled tasks (server + kiosk browser) that fire at logon.

$ErrorActionPreference = "Stop"

$installDir = "C:\xeneon-dash"
$exePath = Join-Path $installDir "xeneon-dash.exe"
$logsDir = Join-Path $installDir "logs"
$configPath = Join-Path $installDir "config.json"
$configExample = Join-Path $installDir "config.example.json"

# Verify install location
if (-not (Test-Path $exePath)) {
    throw "xeneon-dash.exe not found at $exePath. Extract the zip to C:\xeneon-dash\ first."
}
if (-not (Test-Path $logsDir)) {
    New-Item -ItemType Directory -Path $logsDir | Out-Null
}
if ((-not (Test-Path $configPath)) -and (Test-Path $configExample)) {
    Copy-Item $configExample $configPath
    Write-Host "Created config.json from config.example.json"
}

# Locate Edge
$edgePath = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
if (-not (Test-Path $edgePath)) {
    $edgePath = "${env:ProgramFiles}\Microsoft\Edge\Application\msedge.exe"
}
if (-not (Test-Path $edgePath)) {
    throw "Microsoft Edge not found. Install Edge or edit this script to point at your kiosk browser."
}

$user = "$env:COMPUTERNAME\$env:USERNAME"

# Stop + delete any existing tasks (idempotent)
foreach ($name in @("XeneonDash", "XeneonDashKiosk")) {
    schtasks /End /TN $name 2>$null | Out-Null
    schtasks /Delete /TN $name /F 2>$null | Out-Null
}

# Server task - run at logon
$svrAction = New-ScheduledTaskAction -Execute $exePath -WorkingDirectory $installDir
$svrTrigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -ExecutionTimeLimit (New-TimeSpan -Days 0) `
    -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited

Register-ScheduledTask `
    -TaskName "XeneonDash" `
    -Action $svrAction `
    -Trigger $svrTrigger `
    -Settings $settings `
    -Principal $principal `
    -Description "xeneon-dash HTTP server (Corsair Xeneon kiosk)" `
    -Force | Out-Null

# Kiosk task - run at logon, 10 second delay so server is up first
$kioskArgs = @(
    "--kiosk", "http://localhost:7777/",
    "--edge-kiosk-type=fullscreen",
    "--no-first-run",
    "--disable-pinch",
    "--overscroll-history-navigation=0"
) -join ' '
$kioskAction = New-ScheduledTaskAction -Execute $edgePath -Argument $kioskArgs
$kioskTrigger = New-ScheduledTaskTrigger -AtLogOn -User $user
$kioskTrigger.Delay = "PT10S"

Register-ScheduledTask `
    -TaskName "XeneonDashKiosk" `
    -Action $kioskAction `
    -Trigger $kioskTrigger `
    -Settings $settings `
    -Principal $principal `
    -Description "xeneon-dash Edge kiosk window" `
    -Force | Out-Null

Write-Host ""
Write-Host "Installed."
Write-Host "  XeneonDash      : $exePath  (at logon)"
Write-Host "  XeneonDashKiosk : msedge --kiosk http://localhost:7777  (at logon, 10s delay)"
Write-Host ""
Write-Host "To start now without logging out:"
Write-Host "  schtasks /Run /TN XeneonDash"
Write-Host "  Start-Sleep 6"
Write-Host "  schtasks /Run /TN XeneonDashKiosk"
Write-Host ""
Write-Host "Recommended Windows setting:"
Write-Host "  System -> Power & battery -> Screen -> Never  (on the Xeneon's profile)"
