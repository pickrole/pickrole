# 0027. The release feed when the GitHub API refuses

- Status: accepted
- Date: 2026-09-28
- Amends: [0024](0024-updates-from-the-app.md)

## Context

[ADR 0024](0024-updates-from-the-app.md) lists releases through the GitHub REST API. Without a token, GitHub allows
60 calls an hour per IP address, and a company network shows up as one address for everyone behind it, so the check
can fail with HTTP 403 no matter how rarely PickRole itself asks. Some company proxies also let `github.com` through
and refuse `api.github.com`. The error shown was the raw URL and status code, which said nothing about either.

## Decision

- When the API answers with an error status, PickRole reads the Atom feed of the release page
  (`https://github.com/pickrole/pickrole/releases.atom`) instead. It is served by `github.com`, like the downloads, and
  doesn't count against the API limit.
- The feed has no drafts and no pre-release flag: pre-releases are told apart by their version, as tags with a suffix
  are always published as pre-releases. It doesn't list the files either: they are downloaded from the release's usual
  address (`…/releases/download/<tag>/<file>`) and still checked against its `SHA256SUMS`, so nothing about the
  download's safety changes. A `Security` heading in the notes (HTML there) still marks a security update.
- When both fail, the message says what the failure means: GitHub's rate limit (and when it resets), a refusal that
  may come from the proxy, no answer in time (check the proxy), or another status.

## Consequences

- The check works on networks that share an address or only allow `github.com`.
- The feed only covers the latest releases (about ten), which is enough to find the newest one; a security fix in an
  older, skipped release could go unnoticed in the rare case the API is refused and more than ten releases were
  skipped.
