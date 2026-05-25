xeneon-dash
================================================================
Local ambient dashboard for the Corsair Xeneon (2560x720)
running on Windows 10/11 with Microsoft Edge in kiosk mode.

------------------------------------------------------------
FIRST-TIME INSTALL
------------------------------------------------------------

1.  Extract this zip to:
        C:\xeneon-dash\

2.  Right-click install-task.ps1 -> Run with PowerShell (as
    Administrator).

3.  Either log out and back in, or start it now:
        schtasks /Run /TN XeneonDash
        Start-Sleep 6
        schtasks /Run /TN XeneonDashKiosk

The kiosk opens http://localhost:7777 in Edge fullscreen.

------------------------------------------------------------
UPDATES
------------------------------------------------------------

1.  Download the newer xeneon-dash-YYYYMMDD.zip
2.  Extract OVER C:\xeneon-dash\  (overwrites .exe + .ps1
    files; leaves config.json and logs/ alone)
3.  Run update.ps1 from PowerShell:
        cd C:\xeneon-dash
        .\update.ps1

------------------------------------------------------------
DESKTOP PANEL MODE (non-kiosk install)
------------------------------------------------------------

If you DON'T have a Xeneon and just want the dashboard as a
borderless panel on the bottom of a side monitor:

1.  Extract this zip somewhere (anywhere).
2.  Run xeneon-dash.exe (it'll listen on port 7777).
3.  Run launch-panel.ps1 (Windows) or launch-panel.sh (Linux).
    - Opens Edge / Chrome in --app mode (no browser chrome).
    - Window is fully movable and resizable.
    - Position + size are remembered for next launch.
4.  Toggle always-on-top:
    - Windows: PowerToys -> Win+Ctrl+T
    - Linux:  wmctrl -r :ACTIVE: -b add,above

------------------------------------------------------------
CONFIG
------------------------------------------------------------

Open http://localhost:7777/config in any browser. Form lets
you set:
    - Primary location (city label, lat, lon, IANA timezone)
      Drives weather, astro, AQHI, ECCC alerts.
      AQHI and ECCC alerts only work in Canada.
    - Secondary clock (city label, IANA timezone)
      Drives the second clock + standup HUD.

Save, then restart xeneon-dash for changes to apply.

You can also edit config.json directly. Defaults are
Hamilton ON + Adelaide AU.

The kiosk Edge task is hard-coded to port 7777. If you
change the port in config, also update the XeneonDashKiosk
scheduled task action.

------------------------------------------------------------
DISABLE SCREEN BLANKING
------------------------------------------------------------

Windows Settings -> System -> Power & battery -> Screen
        On battery / When plugged in -> Never

------------------------------------------------------------
UNINSTALL
------------------------------------------------------------

    schtasks /End    /TN XeneonDash
    schtasks /End    /TN XeneonDashKiosk
    schtasks /Delete /TN XeneonDash      /F
    schtasks /Delete /TN XeneonDashKiosk /F
    Remove-Item -Recurse C:\xeneon-dash\

------------------------------------------------------------
URL
------------------------------------------------------------

http://localhost:7777
