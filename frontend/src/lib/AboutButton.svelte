<script lang="ts">
  // The "About" button of the headers, with a dot when a new version is out:
  // the notice lives in the About dialog (docs/adr/0024).
  import Icon from './Icon.svelte'
  import { t } from './i18n/index.svelte'
  import type { UpdateInfo } from './types'

  let { update, onAbout }: { update: UpdateInfo | null; onAbout: () => void } = $props()

  const label = $derived(update?.available ? t('about.titleUpdate') : t('about.title'))
</script>

<button
  class="relative flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
  aria-label={label}
  title={label}
  onclick={onAbout}
>
  <Icon name="info" size={15} />
  {#if update?.available}
    <span class="absolute -top-[3px] -right-[3px] size-[9px] rounded-full border-2 border-bg bg-accent" aria-hidden="true"
    ></span>
  {/if}
</button>
