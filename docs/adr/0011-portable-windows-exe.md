# 0011. Windows as a portable .exe, with no installer

- Status: accepted
- Date: 2026-09-26

## Context

On managed workstations, installing software often needs admin rights. WebView2 ships with Windows 10 and 11.

## Decision

Ship Windows as a portable `.exe` in a `.zip`, built by `scripts/build-windows.ps1`, with no installer.

## Consequences

- Runs without installation and without admin rights.
- No Start menu shortcut and no uninstaller. An installer (NSIS, which Wails supports) can come later.
- The `.exe` isn't code-signed, so SmartScreen warns on first run. Signing needs a code-signing certificate.
