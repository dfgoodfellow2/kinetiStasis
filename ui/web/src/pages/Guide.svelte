<script>
  import { onMount } from 'svelte'
  import Card from '../components/Card.svelte'
  import { guideSections } from '../lib/guide-content.js'

  let activeId = $state(guideSections[0]?.id ?? '')

  function goTo(id) {
    activeId = id
    const el = document.getElementById(`guide-${id}`)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  onMount(() => {
    if (typeof IntersectionObserver === 'undefined') return

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) activeId = entry.target.dataset.guideId
        }
      },
      { rootMargin: '-15% 0px -70% 0px', threshold: 0 }
    )

    for (const section of guideSections) {
      const el = document.getElementById(`guide-${section.id}`)
      if (el) observer.observe(el)
    }

    return () => observer.disconnect()
  })
</script>

{#snippet blocks(items, compact)}
  <div class={compact ? 'space-y-2 text-sm text-gray-400' : 'space-y-3 text-[15px] leading-relaxed text-gray-300'}>
    {#each items as block}
      {#if typeof block === 'string'}
        <p>{block}</p>
      {:else if block.list}
        <ul class="list-disc pl-5 space-y-1.5 marker:text-emerald-500">
          {#each block.list as item}
            <li>{item}</li>
          {/each}
        </ul>
      {:else if block.steps}
        <ol class="list-decimal pl-5 space-y-1.5 marker:text-emerald-500">
          {#each block.steps as item}
            <li>{item}</li>
          {/each}
        </ol>
      {:else if block.terms}
        <dl class="space-y-2.5">
          {#each block.terms as entry}
            <div class={compact ? '' : 'sm:flex sm:gap-2'}>
              <dt class={`font-semibold ${compact ? 'text-gray-300' : 'text-gray-100'}`}>{entry.term}</dt>
              <dd class={compact ? 'text-gray-400' : 'text-gray-400'}>{entry.def}</dd>
            </div>
          {/each}
        </dl>
      {:else if block.tip}
        <div class="border-l-2 border-emerald-500 bg-emerald-500/5 rounded-r-lg px-3 py-2 text-sm text-gray-300">
          {block.tip}
        </div>
      {/if}
    {/each}
  </div>
{/snippet}

<div class="max-w-screen-xl mx-auto">
  <header class="mb-4">
    <h1 class="text-2xl font-bold text-emerald-400">Guide</h1>
    <p class="text-sm text-gray-400 mt-1">
      The plain-language manual — what each number means and how to use it. Open “Under the hood” if you want the real mechanics.
    </p>
  </header>

  <div class="md:grid md:grid-cols-[210px_minmax(0,1fr)] md:gap-6">
    <!-- Mini table of contents -->
    <aside class="sticky top-0 md:top-4 z-30 md:self-start mb-4 md:mb-0 -mx-4 px-4 py-2 md:mx-0 md:px-0 md:py-0 bg-gray-900/95 md:bg-transparent backdrop-blur md:backdrop-blur-none">
      <nav class="bg-gray-800 border border-gray-700 rounded-xl p-2 md:p-3" aria-label="Guide sections">
        <div class="hidden md:block text-[11px] uppercase tracking-wider text-gray-500 px-2 pb-2">On this page</div>
        <ul class="flex md:flex-col gap-1 overflow-x-auto md:overflow-visible">
          {#each guideSections as section}
            <li class="shrink-0 md:shrink">
              <a
                href={`#guide-${section.id}`}
                class={`block whitespace-nowrap w-full text-left px-2.5 py-2 rounded-lg text-sm transition-colors ${
                  activeId === section.id
                    ? 'text-emerald-300 bg-emerald-500/15'
                    : 'text-gray-400 hover:text-white hover:bg-gray-700/60'
                }`}
                onclick={(event) => {
                  event.preventDefault()
                  goTo(section.id)
                }}
                aria-current={activeId === section.id ? 'location' : undefined}
              >
                {section.title}
              </a>
            </li>
          {/each}
        </ul>
      </nav>
    </aside>

    <!-- Sections -->
    <div class="space-y-4 min-w-0">
      {#each guideSections as section}
        <Card>
          <section
            id={`guide-${section.id}`}
            data-guide-id={section.id}
            class="scroll-mt-24 md:scroll-mt-6"
          >
            <h2 class="text-lg font-bold text-emerald-400 mb-3 flex items-center gap-2">
              <span aria-hidden="true">{section.icon}</span>
              {section.title}
            </h2>

            {@render blocks(section.body, false)}

            {#if section.underTheHood?.length}
              <details class="group mt-4 border-t border-gray-700/70 pt-3">
                <summary class="flex items-center gap-2 cursor-pointer select-none list-none text-sm font-semibold text-gray-300 hover:text-white [&::-webkit-details-marker]:hidden">
                  <span>Under the hood</span>
                  <span class="text-emerald-400 transition-transform group-open:rotate-180" aria-hidden="true">▾</span>
                </summary>
                <div class="mt-3 rounded-lg bg-gray-900/50 border border-gray-700/60 p-3">
                  {@render blocks(section.underTheHood, true)}
                </div>
              </details>
            {/if}
          </section>
        </Card>
      {/each}
    </div>
  </div>
</div>
