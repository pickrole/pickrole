import type { About, Account, Config, ConnectionTest, DeviceAuth, LoadResult, MavenDetection, Overview, Recent, UpdateInfo, UpdateResult } from './types'

// In-memory stand-in for the Go backend, used by `npm run dev` in a plain
// browser. Data is illustrative only.

const hours = (h: number) => new Date(Date.now() + h * 3_600_000).toISOString()
const ago = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString()
const zero = '0001-01-01T00:00:00Z'
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))

let config: Config = {
  version: 1,
  sso: { startUrl: 'https://example.awsapps.com/start', region: 'us-east-1', sessionName: 'pickrole' },
  codeArtifact: {
    enabled: true,
    domain: 'example',
    domainOwner: '111122223333',
    region: 'us-east-1',
    tools: {
      maven: { enabled: true, serverIds: ['codeartifact'], settingsPath: '~/.m2/settings.xml' },
      gradle: false,
      npm: false,
      pip: false,
    },
  },
  proxy: { mode: 'system', url: '', noProxy: '' },
  preferences: { profileMode: 'default', autoRenew: true, checkUpdates: true, theme: 'dark', language: 'system', prodPattern: '' },
}

// The backend decides which roles are read-only (config.IsReadOnlyRole); the mock only has ReadOnly.
const accounts: Account[] = (
  [
  { id: '111111111111', name: 'data-dev', email: '', roles: ['Developer'], production: false, favorite: false },
  { id: '222222222222', name: 'data-prod', email: '', roles: ['Developer', 'ReadOnly'], production: true, favorite: false },
  { id: '333333333333', name: 'platform-dev', email: '', roles: ['Admin', 'Developer', 'ReadOnly'], production: false, favorite: true },
  { id: '444444444444', name: 'platform-prod', email: '', roles: ['Admin', 'ReadOnly'], production: true, favorite: true },
  { id: '555555555555', name: 'sandbox', email: '', roles: ['Admin'], production: false, favorite: false },
  ] as Omit<Account, 'readOnlyRoles'>[]
).map((a) => ({ ...a, readOnlyRoles: (a.roles ?? []).filter((r) => r === 'ReadOnly') }))

let recents: Recent[] = [
  { accountId: '333333333333', accountName: 'platform-dev', role: 'Developer', usedAt: ago(15), production: false },
  { accountId: '111111111111', accountName: 'data-dev', role: 'Developer', usedAt: ago(72), production: false },
  { accountId: '444444444444', accountName: 'platform-prod', role: 'ReadOnly', usedAt: ago(120), production: true },
]

let active: Overview['active'] = null
let loggedIn = true
let sessionExpires = hours(7.2)
const access: Record<string, boolean> = {
  '333333333333/Developer': true,
  '333333333333/Admin': true,
  '333333333333/ReadOnly': false,
  '444444444444/ReadOnly': false,
}

const overview = (): Overview => ({
  version: 'mock',
  configured: true,
  config,
  session: { loggedIn, expiresAt: loggedIn ? sessionExpires : zero },
  accounts: accounts.map((a) => ({ ...a })),
  cacheUpdatedAt: ago(52),
  active,
  recents,
  codeArtifactAccess: { ...access },
})

export const mock = {
  async Overview() {
    return overview()
  },
  async DetectSSO() {
    return null
  },
  async DefaultConfig() {
    return structuredClone(config)
  },
  async SaveConfig(cfg: Config) {
    config = structuredClone(cfg)
    return overview()
  },
  async TestConnection(cfg: Config): Promise<ConnectionTest> {
    await wait(700)
    const manual = cfg.proxy.mode === 'manual'
    return {
      target: `oidc.${cfg.sso.region}.amazonaws.com`,
      proxy: manual ? cfg.proxy.url.replace(/^[a-z0-9]+:\/\//, '').replace(/\/$/, '') : '',
      source: manual ? 'manual' : cfg.proxy.mode === 'none' ? 'none' : 'windows',
      note: '',
      error: '',
    }
  },
  async DetectMaven(_cfg: Config): Promise<MavenDetection> {
    await wait(400)
    return {
      serverIds: ['ca-releases', 'ca-snapshots', 'ca-plugins', 'ca-mirror'],
      domains: [{ domain: 'example', owner: '111122223333', region: 'us-east-1' }],
    }
  },
  async ImportConfig() {
    return structuredClone(config)
  },
  async ExportConfig() {
    return '/home/you/pickrole.json'
  },
  async StartLogin(): Promise<DeviceAuth> {
    return { userCode: 'WDJB-MJHT', verificationUrl: 'https://device.sso.us-east-1.amazonaws.com/', expiresAt: hours(0.15) }
  },
  async WaitLogin() {
    await wait(2500)
    loggedIn = true
    sessionExpires = hours(8)
    return overview()
  },
  async OpenURL() {},
  async RenewSession() {
    await wait(600)
    sessionExpires = hours(8)
    return overview()
  },
  async RefreshAccounts() {
    await wait(900)
    return overview()
  },
  async LoadProfile(accountId: string, role: string): Promise<LoadResult> {
    await wait(700)
    const acct = accounts.find((a) => a.id === accountId)!
    const ca = !/read/i.test(role)
    access[`${accountId}/${role}`] = ca
    active = {
      accountId,
      accountName: acct.name,
      role,
      profile: 'default',
      expiresAt: hours(8),
      codeArtifact: ca,
      codeArtifactExpiresAt: ca ? hours(12) : zero,
    }
    recents = [
      { accountId, accountName: acct.name, role, usedAt: new Date().toISOString(), production: acct.production },
      ...recents.filter((r) => !(r.accountId === accountId && r.role === role)),
    ]
    return {
      active,
      written: ['~/.aws/credentials [default]', ...(ca ? ['~/.m2/settings.xml'] : [])],
      warnings: [],
    }
  },
  async ToggleFavorite(accountId: string) {
    const a = accounts.find((x) => x.id === accountId)
    if (a) a.favorite = !a.favorite
    return overview()
  },
  async CopyText(text: string) {
    await navigator.clipboard?.writeText(text)
  },
  async CopyExport() {},
  async SetLanguage(_lang: string) {},
  async CheckUpdate(): Promise<UpdateInfo> {
    return { available: true, version: '0.2.0-beta.9', url: 'https://github.com/pickrole/pickrole/releases', canInstall: true }
  },
  // Shows the Linux fallback, since the mock can't restart anything.
  async ApplyUpdate(): Promise<UpdateResult> {
    await wait(1200)
    return {
      manualCommand: "sudo dnf install '/home/you/.cache/pickrole/updates/pickrole_0.2.0-beta.9_el8_x86_64.rpm'",
      reason: 'exit status 126: Not authorized',
    }
  },
  async About(): Promise<About> {
    return {
      version: 'mock',
      commit: 'a1b2c3d',
      buildDate: ago(3),
      go: 'go1.27.0',
      platform: 'windows/amd64',
      repository: 'https://github.com/pickrole/pickrole',
      files: [
        { kind: 'credentials', path: '~/.aws/credentials' },
        { kind: 'ssoCache', path: '~/.aws/sso/cache' },
        { kind: 'config', path: '~/AppData/Roaming/pickrole/config.json' },
        { kind: 'maven', path: '~/.m2/settings.xml' },
      ],
    }
  },
}
