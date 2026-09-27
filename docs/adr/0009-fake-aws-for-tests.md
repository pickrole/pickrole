# 0009. A local fake AWS for tests

- Status: accepted
- Date: 2026-09-26

## Context

Testing the real flow needs IAM Identity Center with AWS Organizations, which not every contributor has. Interface
mocks would test the code but not the real conversation with the SDK (serialization, errors, pagination).

## Decision

Keep in `internal/fakeaws` a fake AWS that implements the operations PickRole uses in the real REST-JSON format: SSO
OIDC, the SSO portal and CodeArtifact's `GetAuthorizationToken`. The SDK clients point to it through
`AWS_ENDPOINT_URL`, the same variable the AWS CLI uses. `cmd/fakeaws` serves the same fake for manual testing.

## Consequences

- The `internal/app` tests cover the whole flow with the real SDK clients.
- Anyone can test the app without an AWS account (`scripts/mock-aws.ps1` on Windows).
- The fake has to follow the format of the operations used. It has no authentication, so it only listens on loopback
  unless `-allow-remote` is passed.
