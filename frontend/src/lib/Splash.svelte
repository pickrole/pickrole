<script lang="ts">
  // Opening screen: the logo and name appear, then the tagline is typed out.
  // The caret keeps blinking and the app waits for Enter or a click; any key
  // or click while typing completes the text at once.
  import { onMount } from 'svelte'
  import Icon from './Icon.svelte'
  import { t } from './i18n/index.svelte'

  // lines: the tagline split where it must break, so nothing reflows while typing.
  let { lines, ready, onDone }: { lines: string[]; ready: boolean; onDone: () => void } = $props()

  // Where each line starts in the typed count (the break itself costs no key).
  const starts = $derived(lines.map((_, i) => lines.slice(0, i).join('').length))
  const total = $derived(lines.join('').length)

  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches

  let typed = $state(0)
  let finished = $state(reduced)
  let wantsIn = $state(false)
  let leaving = $state(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  // A human rhythm: a little jitter per key and a pause after punctuation
  // and at the line break.
  function typeNext() {
    typed++
    if (typed >= total) {
      finished = true
      return
    }
    const pause = /[,.]/.test(lines.join('')[typed - 1]) || starts.includes(typed) ? 120 : 0
    timer = setTimeout(typeNext, 10 + Math.random() * 16 + pause)
  }

  // The caret sits on the line being typed; at the end of a line it already
  // moves to the start of the next one.
  function caretLine(n: number): number {
    let line = 0
    starts.forEach((start, i) => {
      if (start <= n) line = i
    })
    return line
  }

  onMount(() => {
    if (reduced) typed = total
    else timer = setTimeout(typeNext, 450)
    return () => clearTimeout(timer)
  })

  function complete() {
    clearTimeout(timer)
    typed = total
    finished = true
  }

  // Enter as soon as the app has its data.
  $effect(() => {
    if (wantsIn && ready && !leaving) {
      leaving = true
      setTimeout(onDone, 200)
    }
  })

  function onKey(e: KeyboardEvent) {
    if (!finished) complete()
    else if (e.key === 'Enter') {
      e.preventDefault()
      wantsIn = true
    }
  }

  function onClick() {
    if (!finished) complete()
    else wantsIn = true
  }
</script>

<svelte:window onkeydown={onKey} onclick={onClick} />

<div class="splash relative flex h-full flex-col items-center justify-center overflow-hidden px-8 text-center" class:leaving>
  <div class="brand-glow glow" aria-hidden="true"></div>
  <div class="logo relative flex size-16 items-center justify-center rounded-[18px] bg-accent text-accent-ink">
    <Icon name="key" size={34} stroke={2.3} />
  </div>
  <h1 class="name relative mt-5 font-display text-[32px] font-semibold tracking-tight">PickRole</h1>
  <!-- Each line keeps its full width (the untyped rest is transparent) and never wraps. -->
  <p class="relative mt-3 text-[15px] leading-relaxed text-muted" aria-label={lines.join(' ')}>
    {#each lines as line, i (i)}
      {@const shown = Math.min(Math.max(typed - starts[i], 0), line.length)}
      <span class="block whitespace-nowrap" aria-hidden="true"
        >{line.slice(0, shown)}{#if caretLine(typed) === i}<span class="caret" class:blink={finished}></span
          >{/if}<span class="text-transparent">{line.slice(shown)}</span></span
      >
    {/each}
  </p>
  <p class="hint relative mt-10 flex h-6 items-center gap-2 text-xs text-faint" class:show={finished} aria-live="polite">
    {#if finished && wantsIn && !ready}
      {t('splash.loading')}
    {:else if finished}
      {t('splash.press')}
      <span class="rounded-[5px] border border-accent-line px-[7px] py-[3px] font-mono text-[11px] text-accent-soft-fg"
        >Enter</span
      >
      {t('splash.toEnter')}
    {/if}
  </p>
</div>

<style>
  .splash {
    transition: opacity 200ms ease-in;
  }
  .splash.leaving {
    opacity: 0;
  }
  .glow {
    opacity: 0;
    animation: fade-in 900ms ease-out forwards;
  }
  .logo {
    animation: pop 450ms cubic-bezier(0.2, 0.9, 0.3, 1.25) both;
  }
  .name {
    animation: rise 450ms ease-out 150ms both;
  }
  /* Zero width in the layout, so moving it never changes a line's width. */
  .caret {
    position: relative;
    display: inline-block;
    width: 0;
    height: 1.1em;
    vertical-align: -0.2em;
  }
  .caret::after {
    content: '';
    position: absolute;
    inset: 0 auto 0 1px;
    width: 2px;
    background: var(--accent);
  }
  .hint {
    opacity: 0;
    transition: opacity 300ms ease-out 150ms;
  }
  .hint.show {
    opacity: 1;
  }
  .caret.blink {
    animation: blink 1s steps(1) infinite;
  }
  @keyframes fade-in {
    to {
      opacity: 1;
    }
  }
  @keyframes pop {
    from {
      opacity: 0;
      transform: scale(0.8);
    }
  }
  @keyframes rise {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
  }
  @keyframes blink {
    50% {
      opacity: 0;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .glow,
    .logo,
    .name {
      animation: none;
      opacity: 1;
    }
    .hint {
      transition: none;
    }
  }
</style>
