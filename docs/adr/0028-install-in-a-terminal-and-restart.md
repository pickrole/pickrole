# 0028. Install in a terminal PickRole opens, and restart after it

- Status: accepted
- Date: 2026-09-29
- Amends: [0026](0026-updates-with-pbrun.md)

## Context

On machines that elevate through `pbrun` ([ADR 0026](0026-updates-with-pbrun.md)), PickRole showed the install command
to copy into a terminal. Once the package was installed, the open PickRole was still the old version until it was
closed and opened again by hand, and nothing said so. `pbrun` needs a terminal and follows a company policy, so a
program can't install silently there, and it shouldn't try.

## Decision

- On those machines **Update now** downloads and checks the package, then opens a terminal window (the first found of
  `gnome-terminal`, `konsole`, `xfce4-terminal`, `x-terminal-emulator`, `xterm`) running the same `pbrun dnf install`
  (or `apt install`) command. The user only answers `pbrun` if it asks. When the install fails, the window stays open
  so the error can be read. The command is still shown, in case no terminal appears; with no terminal found, it's the
  only option, as before.
- On Linux, PickRole watches its executable (every few seconds; a package manager puts a new file in its place). After
  an update started from PickRole, it restarts into the new version by itself. When the new version was installed
  some other way, the **About** button gets the dot and About offers **Restart**.
- Local builds (version `dev`) aren't watched, and Windows doesn't need it: there the update already restarts
  PickRole.

## Consequences

- An update on a `pbrun` machine is a click and, at most, the `pbrun` prompt: nothing to copy, and no manual restart.
- The terminal runs a command PickRole built from its own checked download; the user still sees it and authorizes it
  in `pbrun`.
- Whether the command is allowed still depends on the company's policy.
