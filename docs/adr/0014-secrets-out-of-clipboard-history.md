# 0014. Secrets out of the clipboard history

- Status: accepted
- Date: 2026-09-26

## Context

"Copy export" puts the secret key and session token on the clipboard. On Windows 10 and 11 that goes to the clipboard
history (Win+V) and, with sync on, to the cloud and the user's other devices.

## Decision

Secrets are copied with `Platform.SetSecretClipboard`. On Windows, `internal/clipboard` marks the content with the
`ExcludeClipboardContentFromMonitorProcessing`, `CanIncludeInClipboardHistory = 0` and `CanUploadToCloudClipboard = 0`
formats, the same ones password managers use. Credentials received from AWS are validated before they reach a file or
the clipboard.

## Consequences

- On Windows, copied credentials stay out of the history and aren't synced.
- On Linux the copy is a plain one: a clipboard manager may keep it.
- Any new secret the UI copies must use `SetSecretClipboard`.
