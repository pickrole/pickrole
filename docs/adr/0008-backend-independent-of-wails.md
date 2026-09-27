# 0008. Backend independent of Wails

- Status: accepted
- Date: 2026-09-26

## Context

Wails needs GTK and WebKit to build on Linux and only runs with a window. Testing the app's logic through it would be
slow and impossible on a CI without a display.

## Decision

All the logic lives in `internal/app` and the packages below it, which **don't import Wails**. What depends on the
desktop (opening URLs, dialogs, clipboard) sits behind the `app.Platform` interface, implemented in `main.go`.

## Consequences

- `go test ./internal/...` runs anywhere, without GTK; tests use a fake `Platform`.
- `main.go` stays thin: new logic goes into `internal/`.
