<script lang="ts">
  import { onMount } from 'svelte'
  import About from './lib/About.svelte'
  import AccountList from './lib/AccountList.svelte'
  import AccountPanel from './lib/AccountPanel.svelte'
  import Header from './lib/Header.svelte'
  import Icon from './lib/Icon.svelte'
  import Login from './lib/Login.svelte'
  import Settings from './lib/Settings.svelte'
  import Splash from './lib/Splash.svelte'
  import StartPanel from './lib/StartPanel.svelte'
  import { fade } from 'svelte/transition'
  import { api, errorMessage, isLoginRequired, isMock } from './lib/api'
  import { tagline } from './lib/brand'
  import { resolveLocale, setLocale, t } from './lib/i18n/index.svelte'
  import { stamp } from './lib/time'
  import type { Config, LoadResult, Overview, Recent } from './lib/types'

  type View = 'loading' | 'setup' | 'login' | 'main' | 'settings'

  let overview = $state<Overview | null>(null)
  let view = $state<View>('loading')
  let setupConfig = $state<Config | null>(null)
  let selectedId = $state<string | null>(null)
  let busy = $state<string | null>(null)
  let result = $state<LoadResult | null>(null)
  let error = $state('')
  let now = $state(Date.now())
  let splash = $state(true)
  let showAbout = $state(false)
  // A load that failed because the session expired; retried after login.
  let pending: { accountId: string; role: string } | null = null

  const selected = $derived(overview?.accounts.find((a) => a.id === selectedId) ?? null)

  // Language: "system" follows the OS. The backend is told too, so its
  // messages (errors, warnings, dialog titles) use the same language.
  $effect(() => {
    const loc = resolveLocale(overview?.config.preferences.language)
    setLocale(loc)
    api.SetLanguage(loc)
  })

  // Theme: dark by default (also while the config loads); "system" follows
  // the OS.
  $effect(() => {
    const theme = overview?.config.preferences.theme ?? 'dark'
    if (theme === 'system') delete document.documentElement.dataset.theme
    else document.documentElement.dataset.theme = theme
  })

  function route(o: Overview): View {
    if (!o.configured) return 'setup'
    if (o.accounts.length === 0) return 'login'
    return 'main'
  }

  onMount(() => {
    const tick = setInterval(() => (now = Date.now()), 30_000)
    ;(async () => {
      try {
        overview = await api.Overview()
        if (!overview.configured) setupConfig = await api.DefaultConfig()
        view = route(overview)
      } catch (e) {
        error = errorMessage(e)
        view = 'main'
      }
    })()
    return () => clearInterval(tick)
  })

  async function run(key: string, fn: () => Promise<void>) {
    busy = key
    error = ''
    try {
      await fn()
    } catch (e) {
      if (isLoginRequired(e)) view = 'login'
      else error = errorMessage(e)
    } finally {
      busy = null
    }
  }

  const refresh = () =>
    run('refresh', async () => {
      overview = await api.RefreshAccounts()
    })

  const renew = () =>
    run('renew', async () => {
      overview = await api.RenewSession()
    })

  function load(accountId: string, role: string) {
    return run(`load:${accountId}/${role}`, async () => {
      try {
        result = await api.LoadProfile(accountId, role)
      } catch (e) {
        if (isLoginRequired(e)) pending = { accountId, role }
        throw e
      }
      selectedId = accountId
      overview = await api.Overview()
    })
  }

  function continueWith(r: Recent) {
    selectedId = r.accountId
    const account = overview?.accounts.find((a) => a.id === r.accountId)
    // Production write roles still go through the confirmation in the panel.
    if (account?.production && !account.readOnlyRoles?.includes(r.role)) return
    load(r.accountId, r.role)
  }

  async function afterLogin(o: Overview) {
    overview = o
    view = 'main'
    if (pending) {
      const { accountId, role } = pending
      pending = null
      await load(accountId, role)
    }
  }

  const toggleFavorite = (id: string) =>
    run('favorite', async () => {
      overview = await api.ToggleFavorite(id)
    })

  const copy = (text: string) =>
    run('copy', async () => {
      await api.CopyText(text)
    })

  const copyExport = () =>
    run('copy', async () => {
      await api.CopyExport()
    })

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && !showAbout && view === 'main' && (e.target as HTMLElement).tagName !== 'INPUT') selectedId = null
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="flex h-full flex-col overflow-hidden bg-bg text-fg">
  {#if splash}
    <Splash lines={tagline()} ready={view !== 'loading'} onDone={() => (splash = false)} />
  {:else}
    <div class="flex min-h-0 grow flex-col" in:fade={{ duration: 200 }}>
      {#if view === 'loading'}
        <div class="m-auto size-6 animate-[spin_0.9s_linear_infinite] rounded-full border-2 border-accent-soft border-t-accent"></div>
      {:else if view === 'setup' && setupConfig}
        <Settings
          initial={setupConfig}
          onboarding
          onSaved={(o) => {
            overview = o
            view = 'login'
          }}
        />
      {:else if view === 'settings' && overview}
        <Settings
          initial={overview.config}
          onboarding={false}
          onSaved={(o) => {
            overview = o
            view = route(o)
          }}
          onCancel={() => (view = overview ? route(overview) : 'main')}
        />
      {:else if view === 'login' && overview}
        <header class="flex h-14 shrink-0 items-center justify-between border-b border-line px-3.5">
          <!-- Back to the accounts, or on first use to the settings, where a wrong start URL or region is fixed. -->
          <button
            class="flex size-9 items-center justify-center rounded-lg border border-line"
            aria-label={t('settings.back')}
            title={t('settings.back')}
            onclick={() => (view = overview && overview.accounts.length > 0 ? 'main' : 'settings')}
          >
            <Icon name="back" size={16} stroke={2.2} />
          </button>
          <button
            class="flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
            aria-label={t('about.title')}
            title={t('about.title')}
            onclick={() => (showAbout = true)}
          >
            <Icon name="info" size={15} />
          </button>
        </header>
        <Login
          firstRun={overview.accounts.length === 0}
          onDone={afterLogin}
          onCancel={overview.accounts.length > 0 ? () => (view = 'main') : undefined}
        />
      {:else if overview}
        <Header
          session={overview.session}
          active={overview.active}
          production={overview.accounts.find((a) => a.id === overview?.active?.accountId)?.production ?? false}
          {now}
          {busy}
          onRefresh={refresh}
          onSettings={() => (view = 'settings')}
          onRenew={renew}
          onLogin={() => (view = 'login')}
          onAbout={() => (showAbout = true)}
          onShowActive={() => (selectedId = overview?.active?.accountId ?? null)}
        />
        <div class="flex min-h-0 grow">
          <AccountList
            accounts={overview.accounts}
            {selectedId}
            activeId={overview.active?.accountId ?? null}
            onSelect={(id) => {
              selectedId = id
              if (result?.active.accountId !== id) result = null
            }}
          />
          {#if selected}
            {#key selected.id}
              <AccountPanel
                account={selected}
                active={overview.active}
                access={overview.codeArtifactAccess}
                codeArtifactEnabled={overview.config.codeArtifact.enabled}
                {busy}
                {result}
                onLoad={(role) => load(selected.id, role)}
                onToggleFavorite={() => toggleFavorite(selected.id)}
                onCopy={copy}
                onCopyExport={copyExport}
              />
            {/key}
          {:else}
            <StartPanel recents={overview.recents} {busy} onContinue={continueWith} />
          {/if}
        </div>
        <footer class="flex h-8 shrink-0 items-center justify-between gap-4 border-t border-line px-5 text-xs text-faint">
          {#if error}
            <span class="flex min-w-0 items-center gap-1.5 truncate text-prod-fg" role="alert">
              <Icon name="alert" size={13} />{error}
            </span>
          {:else}
            <span></span>
          {/if}
          <span class="shrink-0">
            {[isMock ? t('footer.sampleData') : '', overview.cacheUpdatedAt ? t('footer.cache', { time: stamp(overview.cacheUpdatedAt) }) : '']
              .filter(Boolean)
              .join(' · ')}
          </span>
        </footer>
      {/if}
    </div>
  {/if}
  {#if showAbout}
    <About onClose={() => (showAbout = false)} />
  {/if}
</div>
