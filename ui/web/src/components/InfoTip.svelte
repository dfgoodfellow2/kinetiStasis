<script>
  // Small self-contained ⓘ button + popover.
  // Pages own the label; this only supplies the trigger so the host keeps its layout.
  let { label = '', text = '', wide = false } = $props()

  let open = $state(false)
  let wrap = $state(null)
  let popStyle = $state('')
  let above = $state(false)

  const NARROW_W = 260
  const WIDE_W = 320
  const MARGIN = 8

  // Keep the popover inside the viewport: clamp horizontally and flip up when
  // there is no room below. Re-measured on open and on resize/scroll.
  function place() {
    if (!wrap) return
    const r = wrap.getBoundingClientRect()
    const w = Math.min(wide ? WIDE_W : NARROW_W, window.innerWidth - MARGIN * 2)
    let left = r.left + r.width / 2 - w / 2
    left = Math.max(MARGIN, Math.min(left, window.innerWidth - MARGIN - w))
    const shift = left - (r.left + r.width / 2)
    above = r.bottom + 150 > window.innerHeight && r.top > 160
    popStyle = `width: ${Math.round(w)}px; left: calc(50% + ${Math.round(shift)}px);`
  }

  function toggle() {
    open = !open
    if (open) place()
  }

  function onDocPointer(e) {
    if (wrap && !wrap.contains(e.target)) open = false
  }

  function onDocKeydown(e) {
    if (e.key !== 'Escape' || !open) return
    open = false
    wrap?.querySelector('button')?.focus()
  }

  $effect(() => {
    if (!open) return
    place()
    const reposition = () => place()
    document.addEventListener('pointerdown', onDocPointer, true)
    document.addEventListener('keydown', onDocKeydown)
    window.addEventListener('resize', reposition)
    window.addEventListener('scroll', reposition, true)
    return () => {
      document.removeEventListener('pointerdown', onDocPointer, true)
      document.removeEventListener('keydown', onDocKeydown)
      window.removeEventListener('resize', reposition)
      window.removeEventListener('scroll', reposition, true)
    }
  })
</script>

<span class="relative inline-flex items-center align-middle" bind:this={wrap}>
  <button
    type="button"
    class="inline-flex items-center justify-center rounded-full text-gray-400 hover:text-cyan-300 focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 transition-colors"
    aria-label={`What is ${label || 'this'}?`}
    aria-expanded={open}
    onclick={toggle}
  >
    <span class="text-[13px] leading-none" aria-hidden="true">ⓘ</span>
  </button>

  {#if open}
    <span
      role="dialog"
      aria-label={`${label || 'Info'} help`}
      class="absolute z-50 block rounded-lg border border-gray-600 bg-gray-900 px-3 py-2 text-left text-xs font-normal leading-snug text-gray-200 shadow-xl {above ? 'bottom-full mb-1.5' : 'top-full mt-1.5'}"
      style={popStyle}
    >
      {text}
    </span>
  {/if}
</span>
