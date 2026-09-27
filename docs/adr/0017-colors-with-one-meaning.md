# 0017. Colors as tokens, one meaning each

- Status: accepted
- Date: 2026-09-26

## Context

The app shows states users need to tell apart at a glance: primary action, active profile, expiring session and
production account. Reusing a color for different things confuses.

## Decision

Every color is a token in `frontend/src/app.css`, in light and dark themes, and each token has one meaning: `accent`
(action), `ok` (active and success), `warn` (expiring), `prod` (production) and `glow` (brand backdrop). The accent is
emerald green; `ok` is lime, with a hue far enough from the accent not to be confused with it. Details in
[design.md](../design.md).

## Consequences

- Changing the palette means changing the tokens, and the app icon follows the accent.
- A new color comes in as a token, with a defined meaning and contrast checked in both themes.
