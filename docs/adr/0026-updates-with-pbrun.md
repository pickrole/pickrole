# 0026. Updates on machines that elevate through pbrun

- Status: accepted
- Date: 2026-09-28
- Amends: [0024](0024-updates-from-the-app.md)

## Context

[ADR 0024](0024-updates-from-the-app.md) installs Linux updates with `pkexec`, which asks for an administrator password
in a desktop dialog, and falls back to a `sudo dnf install …` command. Many companies don't give users that password or
`sudo`: administrator commands go through a privilege manager such as BeyondTrust's `pbrun`, which runs from a terminal
and follows a company policy. There, `pkexec` asks for a password the user doesn't have, and the `sudo` command doesn't
apply.

## Decision

- When `pbrun` is installed (in `PATH`, or in `/usr/bin`, `/usr/local/bin` or `/usr/sbin`, since an app started from
  the menu may have a short `PATH`), PickRole doesn't try `pkexec`.
- It still downloads the package and checks it against `SHA256SUMS`, then shows `pbrun dnf install '<file>'` (or
  `pbrun apt install`) to run in a terminal.
- The update dialog says so before anything happens ("Download and check", and the install is done with the command it
  shows), instead of promising a password prompt.

## Consequences

- On pbrun machines the update takes one command, with the right tool, and no failed password prompt.
- Whether that command is allowed depends on the company's pbrun policy; PickRole can't know. When it isn't, the
  install follows the company's own process, with the checked package already downloaded.
