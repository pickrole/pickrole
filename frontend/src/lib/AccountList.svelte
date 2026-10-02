<script lang="ts">
  import Icon from './Icon.svelte'
  import ProdTag from './ProdTag.svelte'
  import type { Account } from './types'
  import { t, tn } from './i18n/index.svelte'

  let {
    accounts,
    selectedId,
    activeId,
    onSelect,
    onHome,
  }: {
    accounts: Account[]
    selectedId: string | null
    activeId: string | null
    onSelect: (id: string) => void
    /** Back to the start screen: continue where you left off, recents. */
    onHome: () => void
  } = $props()

  let query = $state('')
  let input: HTMLInputElement | undefined = $state()

  const filtered = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return q ? accounts.filter((a) => a.name.toLowerCase().includes(q) || a.id.includes(q)) : accounts
  })
  const favorites = $derived(filtered.filter((a) => a.favorite))
  const others = $derived(filtered.filter((a) => !a.favorite))

  function onKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault()
      input?.focus()
      input?.select()
    }
  }

  function onSearchKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && filtered.length > 0) onSelect(filtered[0].id)
    if (e.key === 'Escape') query = ''
  }

  const roleCount = (a: Account) => tn('accounts.roles', a.roles?.length ?? 0)
</script>

<svelte:window onkeydown={onKey} />

<aside class="flex w-[300px] shrink-0 flex-col gap-3 border-r border-line p-3">
  <label
    class="relative flex h-[38px] shrink-0 items-center gap-2 rounded-[9px] border border-line bg-surface px-2.5 text-faint focus-within:border-accent-line"
  >
    <Icon name="search" size={15} />
    <span class="sr-only">{t('accounts.search')}</span>
    <input
      bind:this={input}
      bind:value={query}
      onkeydown={onSearchKey}
      type="text"
      placeholder={t('accounts.searchPlaceholder')}
      class="min-w-0 grow bg-transparent text-[13.5px] text-fg outline-none placeholder:text-faint"
    />
    <span class="shrink-0 rounded-[5px] border border-line px-[5px] py-[2px] font-mono text-[10.5px] whitespace-nowrap">Ctrl K</span>
  </label>

  <button
    class="flex h-[38px] shrink-0 items-center gap-2.5 rounded-lg px-2.5 text-left text-sm {selectedId === null
      ? 'bg-surface-2 font-semibold'
      : 'font-medium text-muted hover:bg-surface hover:text-fg'}"
    aria-current={selectedId === null ? 'page' : undefined}
    onclick={onHome}
  >
    <Icon name="home" size={15} />
    <span class="grow">{t('accounts.home')}</span>
    <span class="shrink-0 rounded-[5px] border border-line px-[5px] py-[2px] font-mono text-[10.5px] font-normal whitespace-nowrap text-faint">Esc</span>
  </button>

  <div class="-mr-1 flex min-h-0 grow flex-col gap-4 overflow-y-auto pr-1">
    {#snippet group(title: string, list: Account[], favorite = false)}
      {#if list.length > 0}
        <div class="flex flex-col gap-0.5">
          <div class="flex items-center gap-2 px-2.5 pb-1.5">
            {#if favorite}<span class="text-accent"><Icon name="star" size={12} /></span>{/if}
            <span class="text-xs font-semibold tracking-[0.06em] text-muted uppercase">{title}</span>
            <span class="rounded-full bg-surface-2 px-1.5 font-mono text-[10.5px] leading-[18px] text-muted">{list.length}</span>
            <span class="h-px grow bg-line" aria-hidden="true"></span>
          </div>
          {#each list as a (a.id)}
            <button
              class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-[7px] text-left {a.id === selectedId
                ? 'bg-surface-2'
                : 'hover:bg-surface'}"
              aria-current={a.id === selectedId ? 'true' : undefined}
              onclick={() => onSelect(a.id)}
            >
              <div class="flex min-w-0 grow flex-col gap-px">
                <span class="flex items-center gap-1.5">
                  <span class="truncate text-sm {a.id === selectedId ? 'font-semibold' : 'font-medium'}">{a.name}</span>
                  {#if a.production}<ProdTag />{/if}
                </span>
                <span class="font-mono text-[11.5px] text-faint">{a.id}</span>
              </div>
              {#if a.id === activeId}
                <span class="flex items-center gap-[5px] text-xs text-ok">
                  <span class="size-[7px] rounded-full bg-ok"></span>{t('accounts.active')}
                </span>
              {:else}
                <span class="text-xs whitespace-nowrap text-faint">{roleCount(a)}</span>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    {/snippet}

    {@render group(t('accounts.favorites'), favorites, true)}
    {@render group(favorites.length > 0 ? t('accounts.all') : t('accounts.accounts'), others)}

    {#if filtered.length === 0}
      <p class="px-2.5 text-[13px] text-faint">
        {accounts.length === 0 ? t('accounts.emptyCache') : t('accounts.noMatch', { query })}
      </p>
    {/if}
  </div>
</aside>
