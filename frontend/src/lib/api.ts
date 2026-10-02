import type {
  About,
  Config,
  ConnectionTest,
  DeviceAuth,
  LoadResult,
  MavenDetection,
  Overview,
  SSO,
  UpdateInfo,
  UpdateResult,
} from './types'
import { mock } from './mock'

// Bridge to the Go backend (internal/app.Service). Wails exposes bound
// methods on window.go.<package>.<Type>. Outside Wails — `npm run dev` in a
// plain browser — calls go to an in-memory mock so the UI can be worked on
// without AWS.

type Backend = {
  Overview(): Promise<Overview>
  DetectSSO(): Promise<SSO | null>
  DefaultConfig(): Promise<Config>
  SaveConfig(cfg: Config): Promise<Overview>
  ImportConfig(): Promise<Config | null>
  ExportConfig(): Promise<string>
  TestConnection(cfg: Config): Promise<ConnectionTest>
  DetectMaven(cfg: Config): Promise<MavenDetection>
  StartLogin(): Promise<DeviceAuth>
  WaitLogin(): Promise<Overview>
  OpenURL(url: string): Promise<void>
  RenewSession(): Promise<Overview>
  RefreshAccounts(): Promise<Overview>
  LoadProfile(accountId: string, role: string): Promise<LoadResult>
  ToggleFavorite(accountId: string): Promise<Overview>
  CopyText(text: string): Promise<void>
  CopyExport(): Promise<void>
  CopyCodeArtifactExport(): Promise<void>
  SetLanguage(lang: string): Promise<void>
  About(): Promise<About>
  CheckUpdate(manual: boolean): Promise<UpdateInfo>
  ApplyUpdate(): Promise<UpdateResult>
  RestartApp(): Promise<void>
}

declare global {
  interface Window {
    go?: { app?: { Service?: Backend } }
    runtime?: { EventsOn?: (name: string, cb: (...data: unknown[]) => void) => () => void }
  }
}

/**
 * Listens to an event the backend sends when something changes in the
 * background (app.EventOverview, app.EventRenewError). Returns the function
 * that stops listening; outside Wails there are no events.
 */
export function onBackendEvent(name: string, cb: (...data: unknown[]) => void): () => void {
  return window.runtime?.EventsOn?.(name, cb) ?? (() => {})
}

export const isMock = !window.go?.app?.Service

export const api: Backend = window.go?.app?.Service ?? mock

const LOGIN_REQUIRED = 'LOGIN_REQUIRED'

/** Errors from Go arrive as strings; normalise anything to a message. */
export function errorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err)
  return raw.startsWith(LOGIN_REQUIRED) ? raw.slice(LOGIN_REQUIRED.length).replace(/^:\s*/, '') : raw
}

/** True when the SSO session is gone and the user must log in again. */
export function isLoginRequired(err: unknown): boolean {
  const raw = err instanceof Error ? err.message : String(err)
  return raw.startsWith(LOGIN_REQUIRED)
}
