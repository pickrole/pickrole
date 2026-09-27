// Types mirror the Go structs in internal/app, internal/config and
// internal/store. Times arrive as RFC 3339 strings; Go's zero time means
// "not set".

export type ProfileMode = 'default' | 'named'
export type Theme = 'system' | 'light' | 'dark'

export interface SSO {
  startUrl: string
  region: string
  sessionName: string
}

export interface Maven {
  enabled: boolean
  serverId: string
  settingsPath: string
}

export interface Tools {
  maven: Maven
  gradle: boolean
  npm: boolean
  pip: boolean
}

export interface CodeArtifact {
  enabled: boolean
  domain: string
  domainOwner: string
  region: string
  repository: string
  tools: Tools
}

export interface Preferences {
  profileMode: ProfileMode
  autoRenew: boolean
  startMinimized: boolean
  theme: Theme
  /** 'system' follows the OS language. */
  language: 'system' | 'en' | 'pt-BR'
  prodPattern: string
}

/** 'system': HTTPS_PROXY/NO_PROXY when set, otherwise the OS settings. */
export type ProxyMode = 'system' | 'manual' | 'none'

export interface Proxy {
  mode: ProxyMode
  /** Only for 'manual', e.g. http://proxy.example.com:3128. */
  url: string
  /** Hosts that skip the proxy, comma-separated. */
  noProxy: string
}

export interface Config {
  version: number
  sso: SSO
  codeArtifact: CodeArtifact
  proxy: Proxy
  preferences: Preferences
}

/** Mirrors app.ConnectionTest. */
export interface ConnectionTest {
  /** The host tried: the SSO sign-in endpoint. */
  target: string
  /** "host:port" of the proxy used; empty for a direct connection. */
  proxy: string
  source: 'manual' | 'environment' | 'windows' | 'gnome' | 'none'
  /** A system proxy setting PickRole couldn't use. */
  note: string
  /** Empty when the target answered. */
  error: string
}

export interface Account {
  id: string
  name: string
  email: string
  roles: string[] | null
  production: boolean
  favorite: boolean
  /** Roles whose names say they only read; in production the others ask for confirmation. */
  readOnlyRoles: string[] | null
}

export interface Recent {
  accountId: string
  accountName: string
  role: string
  usedAt: string
  production: boolean
}

export interface Active {
  accountId: string
  accountName: string
  role: string
  profile: string
  expiresAt: string
  codeArtifact: boolean
  codeArtifactExpiresAt: string
}

export interface Session {
  loggedIn: boolean
  expiresAt: string
}

/** Mirrors app.About: build details and the files PickRole uses. */
export interface About {
  version: string
  commit: string
  /** RFC 3339 (UTC); empty for local builds without build scripts. */
  buildDate: string
  go: string
  /** GOOS/GOARCH, e.g. "windows/amd64". */
  platform: string
  repository: string
  files: { kind: 'credentials' | 'ssoCache' | 'config' | 'maven'; path: string }[]
}

export interface Overview {
  version: string
  configured: boolean
  config: Config
  session: Session
  accounts: Account[]
  cacheUpdatedAt: string
  active: Active | null
  recents: Recent[]
  /** "accountId/role" → had CodeArtifact access last time. Absent = unknown. */
  codeArtifactAccess: Record<string, boolean>
}

export interface DeviceAuth {
  userCode: string
  verificationUrl: string
  expiresAt: string
}

export interface LoadResult {
  active: Active
  written: string[]
  warnings: string[]
}
