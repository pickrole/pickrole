# 0016. Portuguese UI, English code

- Status: superseded by [0018](0018-english-first-localized-ui.md)
- Date: 2026-09-26

## Context

The first users spoke Portuguese. Code in English makes contributions easier and follows Go and ecosystem conventions.

## Decision

UI, error messages and documentation in **Brazilian Portuguese**. Code, comments and names in **English**.

## Consequences

- User-facing messages were written in Portuguese directly in Go.
- An English UI was on the roadmap and would need the strings extracted into translations. That happened in
  [ADR 0018](0018-english-first-localized-ui.md).
