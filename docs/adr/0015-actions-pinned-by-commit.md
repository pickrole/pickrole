# 0015. GitHub Actions pinned by commit

- Status: accepted
- Date: 2026-09-26

## Context

The workflows build the published binaries, and the release workflow can write to the repository. An action referenced
by tag (`@v4`) can change if the tag is moved, including through an attack on that action's project.

## Decision

Every action is referenced by commit SHA, with the version in a comment. Dependabot opens pull requests when new
versions come out.

## Consequences

- The code that runs in CI only changes through a reviewed pull request.
- Updates arrive as Dependabot pull requests, which need review, especially for major versions.
