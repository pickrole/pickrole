# 0025. The new-version notice lives in About

- Status: accepted
- Date: 2026-09-28
- Amends: [0024](0024-updates-from-the-app.md)

## Context

[ADR 0024](0024-updates-from-the-app.md) put a "Version X" badge in the header when a newer release is out. The header
already holds the active profile, the SSO session and three buttons; a badge there competes with what people look at
every day. The version itself is shown in the About dialog.

## Decision

- The header has no badge. The **About** button (in the main header and on the sign-in screen) gets a small accent dot
  when a newer release is out, and its label says so.
- The About dialog shows, above the version, "Version X is available" with **Update**, which opens the update dialog of
  ADR 0024. Without a newer release it offers **Check for updates**, which checks right away, even with the periodic
  check turned off in the preferences.

## Consequences

- The notice is visible but quiet: a dot, like a notification, where the version already is.
- People who never open About still see the dot; the rest of ADR 0024 (download, checksum, install, restart) is
  unchanged.
