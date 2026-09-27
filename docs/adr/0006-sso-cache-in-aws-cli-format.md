# 0006. SSO cache in the AWS CLI format

- Status: accepted
- Date: 2026-09-26

## Context

PickRole users also use the AWS CLI. If each kept its own session, users would sign in twice and the sessions would
expire at different times.

## Decision

Keep the SSO token in `~/.aws/sso/cache/<sha1 of the session name>.json`, with the same fields as the AWS CLI.

## Consequences

- A session opened in PickRole works for `aws --profile` with the same `sso-session`, and the other way around.
- The file name uses SHA-1 because that's the CLI's format, not for security (annotated for `gosec`).
- Changes to the AWS CLI format have to be followed.
