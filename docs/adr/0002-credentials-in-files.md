# 0002. Credentials in files, never in environment variables

- Status: accepted
- Date: 2026-09-26

## Context

The common ways to use IAM Identity Center leave credentials in environment variables (`export AWS_...`) or depend on
a script run in every shell. Variables only live in the shell that set them: IDEs, new terminals and background jobs
don't see them, and switching accounts means repeating the process everywhere.

## Decision

PickRole writes the temporary credentials to the shared AWS credentials file (`~/.aws/credentials`), in the
`[default]` profile or in one named profile per account and role. Every AWS client (CLI, SDKs, IDEs, Terraform) reads
that file with no setup. Copying `export` lines stays available as a shortcut for one-off cases, never as the main path.

## Alternatives considered

- **`credential_process` in `~/.aws/config`**: needs PickRole reachable on every call from every client.
- **Environment variables**: the very problem the project exists to solve.

## Consequences

- Switching profiles affects every terminal and IDE on their next command, with nothing to restart.
- The file holds secrets: `0600` permissions, atomic writes and edits limited to the target section
  ([ADR 0005](0005-surgical-file-edits.md)).
