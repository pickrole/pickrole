<script lang="ts">
  import Icon from './Icon.svelte'
  import ProdTag from './ProdTag.svelte'
  import type { Account, Active, LoadResult } from './types'
  import { clock } from './time'
  import { t, tn } from './i18n/index.svelte'

  let {
    account,
    active,
    access,
    codeArtifactEnabled,
    busy,
    result,
    onLoad,
    onToggleFavorite,
    onCopy,
    onCopyExport,
    onCopyCodeArtifactExport,
  }: {
    account: Account
    active: Active | null
    access: Record<string, boolean>
    codeArtifactEnabled: boolean
    busy: string | null
    result: LoadResult | null
    onLoad: (role: string) => void
    onToggleFavorite: () => void
    onCopy: (text: string) => void
    onCopyExport: () => void
    /** Copies only `export CODEARTIFACT_AUTH_TOKEN=…`, which works without the AWS_* variables. */
    onCopyCodeArtifactExport: () => void
  } = $props()

  let confirming = $state<string | null>(null)

  const roles = $derived(account.roles ?? [])
  const isActive = (role: string) => active?.accountId === account.id && active.role === role
  const loadKey = (role: string) => `load:${account.id}/${role}`
  const isLoading = (role: string) => busy === loadKey(role)
  // Read-only roles (decided by the backend, on whole words) need no confirmation, even in production.
  const needsConfirm = (role: string) => account.production && !account.readOnlyRoles?.includes(role)
  const showResult = $derived(result !== null && result.active.accountId === account.id)

  function load(role: string) {
    if (needsConfirm(role) && confirming !== role && !isActive(role)) {
      confirming = role
      return
    }
    confirming = null
    onLoad(role)
  }

  function caLabel(role: string): { text: string; ok: boolean } | null {
    if (!codeArtifactEnabled) return null
    const known = access[`${account.id}/${role}`]
    if (known === undefined) return null
    return known ? { text: t('account.codeArtifactAvailable'), ok: true } : { text: t('account.codeArtifactNone'), ok: false }
  }
</script>

