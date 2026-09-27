<script lang="ts">
  import Icon from './Icon.svelte'
  import ProdTag from './ProdTag.svelte'
  import type { Active, Session } from './types'
  import { minutesLeft, remaining } from './time'
  import { t, tn } from './i18n/index.svelte'

  let {
    session,
    active,
    production,
    now,
    busy,
    onRefresh,
    onSettings,
    onRenew,
    onLogin,
    onAbout,
    onShowActive,
  }: {
    session: Session
    active: Active | null
    production: boolean
    now: number
    busy: string | null
    onRefresh: () => void
    onSettings: () => void
    onRenew: () => void
    onLogin: () => void
    onAbout: () => void
    onShowActive: () => void
  } = $props()

  const left = $derived(session.loggedIn ? minutesLeft(session.expiresAt, now) : -1)
  const state = $derived(left < 0 ? 'expired' : left < 30 ? 'warn' : 'ok')
  // Role credentials expire (usually after an hour) even while the SSO session lasts.
  const activeExpired = $derived(active !== null && minutesLeft(active.expiresAt, now) < 0)
</script>

<header class="flex h-14 shrink-0 items-center gap-2 border-b border-line pr-3.5 pl-3">
  {#if active}
    <button
      class="flex min-w-0 items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-surface"
      title={t('header.activeProfile')}
      onclick={onShowActive}
    >
      <span class="size-[7px] shrink-0 rounded-full {activeExpired ? 'bg-faint' : 'bg-ok'}"></span>
      <span class="truncate text-sm font-semibold">{active.accountName}</span>
      <span class="shrink-0 text-sm text-muted">/ {active.role}</span>
      {#if production}<ProdTag />{/if}
      {#if activeExpired}<span class="shrink-0 text-xs text-faint">{t('header.profileExpired')}</span>{/if}
    </button>
  {:else}
    <span class="px-2 text-sm text-faint">{t('header.noActiveProfile')}</span>
  {/if}
  <span class="grow"></span>

  {#if state === 'ok'}
    <span
      class="flex items-center gap-[7px] rounded-full border border-line bg-surface px-[11px] py-[5px] text-[12.5px] text-muted"
      title={t('header.session')}
    >
      <span class="size-[7px] rounded-full bg-ok"></span>SSO · {remaining(session.expiresAt, now)}
    </span>
  {:else if state === 'warn'}
    <span
      class="flex items-center gap-2 rounded-full border border-warn-line bg-warn-soft py-[3px] pr-1 pl-[11px] text-[12.5px] text-warn-fg"
    >
      <span class="size-[7px] rounded-full bg-warn"></span>{t('header.sessionExpiresIn', { time: remaining(session.expiresAt, now) })}
      <button
        class="h-[26px] rounded-full bg-warn px-2.5 text-xs font-semibold text-warn-ink"
        onclick={onRenew}
        disabled={busy === 'renew'}>{t('header.renew')}</button
      >
    </span>
  {:else}
    <span
      class="flex items-center gap-2 rounded-full border border-line bg-surface py-[3px] pr-1 pl-[11px] text-[12.5px] text-muted"
    >
      <span class="size-[7px] rounded-full bg-faint"></span>{t('header.sessionExpired')}
      <button class="h-[26px] rounded-full bg-accent px-2.5 text-xs font-semibold text-accent-ink" onclick={onLogin}
        >{t('header.signIn')}</button
      >
    </span>
  {/if}

  <button
    class="flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
    aria-label={t('header.refresh')}
    title={t('header.refresh')}
    onclick={onRefresh}
    disabled={busy === 'refresh'}
  >
    <span class={busy === 'refresh' ? 'animate-[spin_0.9s_linear_infinite]' : ''}>
      <Icon name="refresh" size={15} />
    </span>
  </button>
  <button
    class="flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
    aria-label={t('about.title')}
    title={t('about.title')}
    onclick={onAbout}
  >
    <Icon name="info" size={15} />
  </button>
  <button
    class="flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
    aria-label={t('header.settings')}
    title={t('header.settings')}
    onclick={onSettings}
  >
    <Icon name="sliders" size={15} />
  </button>
</header>
