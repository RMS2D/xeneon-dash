# xeneon-dash

Local ambient dashboard for the Corsair Xeneon 32:9 (2560x720) or any
side-monitor strip. Single Go binary, vanilla JS frontend, runs offline.

## What it shows

- Two configurable clocks (primary + second city)
- Weather: current + 24h hourly + tomorrow's forecast, with WMO icons
- Astro: sun and moon arcs with live position tracking, sunrise/sunset
  countdown, moon phase shape on the tracker
- Air quality: Canadian AQHI (ECCC) for Canada locations, European AQI (Open-Meteo, global) elsewhere
- Environment Canada weather alerts banner (Canada only)
- Security feed from omnomfeeds.com with CVE chips and severity tints
- Top bar: date, UTC, AQHI, top-mentioned CVE of the day, feed item count
- Night dim from midnight to 6 AM, automatic 4 AM reload, embedded
  JetBrains Mono

## Two ways to run

### Kiosk (full-screen)

1. Download the latest `xeneon-dash-YYYYMMDD.zip` from the
   [releases page](https://github.com/RMS2D/xeneon-dash/releases)
   (or build from source - see below)
2. Extract to `C:\xeneon-dash\`
3. Right-click `install-task.ps1` -> Run with PowerShell (as Administrator)
4. Log out and back in, or start it now:
   ```
   schtasks /Run /TN XeneonDash
   schtasks /Run /TN XeneonDashKiosk
   ```

### Desktop panel (resizable, for a strip on the side of your monitor)

1. Extract the zip anywhere
2. Run `xeneon-dash.exe`
3. Run `launch-panel.ps1` (Windows) or `launch-panel.sh` (Linux)

Opens Edge / Chrome in `--app` mode. Drag the top bar to move, drag
edges to resize. The dashboard automatically scales to fit any window
size while preserving its 32:9 aspect ratio. Position and size are
remembered between launches via a dedicated `--user-data-dir`.

**Remove the title bar:** install as a PWA. In the launched window
click `...` (top-right) -> Apps -> Install xeneon-dash. Then launch
from the Start menu / app drawer; opens chromeless.

**Always on top:** PowerToys `Win+Ctrl+T` on Windows;
`wmctrl -r :ACTIVE: -b add,above` on Linux.

## Config

Open `http://localhost:7777/config` in any browser. Form lets you set:

- **Primary location** (city label, lat, lon, IANA timezone) - drives
  weather, astro, AQHI, and alerts
- **Secondary clock** (city label, IANA timezone) - drives the second
  clock and the standup countdown

Save, then restart `xeneon-dash.exe` for changes to apply. You can
also edit `config.json` directly.

Defaults are Hamilton ON + Adelaide AU.

Air quality switches automatically: Canadian AQHI (1-10 scale) for
Canada, European AQI (0-100+ scale) everywhere else. ECCC alerts only
fire for Canadian locations.

## Build from source

Windows:

```powershell
.\scripts\build.ps1
```

Produces `dist/xeneon-dash-YYYYMMDD.zip` with the binary, install
scripts, panel-launcher scripts, and a README.

## Stack

- Go 1.22, single binary via `embed.FS`
- Vanilla JS frontend, no build step
- IANA tz database embedded via `time/tzdata`
- Open-Meteo (weather + air quality fallback), Environment Canada
  GeoMet (AQHI, alerts), omnomfeeds.com (security RSS)

## License

MIT. See `LICENSE`.
