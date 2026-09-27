<script lang="ts">
  let { label = 'PROD', size = 'sm' }: { label?: string; size?: 'sm' | 'md' } = $props()
</script>

<span
  class="rounded-[5px] bg-prod-soft font-semibold tracking-[0.06em] text-prod-fg {size === 'md'
    ? 'px-2 py-[3px] text-[11px]'
    : 'px-1.5 py-[2px] text-[10px]'}"
>
  {label}
</span>
