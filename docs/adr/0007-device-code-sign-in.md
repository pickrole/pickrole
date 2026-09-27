# 0007. Device-code sign-in, renewed with the refresh token

- Status: accepted
- Date: 2026-09-26

## Context

IAM Identity Center supports the OAuth device authorization flow, which needs no local server and no redirect.
Sessions expire, and opening the browser at every expiry is annoying.

## Decision

Register PickRole as a public OIDC client with the `sso:account:access` scope, which makes the flow also return a
refresh token. Signing in opens the browser on the authorization page and shows the code to check. Renewal uses the
refresh token, without the browser, until it expires.

## Consequences

- No local port to open and no redirect to configure.
- When the refresh token expires or is revoked, the backend returns a `LOGIN_REQUIRED` error, and the UI goes back to
  the sign-in and retries the pending action afterwards.
