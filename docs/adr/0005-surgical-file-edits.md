# 0005. Surgical edits to the credentials file and settings.xml

- Status: accepted
- Date: 2026-09-26

## Context

`~/.aws/credentials` and Maven's `settings.xml` belong to the user: they hold other profiles, comments, formatting and
entries PickRole knows nothing about. Rewriting them from a parser would drop comments and change the formatting.

## Decision

Edit both files **as text**, replacing only the target profile section or `<server>` entry and keeping the rest
untouched. Writes are atomic (a temporary file, then `rename`). The first time an existing `settings.xml` changes, the
original is kept in `settings.xml.pickrole.bak`.

## Alternatives considered

- **Parse and rewrite with an INI or XML library**: loses comments and formatting and may reorder the file.

## Consequences

- Users can keep editing these files by hand.
- The editing code has to handle cases like commented-out blocks and missing elements, and has a test for each.
