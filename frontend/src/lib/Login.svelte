<script lang="ts">
  import { onMount } from 'svelte'
  import Icon from './Icon.svelte'
  import { api, errorMessage } from './api'
  import type { DeviceAuth, Overview } from './types'
  import { t, tn } from './i18n/index.svelte'

  let { firstRun, onDone, onCancel }: { firstRun: boolean; onDone: (o: Overview) => void; onCancel?: () => void } =
    $props()

  let auth = $state<DeviceAuth | null>(null)
  let step = $state<'starting' | 'waiting' | 'accounts' | 'error'>('starting')
  let error = $state('')
  let copied = $state(false)

  async function begin() {
    step = 'starting'
    error = ''
    try {
      auth = await api.StartLogin()
      step = 'waiting'
      const overview = await api.WaitLogin()
      step = 'accounts'
      onDone(overview)
    } catch (e) {
      error = errorMessage(e)
      step = 'error'
    }
  }

  async function copy() {
    if (!auth) return
    await api.CopyText(auth.userCode)
    copied = true
    setTimeout(() => (copied = false), 1500)
  }

  onMount(begin)

  type Item = { label: string; state: 'done' | 'current' | 'todo' }
  const items = $derived<Item[]>([
    { label: t('login.step.config'), state: 'done' },
    {
      label: step === 'accounts' ? t('login.step.authorized') : t('login.step.waiting'),
      state: step === 'accounts' ? 'done' : 'current',
    },
    { label: t('login.step.accounts'), state: step === 'accounts' ? 'current' : 'todo' },
  ])
</script>

<div class="flex grow items-center justify-center">
  <div class="flex w-[420px] flex-col gap-5">
    <div class="flex flex-col gap-2">
      <span class="text-xs font-semibold tracking-[0.08em] text-accent-soft-fg uppercase"
        >{firstRun ? t('login.firstRun') : t('login.again')}</span
      >
      <h1 class="font-display text-[26px] leading-tight font-semibold">{t('login.title')}</h1>
      <p class="text-sm leading-relaxed text-muted">
        {t('login.body')}
      </p>
    </div>

    <div class="flex items-center justify-between rounded-xl border border-line bg-surface px-[18px] py-4">
      <span class="font-mono text-[28px] font-medium tracking-[0.12em] select-text">{auth?.userCode ?? '····-····'}</span>
      <button
        class="flex size-10 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
        aria-label={t('login.copyCode')}
        onclick={copy}
        disabled={!auth}
      >
        <Icon name={copied ? 'check' : 'copy'} size={16} />
      </button>
    </div>

    <div class="flex gap-2.5">
      <button
        class="flex h-11 items-center gap-2 rounded-[10px] bg-accent px-[18px] text-sm font-semibold text-accent-ink"
        onclick={() => auth && api.OpenURL(auth.verificationUrl)}
        disabled={!auth}
      >
        {t('login.openBrowser')} <Icon name="external" size={15} stroke={2.2} />
      </button>
      {#if onCancel}
        <button class="h-11 rounded-[10px] border border-line-strong px-4 text-sm" onclick={onCancel}>{t('login.cancel')}</button>
      {/if}
    </div>

    {#if step === 'error'}
      <div class="flex items-start gap-2.5 rounded-[10px] border border-prod bg-prod-card px-3.5 py-3 text-[13px] text-prod-fg">
        <span class="mt-0.5 shrink-0"><Icon name="alert" size={15} /></span>
        <span class="grow">{error}</span>
        <button class="font-semibold underline" onclick={begin}>{t('login.retry')}</button>
      </div>
    {:else}
      <div class="flex flex-col gap-[11px] pt-1">
        {#each items as it (it.label)}
          <div class="flex items-center gap-3 text-sm">
            {#if it.state === 'done'}
              <span class="flex size-[22px] items-center justify-center rounded-full bg-ok-soft text-ok"
                ><Icon name="check" size={13} stroke={3} /></span
              >
              <span class="text-muted">{it.label}</span>
            {:else if it.state === 'current'}
              <span
                class="m-0.5 box-border size-[18px] animate-[spin_0.9s_linear_infinite] rounded-full border-2 border-accent-soft border-t-accent"
              ></span>
              <span class="font-medium">{it.label}</span>
            {:else}
              <span class="m-0.5 box-border size-[18px] rounded-full border-2 border-line"></span>
              <span class="text-faint">{it.label}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    {#if firstRun}
      <p class="border-t border-line pt-3.5 text-[13px] leading-normal text-faint">
        {t('login.onlyOnce')}
      </p>
    {/if}
  </div>
</div>
