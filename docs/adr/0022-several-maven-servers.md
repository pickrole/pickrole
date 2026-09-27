# 0022. Several Maven servers, detected from settings.xml

- Status: accepted
- Date: 2026-09-27
- Extends: [0005](0005-surgical-file-edits.md)

## Context

PickRole wrote the CodeArtifact token into a single `<server>` of `settings.xml`, the one with the configured id. Real
`settings.xml` files often have many CodeArtifact entries: one `<server>` per repository, plugin repository or mirror,
spread over several profiles, all using the same token. The AWS instructions make each of them read it from the
environment (`<password>${env.CODEARTIFACT_AUTH_TOKEN}</password>`), and a script exports the variable. With one id,
PickRole updated one entry and Maven still sent an empty password for the others, failing with 401 while PickRole
reported success.

Users also didn't know which ids to enter, nor where the domain, owner account and region come from, although all of
it is already in `settings.xml`.

## Decision

- The config holds a list of server ids (`serverIds`). Loading a profile writes the token into every one of them in a
  single edit, with the same rules as before: text edit, one backup, `0600`, path inside the home folder. A config
  with the old single `serverId` is read as a one-item list.
- **Settings → CodeArtifact → Detect in settings.xml** reads the file and fills the list with:
  - the id of every `<repository>`, `<pluginRepository>` and `<mirror>` whose URL is a CodeArtifact repository, only
    of the configured domain when one is set;
  - the id of every `<server>` whose password reads an environment variable with `CODEARTIFACT` in its name.

  When the domain is empty and the file points to a single CodeArtifact domain, detection also fills the domain,
  owner account and region from the URL.
- Detection only fills the form: nothing is written until the user saves, and the token only goes to the listed ids.
  PickRole never edits entries on its own guess at load time.

## Alternatives considered

- **Update every server that reads the variable, at load time, with no list**: no setup, but PickRole would change
  entries the user never saw, and servers without the variable (a literal old token) would be missed.
- **Keep the environment variable and write it where `mvn` reads it** (`~/.mavenrc`): keeps `settings.xml` untouched,
  but IDEs that embed Maven don't run that script, and it means writing a token into a file a shell executes.

## Consequences

- One PickRole setup covers every CodeArtifact repository of a domain, in every profile.
- The `${env.CODEARTIFACT_AUTH_TOKEN}` references of the listed servers are replaced by the token itself; the
  original stays in the `.pickrole.bak` backup.
- A `settings.xml` pointing to more than one CodeArtifact domain still gets the token of one domain only; detection says
  so.
