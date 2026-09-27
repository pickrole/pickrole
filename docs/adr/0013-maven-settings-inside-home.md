# 0013. Maven's settings.xml only inside the home folder

- Status: accepted
- Date: 2026-09-26

## Context

The path of `settings.xml` receives the CodeArtifact token and may come from a configuration file shared by someone
else. Without limits, a tampered file could point to `\\server\share\settings.xml`: on Windows the token would go to
another machine, and opening the share would also send the user's NTLM hash.

## Decision

The path must end in `.xml` and stay inside the home folder **after following links and junctions**. Network paths,
NTFS alternate data streams (`file:stream`) and surrounding spaces are refused. The check runs when the configuration
is saved and again inside `maven.UpsertServer`, where the token is written. The file is written with `0600`, and the
backup is created with `O_EXCL`, which doesn't follow a planted link.

## Consequences

- An imported configuration can't send the token off the machine.
- Anyone keeping `settings.xml` outside the home folder, for instance in the Maven installation, has to use one inside it.
