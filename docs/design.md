# Design

## Colors

Every color is a token in `frontend/src/app.css`, with a dark theme (the default) and a light one; the settings can also
follow the system. **Each color has one meaning**
([ADR 0017](adr/0017-colors-with-one-meaning.md)):

| Token | Dark | Light | Meaning |
|---|---|---|---|
| `accent` | `#34C08F` | `#0D7A58` | Primary action. Also the app icon color. |
| `glow` | `#154837` | `#CFE9DD` | The accent's deep tone, only for the backdrop of the opening screen and the About dialog. |
| `ok` | `#A3CF62` | `#5A8A16` | Active profile, success. Lime (hue ~84°), far from the accent (~160°). |
| `warn` | `#F0A848` | `#B8741A` | Needs attention soon: the session about to expire, an update that fixes a security issue. |
| `prod` | `#F07A6A` | `#C4412F` | Production accounts. Write roles in production ask for confirmation. |

Rules:

- Don't use a color outside its meaning, and don't add colors outside the tokens.
- Minimum contrast (WCAG): 4.5:1 for text, 3:1 for graphics, in both themes.
- The app icon comes from `build/appicon.svg`, which generates `build/appicon.png` and `build/windows/icon.ico`. If
  the accent changes, regenerate all three.

## Brand

The logo and name appear only on the opening screen and in the About dialog. The window title bar already shows the
icon and name, so headers don't repeat the brand.

Instead, the left of the main header shows the **active profile** (account / role, with the PROD tag in production),
since that's what every terminal is using right now. Clicking it opens the account. When the role credentials have
expired, the dot turns gray and says so. Without an active profile it reads "No active profile".

### Opening screen (`frontend/src/lib/Splash.svelte`)

- The logo and name appear, and the tagline is typed out (~2 s). Then the caret keeps blinking and the screen asks for
  **Enter or a click** to continue. A key or click while typing only completes the text.
- The tagline is a list of lines with fixed breaks (`brand.tagline.*` in the i18n catalogs). Lines never wrap on their
  own and the caret takes no width, so nothing moves while typing.
- It only leaves once the data is loaded, and it honors `prefers-reduced-motion`.
- The start panel's Enter ignores key repeats, so holding the opening screen's Enter doesn't load a profile.

### About (`frontend/src/lib/About.svelte`)

Opened with the ⓘ button in the header, with the opening screen's look. Shows the version, commit, build date,
platform, Go version and the files the app uses. "Copy details" produces text ready for bug reports.

## Languages

English and Brazilian Portuguese, following the system by default and switchable in **Settings → Preferences →
Language** ([ADR 0018](adr/0018-english-first-localized-ui.md)). The strings are in `frontend/src/lib/i18n/` (UI) and
`internal/i18n/messages.go` (backend messages). When adding a string, add it to every language: TypeScript and a Go
test fail otherwise.

Copy guidelines, in both languages: sentence case, short labels, verbs on buttons ("Load", "Copy details"), and errors
that say what happened and what to do next.
