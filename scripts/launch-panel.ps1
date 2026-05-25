# launch-panel.ps1 - opens xeneon-dash as a borderless desktop panel.
# Window position/size are remembered between launches via a dedicated
# user-data-dir. Drag/resize the window normally; Edge remembers it next time.
# Press Win+Ctrl+T (Windows PowerToys) to toggle always-on-top.

$ErrorActionPreference = "Stop"

$url        = "http://localhost:7777"
$userData   = "$env:LOCALAPPDATA\xeneon-dash-panel"

$edge = "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe"
if (-not (Test-Path $edge)) { $edge = "${env:ProgramFiles}\Microsoft\Edge\Application\msedge.exe" }
if (-not (Test-Path $edge)) { throw "Microsoft Edge not found." }

if (-not (Test-Path $userData)) { New-Item -ItemType Directory -Path $userData -Force | Out-Null }

# First launch only - hint default geometry. Subsequent launches use remembered values.
$firstLaunch = -not (Test-Path "$userData\Default\Preferences")
$args = @(
    "--app=$url",
    "--user-data-dir=$userData",
    "--no-first-run",
    "--no-default-browser-check",
    "--disable-features=msExperimentalScrolling"
)
if ($firstLaunch) {
    $args += "--window-size=1920,300"
    $args += "--window-position=0,1080"
}

Start-Process -FilePath $edge -ArgumentList $args
Write-Host "Panel opened. Drag/resize freely; geometry is remembered for next launch."
Write-Host "Always-on-top: Win+Ctrl+T (requires PowerToys)."
Write-Host ""
Write-Host "To remove the Edge title bar entirely, install as PWA:"
Write-Host "  ... menu (top-right) -> Apps -> Install xeneon-dash"
Write-Host "Then launch from Start menu; opens chromeless."
