// Time helpers. Go's zero time ("0001-01-01T00:00:00Z") means "not set".
import { locale, t } from './i18n/index.svelte'

export function parseTime(value: string | null | undefined): Date | null {
  if (!value) return null
  const d = new Date(value)
  return Number.isNaN(d.getTime()) || d.getFullYear() < 2000 ? null : d
}

/** Minutes left until value, negative when past. */
export function minutesLeft(value: string, now = Date.now()): number {
  const d = parseTime(value)
  return d ? Math.floor((d.getTime() - now) / 60_000) : -1
}

/** "7h 12m", "18 min". */
export function remaining(value: string, now = Date.now()): string {
  const m = minutesLeft(value, now)
  if (m < 0) return t('time.expired')
  if (m < 60) return t('time.minutes', { m })
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, '0')}m`
}

/** "21:55". */
export function clock(value: string): string {
  const d = parseTime(value)
  return d ? d.toLocaleTimeString(locale(), { hour: '2-digit', minute: '2-digit' }) : ''
}

/** "22/09 · 08:14" (or "09/22 · 08:14 AM" in English). */
export function stamp(value: string): string {
  const d = parseTime(value)
  if (!d) return ''
  const date = d.toLocaleDateString(locale(), { day: '2-digit', month: '2-digit' })
  return `${date} · ${clock(value)}`
}

/** "just now", "today at 09:10", "yesterday at 18:40", "3 days ago". */
export function relative(value: string, now = new Date()): string {
  const d = parseTime(value)
  if (!d) return ''
  const diffMin = Math.floor((now.getTime() - d.getTime()) / 60_000)
  if (diffMin < 1) return t('time.now')
  const startOfDay = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime()
  const days = Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000)
  if (days === 0) return t('time.today', { time: clock(value) })
  if (days === 1) return t('time.yesterday', { time: clock(value) })
  return t('time.daysAgo', { days })
}
