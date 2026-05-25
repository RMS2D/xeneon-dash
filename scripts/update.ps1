# update.ps1 - after extracting a newer zip over C:\xeneon-dash\, run this.
# Stops the server, lets the new exe take, restarts. Does NOT touch config.json.

$ErrorActionPreference = "Stop"

Write-Host "Stopping XeneonDash ..."
schtasks /End /TN "XeneonDash" 2>$null | Out-Null
Start-Sleep -Seconds 2
Get-Process -Name "xeneon-dash" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

Write-Host "Starting XeneonDash ..."
schtasks /Run /TN "XeneonDash" | Out-Null

Write-Host "Refreshing kiosk browser ..."
schtasks /End /TN "XeneonDashKiosk" 2>$null | Out-Null
Start-Sleep -Seconds 3
schtasks /Run /TN "XeneonDashKiosk" | Out-Null

Write-Host "Done."
