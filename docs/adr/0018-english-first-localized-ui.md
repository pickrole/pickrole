# 0018. English-first project with a localized UI

- Status: accepted
- Date: 2026-09-26

## Context

The repository is going public, and English reaches far more people who look for AWS tooling. A UI only in Portuguese
would greet most of them in a language they don't read, while the first users read Portuguese.
Supersedes [ADR 0016](0016-languages.md).

## Decision

- Repository in **English**: documentation, ADRs, changelog, commits, pull requests and developer tooling (scripts,
  the fake AWS). `README.pt-BR.md` keeps a Portuguese summary.
- UI in **English and Brazilian Portuguese**, following the system language by default, with English as the fallback,
  and switchable in the settings (`language`: `system`, `en`, `pt-BR`).
- Frontend strings in `frontend/src/lib/i18n/`, with `en.ts` as the reference and TypeScript enforcing the same keys in
  every language. Backend messages in `internal/i18n`, with a test for missing keys and mismatched format verbs; the UI
  tells the backend the resolved language with `Service.SetLanguage`.
- No i18n library: two languages and a few hundred strings don't justify one.

## Consequences

- Every user-facing string needs an entry in each language.
- Adding a language means one catalog file on each side.
- History from before this decision (commits and old pull requests) stays in Portuguese.
