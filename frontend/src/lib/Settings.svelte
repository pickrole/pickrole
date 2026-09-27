<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Icon from './Icon.svelte'
  import { api, errorMessage } from './api'
  import type { Config, ConnectionTest, Overview, SSO } from './types'
  import { t, tn, type Key } from './i18n/index.svelte'

  let {
    initial,
    onboarding,
    onSaved,
    onCancel,
  }: {
    initial: Config
    onboarding: boolean
    onSaved: (o: Overview) => void
    onCancel?: () => void
  } = $props()

  // Edit a copy of the config as it was when the screen opened; nothing
  // changes until Save.
  let cfg = $state<Config>(untrack(() => structuredClone($state.snapshot(initial))))
  let tab = $state<'sso' | 'codeartifact' | 'network' | 'prefs'>('sso')
  let detected = $state<SSO | null>(null)
  let error = $state('')
  let notice = $state('')
  let saving = $state(false)
  let testing = $state(false)
  let test = $state<ConnectionTest | null>(null)
  let testError = $state('')

  onMount(async () => {
    detected = await api.DetectSSO()
  })

  const regions = [
    'us-east-1',
    'us-east-2',
    'us-west-1',
    'us-west-2',
    'sa-east-1',
    'ca-central-1',
    'eu-west-1',
    'eu-west-2',
    'eu-central-1',
    'eu-north-1',
    'ap-south-1',
    'ap-southeast-1',
    'ap-southeast-2',
    'ap-northeast-1',
  ]

  async function save() {
    saving = true
    error = ''
    try {
      onSaved(await api.SaveConfig($state.snapshot(cfg)))
    } catch (e) {
      error = errorMessage(e)
    } finally {
      saving = false
    }
  }

  async function importConfig() {
    error = ''
    try {
      const imported = await api.ImportConfig()
      if (imported) {
        cfg = imported
        notice = t('settings.imported')
      }
    } catch (e) {
      error = errorMessage(e)
    }
  }

  async function exportConfig() {
    error = ''
    try {
      const path = await api.ExportConfig()
      if (path) notice = t('settings.exported', { path })
    } catch (e) {
      error = errorMessage(e)
    }
  }

  async function testConnection() {
    testing = true
    test = null
    testError = ''
    try {
      test = await api.TestConnection($state.snapshot(cfg))
    } catch (e) {
      testError = errorMessage(e)
    } finally {
      testing = false
    }
  }

  function testMessage(r: ConnectionTest): string {
    if (!r.proxy) return t('settings.test.direct', { target: r.target })
    const source = t(`settings.source.${r.source}` as Key)
    return t('settings.test.proxy', { target: r.target, proxy: r.proxy, source })
  }

  function useDetected() {
    if (detected) cfg.sso = { ...detected }
  }

  const input =
    'h-[38px] rounded-lg border border-line bg-surface px-3 font-mono text-[13px] text-fg outline-none focus:border-accent-line'
  const label = 'text-[13px] font-medium text-muted'
</script>

