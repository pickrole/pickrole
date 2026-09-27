// UI translations. The language is state: components that call t() re-render
// when it changes. The backend is told the language too, so its messages
// (errors, warnings, dialog titles) match.
import { en } from './en'
import { ptBR } from './pt-BR'

export type Key = keyof typeof en
export type Messages = Record<Key, string>
export type Locale = 'en' | 'pt-BR'
/** The preference as stored in the config. */
export type LanguagePref = 'system' | Locale

const catalogs: Record<Locale, Messages> = { en, 'pt-BR': ptBR }

/** The language for a preference: "system" follows the OS, English otherwise. */
export function resolveLocale(pref: string | undefined): Locale {
  if (pref === 'en' || pref === 'pt-BR') return pref
  const system = (navigator.languages?.[0] ?? navigator.language ?? '').toLowerCase()
  return system.startsWith('pt') ? 'pt-BR' : 'en'
}

let current = $state<Locale>(resolveLocale('system'))

export function locale(): Locale {
  return current
}

export function setLocale(l: Locale) {
  current = l
  document.documentElement.lang = l
}

/** The message for key, with {name} placeholders replaced by params. */
export function t(key: Key, params?: Record<string, string | number>): string {
  let msg = catalogs[current][key] ?? en[key] ?? key
  if (params) for (const [name, value] of Object.entries(params)) msg = msg.split(`{${name}}`).join(String(value))
  return msg
}

/** Picks the ".one" or ".other" form of a counted message. */
export function tn(key: 'accounts.roles', count: number): string {
  return t(count === 1 ? `${key}.one` : `${key}.other`, { count })
}
