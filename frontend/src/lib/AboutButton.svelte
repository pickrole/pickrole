<script lang="ts">
  // The "About" button of the headers, with a dot when a new version is out:
  // the notice lives in the About dialog (docs/adr/0024).
  import Icon from './Icon.svelte'
  import { t } from './i18n/index.svelte'
  import type { UpdateInfo } from './types'

  let { update, onAbout }: { update: UpdateInfo | null; onAbout: () => void } = $props()

  // Accent for a regular update; the warning color for a security fix
  // (docs/adr/0025). Red stays reserved for production.
  const label = $derived(
    update?.installed
      ? t('about.titleInstalled')
      : update?.available
        ? update.security
          ? t('about.titleSecurityUpdate')
          : t('about.titleUpdate')
        : t('about.title'),
  )
</script>

<button
  class="relative flex size-9 items-center justify-center rounded-lg border border-line text-muted hover:text-fg"
  aria-label={label}
  title={label}
  onclick={onAbout}
>
  <Icon name="info" size={15} />
  {#if update?.available}
    <span
      class="absolute -top-[3px] -right-[3px] size-[9px] rounded-full border-2 border-bg {update.security && !update.installed ? 'bg-warn' : 'bg-accent'}"
      aria-hidden="true"
    ></span>
  {/if}
</button>