{#snippet toggle(checked: boolean, labelText: string, onToggle: () => void)}
  <button
    role="switch"
    aria-checked={checked}
    aria-label={labelText}
    class="flex h-[26px] w-11 shrink-0 rounded-full p-[3px] {checked ? 'justify-end bg-accent' : 'justify-start bg-line-strong'}"
    onclick={onToggle}
  >
    <span class="size-5 rounded-full {checked ? 'bg-accent-ink' : 'bg-faint'}"></span>
  </button>
{/snippet}

<div class="flex h-full flex-col">
  <header class="flex h-14 shrink-0 items-center gap-2.5 border-b border-line px-3.5">
    {#if onCancel}
      <button
        class="flex size-9 items-center justify-center rounded-lg border border-line"
        aria-label={t('settings.back')}
        onclick={onCancel}
      >
        <Icon name="back" size={16} stroke={2.2} />
      </button>
    {:else}
      <span class="w-1.5"></span>
    {/if}
    <span class="grow font-display text-[17px] font-semibold">{onboarding ? t('settings.setup') : t('settings.title')}</span>
    <button class="flex h-9 items-center gap-[7px] rounded-lg border border-line px-3 text-[13px]" onclick={importConfig}>
      <Icon name="download" size={14} />{t('settings.import')}
    </button>
    {#if !onboarding}
      <button class="flex h-9 items-center gap-[7px] rounded-lg border border-line px-3 text-[13px]" onclick={exportConfig}>
        <Icon name="upload" size={14} />{t('settings.export')}
      </button>
    {/if}
  </header>

  <nav aria-label={t('settings.sections')} class="flex h-[46px] shrink-0 items-stretch gap-[22px] border-b border-line px-5">
    {#each [['sso', t('settings.tab.sso')], ['codeartifact', t('settings.tab.codeartifact')], ['network', t('settings.tab.network')], ['prefs', t('settings.tab.prefs')]] as [id, name] (id)}
      <button
        class="border-b-2 text-sm {tab === id ? 'border-accent font-semibold text-fg' : 'border-transparent text-muted'}"
        aria-current={tab === id ? 'page' : undefined}
        onclick={() => (tab = id as typeof tab)}>{name}</button
      >
    {/each}
  </nav>

  <div class="flex min-h-0 grow flex-col gap-[18px] overflow-y-auto px-6 py-5">
    {#if tab === 'sso'}
      {#if detected && detected.startUrl !== cfg.sso.startUrl}
        <div class="flex items-center gap-3.5 rounded-xl border border-accent-line bg-accent-soft px-4 py-3">
          <span class="text-accent-soft-fg"><Icon name="info" size={18} /></span>
          <span class="grow text-[13px] leading-normal text-accent-soft-fg">
            {t('settings.detected', { file: '~/.aws/config', url: detected.startUrl })}
          </span>
          <button class="h-8 rounded-lg border border-accent-line px-3 text-[13px] text-accent-soft-fg" onclick={useDetected}
            >{t('settings.useDetected')}</button
          >
        </div>
      {/if}
      <p class="text-[13px] leading-normal text-muted">
        {t('settings.ssoIntro', { import: t('settings.import') })}
      </p>
      <div class="grid grid-cols-3 gap-4">
        <label class="col-span-2 flex flex-col gap-1.5">
          <span class={label}>{t('settings.startUrl')}</span>
          <input class={input} bind:value={cfg.sso.startUrl} placeholder={t('settings.startUrlPlaceholder')} />
        </label>
        <label class="flex flex-col gap-1.5">
          <span class={label}>{t('settings.ssoRegion')}</span>
          <select class="{input} font-sans" bind:value={cfg.sso.region}>
            {#each regions as r (r)}<option value={r}>{r}</option>{/each}
          </select>
        </label>
      </div>
    {:else if tab === 'codeartifact'}
      <div class="flex items-center gap-4">
        <div class="flex grow flex-col gap-[3px]">
          <span class="text-[15px] font-semibold">{t('settings.useCodeArtifact')}</span>
          <span class="text-[13px] text-muted">{t('settings.codeArtifactIntro')}</span>
        </div>
        {@render toggle(cfg.codeArtifact.enabled, t('settings.useCodeArtifact'), () => (cfg.codeArtifact.enabled = !cfg.codeArtifact.enabled))}
      </div>
      {#if cfg.codeArtifact.enabled}
        <div class="grid grid-cols-2 gap-x-4 gap-y-3.5">
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.domain')}</span>
            <input class={input} bind:value={cfg.codeArtifact.domain} placeholder={t('settings.domainPlaceholder')} />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.domainOwner')}</span>
            <input class={input} bind:value={cfg.codeArtifact.domainOwner} placeholder={t('settings.domainOwnerPlaceholder')} inputmode="numeric" />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.region')}</span>
            <select class="{input} font-sans" bind:value={cfg.codeArtifact.region}>
              {#each regions as r (r)}<option value={r}>{r}</option>{/each}
            </select>
          </label>
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.repository')}</span>
            <input class={input} bind:value={cfg.codeArtifact.repository} placeholder={t('settings.repositoryPlaceholder')} />
          </label>
        </div>
        <div class="flex flex-col gap-2">
          <span class={label}>{t('settings.tools')}</span>
          <div class="flex gap-2">
            <button
              aria-pressed={cfg.codeArtifact.tools.maven.enabled}
              class="flex h-[34px] items-center gap-1.5 rounded-full border px-[13px] text-[13px] font-medium {cfg.codeArtifact.tools
                .maven.enabled
                ? 'border-accent bg-accent-soft text-accent-soft-fg'
                : 'border-line-strong text-muted'}"
              onclick={() => (cfg.codeArtifact.tools.maven.enabled = !cfg.codeArtifact.tools.maven.enabled)}
            >
              {#if cfg.codeArtifact.tools.maven.enabled}<Icon name="check" size={13} stroke={2.8} />{/if}Maven
            </button>
            {#each ['Gradle', 'npm', 'pip'] as tool (tool)}
              <button
                class="h-[34px] rounded-full border border-line px-[13px] text-[13px] text-faint"
                disabled
                title={t('settings.soon')}>{tool}</button
              >
            {/each}
          </div>
        </div>
        {#if cfg.codeArtifact.tools.maven.enabled}
          <div class="grid grid-cols-2 gap-x-4 gap-y-3 rounded-[10px] border border-line bg-surface px-4 py-3.5">
            <label class="flex flex-col gap-1.5">
              <span class={label}>{t('settings.mavenServerId')}</span>
              <input class="{input} bg-bg" bind:value={cfg.codeArtifact.tools.maven.serverId} />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class={label}>{t('settings.file')}</span>
              <input class="{input} bg-bg" bind:value={cfg.codeArtifact.tools.maven.settingsPath} />
            </label>
            <span class="col-span-2 text-xs text-faint"
              >{t('settings.mavenNote')}</span
            >
          </div>
        {/if}
      {/if}
    {:else if tab === 'network'}
      <p class="text-[13px] leading-normal text-muted">{t('settings.networkIntro')}</p>
      <fieldset class="flex flex-col gap-2.5">
        <legend class="mb-2.5 text-[15px] font-semibold">{t('settings.proxy')}</legend>
        <label class="flex items-start gap-2.5 text-sm">
          <input type="radio" class="mt-0.5 size-4 accent-accent" bind:group={cfg.proxy.mode} value="system" />
          <span class="flex flex-col gap-0.5">
            <span>{t('settings.proxy.system')} <span class="text-[13px] text-faint">{t('settings.recommended')}</span></span>
            <span class="text-xs text-faint">{t('settings.proxy.systemNote')}</span>
          </span>
        </label>
        <label class="flex items-center gap-2.5 text-sm">
          <input type="radio" class="size-4 accent-accent" bind:group={cfg.proxy.mode} value="manual" />
          {t('settings.proxy.manual')}
        </label>
        <label class="flex items-center gap-2.5 text-sm">
          <input type="radio" class="size-4 accent-accent" bind:group={cfg.proxy.mode} value="none" />
          {t('settings.proxy.none')}
        </label>
      </fieldset>
      {#if cfg.proxy.mode === 'manual'}
        <div class="flex flex-col gap-3 rounded-[10px] border border-line bg-surface px-4 py-3.5">
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.proxyUrl')}</span>
            <input class="{input} bg-bg" bind:value={cfg.proxy.url} placeholder={t('settings.proxyUrlPlaceholder')} />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class={label}>{t('settings.noProxy')}</span>
            <input class="{input} bg-bg" bind:value={cfg.proxy.noProxy} placeholder={t('settings.noProxyPlaceholder')} />
            <span class="text-xs text-faint">{t('settings.noProxyNote')}</span>
          </label>
        </div>
      {/if}
      <div class="flex flex-col gap-2">
        <button
          class="flex h-9 w-fit items-center gap-[7px] rounded-lg border border-line-strong px-3.5 text-[13px] font-medium"
          onclick={testConnection}
          disabled={testing}>{testing ? t('settings.testing') : t('settings.testConnection')}</button
        >
        <div aria-live="polite" class="flex flex-col gap-1 text-[13px] leading-normal">
          {#if testError}
            <span class="text-prod-fg">{testError}</span>
          {:else if test}
            {#if test.error}
              <span class="text-prod-fg">{test.error}</span>
            {:else}
              <span class="flex items-center gap-1.5 text-fg"><Icon name="check" size={14} stroke={2.6} />{testMessage(test)}</span>
            {/if}
            {#if test.note}<span class="text-muted">{test.note}</span>{/if}
          {/if}
        </div>
      </div>
    {:else}
      <fieldset class="flex flex-col gap-2.5">
        <legend class="mb-2.5 text-[15px] font-semibold">{t('settings.whereCredentials')}</legend>
        <label class="flex items-center gap-2.5 text-sm">
          <input type="radio" class="size-4 accent-accent" bind:group={cfg.preferences.profileMode} value="default" />
          {#each t('settings.profileDefault', { name: '\u0000' }).split('\u0000') as part, i (i)}{#if i > 0}<span
                class="font-mono text-[13px] text-muted">default</span
              >{/if}{part}{/each}
          <span class="text-[13px] text-faint">{t('settings.recommended')}</span>
        </label>
        <label class="flex items-center gap-2.5 text-sm">
          <input type="radio" class="size-4 accent-accent" bind:group={cfg.preferences.profileMode} value="named" />
          {t('settings.profileNamed')}
          <span class="text-[13px] text-faint">{t('settings.useAwsProfile')}</span>
        </label>
      </fieldset>
      <div class="h-px bg-line"></div>
      <label class="flex items-center gap-4">
        <span class="grow text-sm">{t('settings.theme')}</span>
        <select class="{input} w-40 font-sans" bind:value={cfg.preferences.theme}>
          <option value="system">{t('settings.theme.system')}</option>
          <option value="dark">{t('settings.theme.dark')}</option>
          <option value="light">{t('settings.theme.light')}</option>
        </select>
      </label>
      <label class="flex items-center gap-4">
        <span class="grow text-sm">{t('settings.language')}</span>
        <select class="{input} w-40 font-sans" bind:value={cfg.preferences.language}>
          <option value="system">{t('settings.language.system')}</option>
          <option value="en">English</option>
          <option value="pt-BR">Português (Brasil)</option>
        </select>
      </label>
      <label class="flex flex-col gap-1.5">
        <span class={label}>{t('settings.prodPattern')}</span>
        <input class={input} bind:value={cfg.preferences.prodPattern} />
        <span class="text-xs text-faint">{t('settings.prodPatternNote')}</span>
      </label>
    {/if}
  </div>

  <footer class="flex h-16 shrink-0 items-center gap-2.5 border-t border-line px-6">
    <span class="grow truncate text-[13px] {error ? 'text-prod-fg' : 'text-muted'}" role={error ? 'alert' : undefined}
      >{error || notice}</span
    >
    {#if onCancel}
      <button class="h-[38px] rounded-lg border border-line-strong px-4 text-sm" onclick={onCancel}>{t('settings.cancel')}</button>
    {/if}
    <button
      class="h-[38px] rounded-lg bg-accent px-[18px] text-sm font-semibold text-accent-ink"
      onclick={save}
      disabled={saving}>{onboarding ? t('settings.saveAndSignIn') : t('settings.save')}</button
    >
  </footer>
</div>
