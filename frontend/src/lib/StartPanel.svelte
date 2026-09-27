<script lang="ts">
  import Icon from './Icon.svelte'
  import ProdTag from './ProdTag.svelte'
  import type { Recent } from './types'
  import { relative } from './time'
  import { t, tn } from './i18n/index.svelte'

  let {
    recents,
    busy,
    onContinue,
  }: {
    recents: Recent[]
    busy: string | null
    onContinue: (r: Recent) => void
  } = $props()

  const last = $derived(recents[0])
  const others = $derived(recents.slice(1, 4))

  function onKey(e: KeyboardEvent) {
    const target = e.target as HTMLElement
    if (e.key === 'Enter' && !e.repeat && last && !busy && !['INPUT', 'BUTTON', 'SELECT', 'TEXTAREA'].includes(target.tagName)) {
      onContinue(last)
    }
  }
</script>

<svelte:window onkeydown={onKey} />

<main class="flex grow flex-col gap-3.5 px-7 py-7">
  {#if last}
    <span class="text-[11px] font-semibold tracking-[0.08em] text-faint uppercase">{t('start.continue')}</span>
    <button
      class="flex w-full items-center gap-3.5 rounded-xl border px-[18px] py-4 text-left {last.production
        ? 'border-prod bg-prod-card'
        : 'border-accent bg-accent-soft'}"
      onclick={() => onContinue(last)}
      disabled={busy !== null}
    >
      <span
        class="flex size-10 shrink-0 items-center justify-center rounded-full {last.production
          ? 'bg-prod text-prod-ink'
          : 'bg-accent text-accent-ink'}"
      >
        <Icon name="play" size={16} />
      </span>
      <span class="flex grow flex-col gap-[3px]">
        <span class="flex items-center gap-2 text-[15px] font-semibold">
          {last.accountName} · {last.role}
          {#if last.production}<ProdTag />{/if}
        </span>
        <span class="text-[12.5px] {last.production ? 'text-prod-fg' : 'text-accent-soft-fg'}"
          >{t('start.used', { when: relative(last.usedAt) })}</span
        >
      </span>
      <span
        class="rounded-[5px] border px-[7px] py-[3px] font-mono text-[11px] {last.production
          ? 'border-prod text-prod-fg'
          : 'border-accent-line text-accent-soft-fg'}">Enter</span
      >
    </button>

    {#if others.length > 0}
      <span class="pt-2.5 text-[11px] font-semibold tracking-[0.08em] text-faint uppercase">{t('start.recent')}</span>
      {#each others as r (r.accountId + r.role)}
        <button
          class="flex w-full items-center gap-2.5 rounded-[10px] border border-line bg-surface px-4 py-3 text-left hover:border-line-strong"
          onclick={() => onContinue(r)}
          disabled={busy !== null}
        >
          <span class="flex grow items-center gap-2 text-sm">
            {r.accountName} · {r.role}
            {#if r.production}<ProdTag />{/if}
          </span>
          <span class="text-xs text-faint">{relative(r.usedAt)}</span>
        </button>
      {/each}
    {/if}
    <p class="pt-1.5 text-[13px] text-faint">{t('start.orPick')}</p>
  {:else}
    <div class="m-auto flex max-w-[300px] flex-col items-center gap-2 text-center">
      <span class="text-[15px] font-semibold">{t('start.emptyTitle')}</span>
      <p class="text-[13px] leading-relaxed text-muted">
        {t('start.emptyBody')}
      </p>
    </div>
  {/if}
</main>
