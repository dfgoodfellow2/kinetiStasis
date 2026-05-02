<script>
  import { dispWeight, weightUnit } from '../lib/utils.js'

  // Props
  export let points = []
  export let units = 'imperial'

  // Filter out points with invalid weightKg and ensure sorted by date (oldest -> newest)
  $: validPoints = points.filter(p => p.weightKg !== undefined && p.weightKg !== null && isFinite(p.weightKg))
  $: sorted = [...validPoints].sort((a, b) => a.date.localeCompare(b.date))

  // Convert points to display units but keep raw (kg) for scaling
  $: displayPoints = sorted.map(p => ({
    date: p.date,
    weight: dispWeight(p.weightKg, units),
    raw: p.weightKg
  }))

  // Calculate min/max for scaling (use raw kg values then convert for display labels)
  $: minWeight = displayPoints.length ? Math.min(...displayPoints.map(p => p.raw)) : 0
  $: maxWeight = displayPoints.length ? Math.max(...displayPoints.map(p => p.raw)) : 0
  $: range = Math.max(maxWeight - minWeight, 0.1)

  // SVG dimensions
  const width = 280
  const height = 80
  const padding = 10

  // Generate SVG path
  $: pathData = (() => {
    if (displayPoints.length < 2) return ''
    const pts = displayPoints.map((p, i) => {
      const x = padding + (i / (displayPoints.length - 1)) * (width - 2 * padding)
      const y = height - padding - ((p.raw - minWeight) / range) * (height - 2 * padding)
      return `${i === 0 ? 'M' : 'L'} ${x} ${y}`
    })
    return pts.join(' ')
  })()

  // Current weight (already converted for display)
  $: current = displayPoints.length ? displayPoints[displayPoints.length - 1].weight : 0

  // Simple trend detection
  $: trend = (() => {
    if (displayPoints.length < 7) return null
    const first = displayPoints[0].raw
    const last = displayPoints[displayPoints.length - 1].raw
    const diff = last - first
    if (Math.abs(diff) < 0.1) return 'stable'
    return diff > 0 ? 'up' : 'down'
  })()
</script>

<div class="flex flex-col items-center text-gray-100">
  <!-- Current weight -->
  <div class="text-2xl font-bold text-gray-100">
    {current} <span class="text-sm font-normal text-gray-400">{weightUnit(units)}</span>
  </div>

  <!-- Sparkline SVG -->
  {#if displayPoints.length >= 2}
    <svg {width} {height} class="w-full max-w-[280px] mt-2" role="img" aria-label="Weight sparkline">
      <!-- Line -->
      <path
        d={pathData}
        fill="none"
        stroke="#10b981"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <!-- End dot -->
      {#if displayPoints.length > 0}
        {@const lastIdx = displayPoints.length - 1}
        {@const x = padding + (lastIdx / (displayPoints.length - 1)) * (width - 2 * padding)}
        {@const y = height - padding - ((displayPoints[lastIdx].raw - minWeight) / range) * (height - 2 * padding)}
        <circle cx={x} cy={y} r="4" fill="#10b981" />
      {/if}
    </svg>
  {:else}
    <div class="h-20 flex items-center justify-center text-gray-500 text-sm">
      No weight data
    </div>
  {/if}

  <!-- Min/Max labels -->
  <div class="flex justify-between w-full text-xs text-gray-500 mt-1 px-2 max-w-[280px]">
    <span>Min: {dispWeight(minWeight, units)} {weightUnit(units)}</span>
    <span>Max: {dispWeight(maxWeight, units)} {weightUnit(units)}</span>
  </div>
</div>

<!-- Optional: small trend indicator (hidden visually but left for reference) -->
{#if trend}
  <span class="sr-only">Trend: {trend}</span>
{/if}
