<script>
  // Wraps VitalsRing with a logging ring built from dashboard.loggedDays.
  // Geometry mirrors VitalsRing.svelte: the calorie ring's outer edge sits at
  // radius size/2, so the streak band starts just beyond it (size/2 + 2) and
  // ends at size/2 + 16 — still inside the size + 40 viewBox.
  //
  // The wrapped VitalsRing should be given dailyLogged={[]} so only its
  // background track shows through the gaps in this ring.
  let {
    loggedDays = [],
    loggingStreak = 0,
    longestStreak = 0,
    size = 160,
    children
  } = $props()

  // Cap at 60 days (contract) and coerce to booleans. Missing data → no ring.
  let days = $derived(
    Array.isArray(loggedDays) && loggedDays.length
      ? loggedDays.slice(-60).map(d => !!d)
      : []
  )
  let n = $derived(days.length || 30)
  let hasData = $derived(days.length > 0)

  let box = $derived(size + 40)
  let center = $derived(box / 2)
  let ringInner = $derived(size / 2 + 2)
  let ringOuter = $derived(size / 2 + 16)
  let ringWidth = $derived(ringOuter - ringInner)
  let ringMid = $derived((ringInner + ringOuter) / 2)

  // Streak colour rule: 30+ days shifts the whole ring to gold/amber.
  let gold = $derived(Number(loggingStreak) >= 30)
  const ACCENT = '#06b6d4'
  const ACCENT_TODAY = '#22d3ee'
  const GOLD = '#f59e0b'
  const GOLD_TODAY = '#fbbf24'

  // One wedge segment per day, inner→outer aligned, with a gap when unlogged.
  let segments = $derived.by(() => {
    const seg = 360 / n
    const gap = Math.max(0.9, seg * 0.18)
    return days.map((logged, i) => {
      const startAngle = i * seg - 90 - (seg - gap) / 2
      const endAngle = startAngle + (seg - gap)
      const sr = (startAngle * Math.PI) / 180
      const er = (endAngle * Math.PI) / 180
      const x1 = Math.cos(sr) * ringInner
      const y1 = Math.sin(sr) * ringInner
      const x2 = Math.cos(er) * ringInner
      const y2 = Math.sin(er) * ringInner
      const x3 = Math.cos(er) * ringOuter
      const y3 = Math.sin(er) * ringOuter
      const x4 = Math.cos(sr) * ringOuter
      const y4 = Math.sin(sr) * ringOuter
      return {
        logged,
        isToday: i === n - 1,
        path: `M ${x1} ${y1} A ${ringInner} ${ringInner} 0 0 1 ${x2} ${y2} L ${x3} ${y3} A ${ringOuter} ${ringOuter} 0 0 0 ${x4} ${y4} Z`
      }
    })
  })

  let loggedCount = $derived(days.filter(Boolean).length)
</script>

<div class="relative flex flex-col items-center" style="width: {box}px">
  {@render children?.()}

  {#if hasData}
    <svg
      class="pointer-events-none absolute left-0 top-0"
      width={box}
      height={box}
      viewBox="0 0 {box} {box}"
      aria-hidden="true"
    >
      <g transform="translate({center},{center})">
        <!-- Track shows through unlogged gaps -->
        <circle
          r={ringMid}
          fill="none"
          stroke="#1f2937"
          stroke-width={ringWidth}
        />

        {#each segments as s, i (i)}
          {#if s.logged}
            <path
              d={s.path}
              fill={gold ? (s.isToday ? GOLD_TODAY : GOLD) : (s.isToday ? ACCENT_TODAY : ACCENT)}
              opacity={s.isToday ? 1 : 0.85}
            />
          {/if}
        {/each}
      </g>
    </svg>
  {/if}

  {#if gold}
    <div class="mt-1 inline-flex items-center gap-1 rounded-full bg-amber-500/15 px-2.5 py-0.5 text-xs font-semibold text-amber-400">
      <span aria-hidden="true">🔥</span> {loggingStreak}-day streak
    </div>
  {/if}

  <span class="sr-only">
    Logged {loggedCount} of {n} days. Current streak {loggingStreak} days, longest {longestStreak} days.
  </span>
</div>
