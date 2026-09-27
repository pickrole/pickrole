# 0001. Record architecture decisions

- Status: accepted
- Date: 2026-09-26

## Context

The project is open and takes contributions. Several choices, like writing credentials to a file or editing files as
text, look odd without their context and would be easy to undo by mistake.

## Decision

Every decision with a lasting impact becomes a numbered file in `docs/adr/`, following `template.md`. An accepted ADR
isn't edited: if the decision changes, a new ADR supersedes it and the old one points to the new one.

## Consequences

- Contributors find the why before changing something.
- Changing course means writing a new ADR, a small and intentional cost.
