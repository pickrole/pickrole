import type { About, Config, ConnectionTest, DeviceAuth, LoadResult, MavenDetection, Overview, SSO } from './types'
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
  SetLanguage(lang: string): Promise<void>
  About(): Promise<About>
}

declare global {
  interface Window {
    go?: { app?: { Service?: Backend } }
  }
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
