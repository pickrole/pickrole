<script lang="ts">
  // "New version" dialog (docs/adr/0024): installs the update when this
  // installation allows it, otherwise shows the command or the release page.
  import { onMount } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import Icon from './Icon.svelte'
  import { api, errorMessage } from './api'
  import { t } from './i18n/index.svelte'
  import type { UpdateInfo } from './types'

  let { info, current, onClose }: { info: UpdateInfo; current: string; onClose: () => void } = $props()

  let step = $state<'ready' | 'working' | 'terminal' | 'manual' | 'error'>('ready')
  let error = $state('')
  let command = $state('')
  let reason = $state('')
  let copied = $state(false)
  let closeButton = $state<HTMLButtonElement>()

  onMount(() => closeButton?.focus())

  async function install() {
    step = 'working'
    error = ''
    try {
      const res = await api.ApplyUpdate()
      // On success PickRole restarts. With pbrun the install runs in a
      // terminal, and PickRole restarts once it's done; the command is kept
      // in case the terminal didn't show up.
      if (res.manualCommand) {
        command = res.manualCommand
        reason = res.reason
        step = res.terminal ? 'terminal' : 'manual'
      }
    } catch (e) {
      error = errorMessage(e)
      step = 'error'
    }
  }

  async function copy() {
    await api.CopyText(command)
    copied = true
    setTimeout(() => (copied = false), 1500)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && step !== 'working') {
      e.preventDefault()
      onClose()
    }
  }
</script>

<svelte:window onkeydown={onKey} />

{#snippet commandBox()}
  <div class="flex items-center gap-2 rounded-[10px] border border-line bg-surface px-3 py-2">
    <code class="grow font-mono text-[12px] break-all select-text">{command}</code>
    <button
      class="flex size-8 shrink-0 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
      aria-label={t('update.copyCommand')}
      title={t('update.copyCommand')}
      onclick={copy}
    >
      <Icon name={copied ? 'check' : 'copy'} size={14} />
    </button>
  </div>
{/snippet}

<div class="fixed inset-0 z-50 flex items-center justify-center p-6" transition:fade={{ duration: 150 }}>
  <button
    class="absolute inset-0 bg-bg/70 backdrop-blur-[2px]"
    aria-label={t('update.close')}
    tabindex="-1"
    onclick={() => step !== 'working' && onClose()}
  ></button>
  <div
    role="dialog"
    aria-modal="true"
    aria-labelledby="update-title"
    class="relative flex w-full max-w-[440px] flex-col gap-4 rounded-2xl border border-line-strong bg-bg px-6 py-5"
    transition:scale={{ start: 0.96, duration: 180 }}
  >
    <div class="flex items-start gap-3">
      <div class="flex grow flex-col gap-1">
        <h2 id="update-title" class="font-display text-lg font-semibold">
          {info.security ? t('update.securityTitle') : t('update.title')}
        </h2>
        <p class="font-mono text-[13px] text-muted">{current} → <span class="text-accent-soft-fg">{info.version}</span></p>
      </div>
      <button
        bind:this={closeButton}
        class="flex size-8 shrink-0 items-center justify-center rounded-lg text-muted hover:text-fg"
        aria-label={t('update.close')}
        onclick={onClose}
        disabled={step === 'working'}
      >
        <Icon name="close" size={15} />
      </button>
    </div>

    <button class="flex w-fit items-center gap-1 text-[13px] text-accent-soft-fg hover:underline" onclick={() => api.OpenURL(info.url)}>
      {t('update.notes')}<Icon name="external" size={12} />
    </button>

    {#if info.security}
      <p class="rounded-[10px] border border-warn-line bg-warn-soft px-3.5 py-2.5 text-[13px] leading-normal text-warn-fg">
        {t('update.securityNote')}
      </p>
    {/if}

    <div aria-live="polite" class="flex flex-col gap-2 text-[13px] leading-normal">
      {#if step === 'working'}
        <span class="flex items-center gap-2.5 text-muted">
          <span
            class="box-border size-4 animate-[spin_0.9s_linear_infinite] rounded-full border-2 border-accent-soft border-t-accent"
          ></span>{t('update.working')}
        </span>
      {:else if step === 'terminal'}
        <span class="flex items-center gap-2.5">
          <span
            class="box-border size-4 shrink-0 animate-[spin_0.9s_linear_infinite] rounded-full border-2 border-accent-soft border-t-accent"
          ></span>{t('update.inTerminal')}
        </span>
        <span class="text-xs text-faint">{t('update.inTerminalFallback')}</span>
        {@render commandBox()}
      {:else if step === 'manual'}
        <span class="text-muted">{t('update.manual')}</span>
        {@render commandBox()}
        {#if reason}<span class="text-xs text-faint">{reason}</span>{/if}
      {:else if step === 'error'}
        <span class="text-prod-fg" role="alert">{error}</span>
      {:else if !info.canInstall}
        <span class="text-muted">{t('update.notSupported')}</span>
      {:else if info.terminalInstall}
        <span class="text-muted">{t('update.explainTerminal')}</span>
      {:else}
        <span class="text-muted">{t('update.explain')}</span>
      {/if}
    </div>

    <div class="flex justify-end gap-2.5">
      {#if info.canInstall && (step === 'ready' || step === 'error')}
        <button class="h-[38px] rounded-lg border border-line-strong px-4 text-sm" onclick={onClose}>{t('update.later')}</button>
        <button class="h-[38px] rounded-lg bg-accent px-[18px] text-sm font-semibold text-accent-ink" onclick={install}
          >{step === 'error' ? t('update.retry') : t('update.install')}</button
        >
      {:else if step === 'terminal'}
        <button class="h-[38px] rounded-lg border border-line-strong px-4 text-sm" onclick={onClose}>{t('update.close')}</button>
      {:else if step !== 'working'}
        <button class="h-[38px] rounded-lg bg-accent px-[18px] text-sm font-semibold text-accent-ink" onclick={() => api.OpenURL(info.url)}
          >{t('update.openPage')}</button
        >
      {/if}
    </div>
  </div>
</div>