<main class="flex min-w-0 grow flex-col gap-3.5 overflow-y-auto px-6 py-[22px]">
  <div class="flex flex-col gap-1">
    <div class="flex items-center gap-2">
      <h1 class="truncate font-display text-[23px] font-semibold">{account.name}</h1>
      {#if account.production}<ProdTag label={t('account.production')} size="md" />{/if}
      <button
        class="flex size-[30px] shrink-0 items-center justify-center rounded-lg {account.favorite
          ? 'text-accent'
          : 'text-faint hover:text-muted'}"
        aria-label={account.favorite ? t('account.favoriteRemove') : t('account.favoriteAdd')}
        aria-pressed={account.favorite}
        onclick={onToggleFavorite}
      >
        <Icon name="star" size={16} />
      </button>
    </div>
    <div class="flex items-center gap-1.5">
      <span class="font-mono text-[12.5px] text-muted">{account.id}</span>
      <button
        class="flex size-[26px] items-center justify-center rounded-md text-faint hover:text-fg"
        aria-label={t('account.copyId')}
        onclick={() => onCopy(account.id)}
      >
        <Icon name="copy" size={13} />
      </button>
    </div>
  </div>

  {#if roles.length === 0}
    <p class="text-[13px] text-muted">{t('account.noRoles')}</p>
  {/if}

  <div class="flex flex-col gap-2">
    {#each roles as role (role)}
      {#if isActive(role)}
        <div class="flex flex-col gap-2.5 rounded-xl border border-ok-line bg-ok-card px-3.5 py-[13px]">
          <div class="flex items-center gap-2.5">
            <span class="flex grow items-center gap-2">
              <span class="text-[15px] font-semibold">{role}</span>
              <span class="rounded-full bg-ok-soft px-2 py-[2px] text-[11px] font-medium text-ok-fg"
                >{t('account.activeUntil', { time: clock(active!.expiresAt) })}</span
              >
            </span>
            <button
              class="h-[34px] rounded-lg border border-ok-line px-3.5 text-[13px]"
              onclick={() => onLoad(role)}
              disabled={busy !== null}>{isLoading(role) ? t('account.renewing') : t('account.renew')}</button
            >
          </div>
          <div class="flex flex-wrap gap-x-3.5 gap-y-1.5 text-xs text-muted">
            <span class="flex items-center gap-[5px]"
              ><span class="text-ok"><Icon name="check" size={12} stroke={3} /></span>{t('account.profile')}
              <span class="font-mono">{active!.profile}</span></span
            >
            {#if active!.codeArtifact}
              <span class="flex items-center gap-[5px]"
                ><span class="text-ok"><Icon name="check" size={12} stroke={3} /></span>{t('account.codeArtifactUntil', { time: clock(active!.codeArtifactExpiresAt) })}</span
              >
            {/if}
          </div>
        </div>
      {:else if confirming === role}
        <div class="flex flex-col gap-3 rounded-xl border border-prod bg-prod-card p-3.5">
          <span class="text-[15px] font-semibold">{role}</span>
          <div class="flex items-start gap-2.5 text-[13px] leading-normal text-prod-fg">
            <span class="mt-0.5 shrink-0"><Icon name="alert" size={16} /></span>
            <span
              >{#each t('account.confirmProd', { role: '\u0000' }).split('\u0000') as part, i (i)}{#if i > 0}<strong
                    >{role}</strong
                  >{/if}{part}{/each}</span
            >
          </div>
          <div class="flex justify-end gap-2">
            <button class="h-[34px] rounded-lg border border-line-strong px-3.5 text-[13px]" onclick={() => (confirming = null)}
              >{t('account.cancel')}</button
            >
            <button
              class="h-[34px] rounded-lg bg-prod px-3.5 text-[13px] font-semibold text-prod-ink"
              onclick={() => load(role)}
              disabled={busy !== null}>{isLoading(role) ? t('account.loading') : t('account.loadProd')}</button
            >
          </div>
        </div>
      {:else}
        {@const ca = caLabel(role)}
        <div class="flex items-center gap-2.5 rounded-xl border border-line bg-surface px-3.5 py-[13px]">
          <div class="flex grow flex-col gap-[3px]">
            <span class="text-[15px] font-semibold">{role}</span>
            {#if ca}
              <span class="text-[12.5px] {ca.ok ? 'text-muted' : 'text-faint'}">{ca.text}</span>
            {/if}
          </div>
          <button
            class="h-[34px] rounded-lg border border-line-strong px-3.5 text-[13px] hover:border-accent"
            onclick={() => load(role)}
            disabled={busy !== null}>{isLoading(role) ? t('account.loading') : t('account.load')}</button
          >
        </div>
      {/if}
    {/each}
  </div>

  {#if showResult && result}
    <div class="mt-auto flex flex-col gap-2">
      {#each result.warnings as w (w)}
        <div class="flex items-start gap-2.5 rounded-[10px] border border-warn-line bg-warn-soft px-3.5 py-2.5 text-[13px] text-warn-fg">
          <span class="mt-0.5 shrink-0"><Icon name="alert" size={15} /></span>{w}
        </div>
      {/each}
      <!-- The buttons move to a line of their own when the window is narrow. -->
      <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1 rounded-[10px] border border-line bg-surface px-3.5 py-[11px] text-[13px]">
        <span class="flex grow basis-[13rem] items-center gap-2.5 text-muted" title={result.written.join('\n')}>
          <span class="shrink-0 text-ok"><Icon name="check" size={15} stroke={2.6} /></span>{t('account.done')}
        </span>
        <span class="ml-auto flex shrink-0">
          <button
            class="h-[30px] rounded-[7px] px-2.5 text-[13px] font-medium whitespace-nowrap text-accent-soft-fg"
            title={result.active.codeArtifact ? t('account.copyExportWithToken') : undefined}
            onclick={onCopyExport}>{t('account.copyExport')}</button
          >
          {#if result.active.codeArtifact}
            <button
              class="h-[30px] rounded-[7px] px-2.5 text-[13px] font-medium whitespace-nowrap text-accent-soft-fg"
              title={t('account.copyTokenHint')}
              onclick={onCopyCodeArtifactExport}>{t('account.copyToken')}</button
            >
          {/if}
        </span>
      </div>
    </div>
  {/if}
</main>
