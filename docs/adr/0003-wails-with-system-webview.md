# 0003. Desktop app with Wails v2 and the system webview

- Status: accepted
- Date: 2026-09-26

## Context

The app needs a pleasant UI, has to run on RHEL 8 and 9 and on Windows, should be light, and needs a backend with
direct access to files and to the AWS SDK. On managed workstations, installing runtimes is often hard.

## Decision

Use **Wails v2**, with a Go backend and a web UI rendered by the system webview: WebKitGTK on Linux and WebView2 on
Windows.

## Alternatives considered

- **Electron**: bundles Chromium, with binaries of hundreds of MB and high memory use; the backend would be Node,
  without the Go AWS SDK.
- **Tauri**: a good candidate, but the backend would be Rust, and Tauri 2 uses webkit2gtk-4.1, which RHEL 8 doesn't have.
- **Fyne or Gio (native Go UI)**: less flexible look and more work to reach the intended design.
- **Wails v3**: v2 is the stable line and builds with RHEL 8's webkit2gtk-4.0. Moving to v3 is a future decision with
  its own ADR.

## Consequences

- A single small binary with no extra runtime.
- The same UI runs on different engines (WebKitGTK and WebView2): CSS features need testing on both.
- Wails v2 has no tray icon: that roadmap item needs another library.
