# 0023. Automatic renewal of the active profile

- Status: accepted
- Date: 2026-09-28

## Context

Role credentials last as long as the permission set's session duration, often one hour. When they expired, every
terminal failed until the user loaded the profile again by hand, and the CodeArtifact token, although it lasts longer,
was only renewed that way too. The config already had an `autoRenew` preference, on by default, that nothing used.

## Decision

- While PickRole is open, the backend checks once a minute (`internal/app/renew.go`):
  - when the SSO access token expires in less than 15 minutes and there is a refresh token, it renews the session
    silently, as **Renew** does;
  - when the active profile's credentials expire in less than 10 minutes, it loads that profile again, the same way
    **Load** does: new credentials in `~/.aws/credentials` and, with CodeArtifact, a new token in `settings.xml`.
- A renewal doesn't count as using the profile: the recent list and "Pick up where you left off" don't change.
- The backend tells the UI through Wails events: `overview` when something was renewed, so the header shows the new
  times, and `renew-error` with the message when a renewal fails. The same error is sent once, not every minute.
- The `autoRenew` preference turns it off, in **Settings → Preferences**. It stays on by default.
- It never opens the browser. When the refresh token is no longer accepted, the user signs in again, as before.

## Alternatives considered

- **`credential_process` in `~/.aws/config`**: the AWS tools would ask PickRole for credentials on demand, with no
  timer. But it needs the PickRole binary to answer from the command line, doesn't cover the CodeArtifact token in
  `settings.xml`, and changes where credentials live ([ADR 0002](0002-credentials-in-files.md)).
- **Renew only the CodeArtifact token**: the role credentials, which expire first, would still break the terminals.

## Consequences

- With PickRole open, terminals keep valid credentials for as long as the SSO session can be renewed, usually the
  working day; after that the header asks for a new sign-in, as before.
- With PickRole closed, nothing is renewed: credentials expire as they always did.
- `settings.xml` is rewritten on each renewal when CodeArtifact is on. The edit only touches the configured
  `<server>` entries ([ADR 0022](0022-several-maven-servers.md)).
