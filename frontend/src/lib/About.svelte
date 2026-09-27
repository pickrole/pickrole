<script lang="ts">
  // "Sobre" dialog, styled after the opening screen: the brand, the build
  // details and where PickRole reads and writes, with a copy button for bug
  // reports.
  import { onMount } from 'svelte'
  import { fade, scale } from 'svelte/transition'
  import Icon from './Icon.svelte'
  import { api, errorMessage } from './api'
  import { tagline } from './brand'
  import { locale, t } from './i18n/index.svelte'
  import type { About } from './types'

  let { onClose }: { onClose: () => void } = $props()

  let about = $state<About | null>(null)
  let error = $state('')
  let copied = $state<'ok' | 'failed' | null>(null)
  let copyError = $state('')
  let closeButton = $state<HTMLButtonElement>()

  const systems: Record<string, string> = { windows: 'Windows', linux: 'Linux', darwin: 'macOS' }

  function platform(p: string): string {
    const [os, arch] = p.split('/')
    return `${systems[os] ?? os} · ${arch}`
  }

  function builtAt(iso: string): string {
    return iso ? new Date(iso).toLocaleString(locale(), { dateStyle: 'long', timeStyle: 'short' }) : t('about.localBuild')
  }

  const details = $derived(
    about
      ? [
          { label: t('about.version'), value: about.version },
          { label: t('about.build'), value: about.commit || '—' },
          { label: t('about.buildDate'), value: builtAt(about.buildDate) },
          { label: t('about.platform'), value: platform(about.platform) },
          { label: t('about.go'), value: about.go },
        ]
      : [],
  )

  onMount(async () => {
    closeButton?.focus()
    try {
      about = await api.About()
    } catch (e) {
      error = errorMessage(e)
    }
  })

  async function copy() {
    if (!about) return
    const text = [
      ...details.map((d) => `${d.label}: ${d.value}`),
      ...about.files.map((f) => `${t(`about.file.${f.kind}`)}: ${f.path}`),
    ].join('\n')
    try {
      await api.CopyText(`PickRole\n${text}`)
      copied = 'ok'
    } catch (e) {
      copied = 'failed'
      copyError = errorMessage(e)
    }
    setTimeout(() => (copied = null), 2000)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<div class="fixed inset-0 z-50 flex items-center justify-center p-6" transition:fade={{ duration: 150 }}>
  <button class="absolute inset-0 bg-bg/70 backdrop-blur-[2px]" aria-label={t('about.close')} tabindex="-1" onclick={onClose}
  ></button>
  <div
    role="dialog"
    aria-modal="true"
    aria-labelledby="about-title"
    class="relative flex max-h-full w-full max-w-[440px] flex-col overflow-hidden rounded-2xl border border-line-strong bg-bg"
    transition:scale={{ start: 0.96, duration: 180 }}
  >
    <div class="relative shrink-0 overflow-hidden px-6 pt-7 pb-5 text-center">
      <div class="brand-glow" aria-hidden="true"></div>
      <button
        bind:this={closeButton}
        class="absolute top-3 right-3 flex size-8 items-center justify-center rounded-lg text-muted hover:text-fg"
        aria-label={t('about.close')}
        onclick={onClose}
      >
        <Icon name="close" size={15} />
      </button>
      <div class="relative mx-auto flex size-12 items-center justify-center rounded-[14px] bg-accent text-accent-ink">
        <Icon name="key" size={26} stroke={2.3} />
      </div>
      <h2 id="about-title" class="relative mt-3.5 font-display text-2xl font-semibold tracking-tight">PickRole</h2>
      <p class="relative mt-1.5 text-[13px] leading-relaxed text-muted">
        {#each tagline() as line (line)}<span class="block">{line}</span>{/each}
      </p>
    </div>

    <div class="min-h-0 overflow-y-auto border-t border-line px-6 py-4 text-[13px]">
      {#if error}
        <p class="flex items-center gap-1.5 text-prod-fg" role="alert"><Icon name="alert" size={13} />{error}</p>
      {:else if about}
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-5 gap-y-1.5">
          {#each details as d (d.label)}
            <dt class="text-faint">{d.label}</dt>
            <dd class="truncate font-mono text-[12.5px]" title={d.value}>{d.value}</dd>
          {/each}
        </dl>
        <h3 class="mt-4 mb-1.5 text-[11px] font-semibold tracking-[0.08em] text-faint uppercase">{t('about.files')}</h3>
        <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-5 gap-y-1.5">
          {#each about.files as f (f.kind)}
            <dt class="text-faint">{t(`about.file.${f.kind}`)}</dt>
            <dd class="truncate font-mono text-[12.5px] select-text" title={f.path}>{f.path}</dd>
          {/each}
        </dl>
      {/if}
    </div>

    <div class="flex shrink-0 items-center gap-3 border-t border-line px-6 py-3 text-[13px]">
      <button
        class="flex h-8 items-center gap-1.5 rounded-lg border border-line px-3 {copied === 'ok'
          ? 'text-ok-fg'
          : copied === 'failed'
            ? 'text-prod-fg'
            : ''}"
        title={copied === 'failed' ? copyError : undefined}
        onclick={copy}
        disabled={!about}
      >
        {#if copied === 'ok'}
          <Icon name="check" size={13} />{t('about.copied')}
        {:else if copied === 'failed'}
          <Icon name="alert" size={13} />{t('about.copyFailed')}
        {:else}
          <Icon name="copy" size={13} />{t('about.copy')}
        {/if}
      </button>
      <span class="grow"></span>
      <button
        class="flex items-center gap-1 text-accent-soft-fg hover:underline"
        onclick={() => about && api.OpenURL(about.repository)}
      >
        {t('about.source')}<Icon name="external" size={12} />
      </button>
      <span class="text-faint">{t('about.license')}</span>
    </div>
  </div>
</div>
