<script>
  import { dispWeight, weightUnit } from '../lib/utils.js'

  // Props
  export let points = []
  export let units = 'imperial'
  // Initial view: 'raw' | 'trend'. The pill toggles it locally afterwards.
  export let series = 'raw'
  // Span (days) used for the display-only EMA smoothing.
  export let trendWindow = 7

  let mode = series === 'trend' ? 'trend' : 'raw'

  // Filter out points with invalid weightKg and ensure sorted by date (oldest -> newest)
  $: validPoints = points.filter(p => p.weightKg !== undefined && p.weightKg !== null && isFinite(p.weightKg))
  $: sorted = [...validPoints].sort((a, b) => a.date.localeCompare(b.date))

  // Convert points to display units but keep raw (kg) for scaling
  $: displayPoints = sorted.map(p => ({
    date: p.date,
    weight: dispWeight(p.weightKg, units),
    raw: p.weightKg
  }))

  // The API only ships raw weigh-ins today. Until the server exposes a full
  // EMA series, smooth the sparkline here for DISPLAY ONLY. Headline trend
  // numbers (Dashboard's weightTrendKg / weightRateKgPerWeek) always come
  // from the server — never from this client-side smoothing.
  function ema(values, alpha) {
    const out = []
    let prev = null
    for (const v of values) {
      prev = prev === null ? v : prev + alpha * (v - prev)
      out.push(prev)
    }
    return out
  }

  $: alpha = 2 / (Math.max(trendWindow, 2) + 1)
  $: trendSeries = (() => {
    if (!displayPoints.length) return []
    const smooth = ema(displayPoints.map(p => p.raw), alpha)
    return displayPoints.map((p, i) => ({
      date: p.date,
      raw: smooth[i],
      weight: dispWeight(smooth[i], units)
    }))
  })()

  // Active series drives both the path and the min/max labels.
  $: active = mode === 'trend' ? trendSeries : displayPoints
  $: hasSeries = active.length >= 2

  // Calculate min/max for scaling (use raw kg values then convert for display labels)
  $: minWeight = active.length ? Math.min(...active.map(p => p.raw)) : 0
  $: maxWeight = active.length ? Math.max(...active.map(p => p.raw)) : 0
  $: range = Math.max(maxWeight - minWeight, 0.1)

  // SVG dimensions
  const width = 280
  const height = 80
  const padding = 10

  // Generate SVG path
  $: pathData = (() => {
    if (active.length < 2) return ''
    const pts = active.map((p, i) => {
      const x = padding + (i / (active.length - 1)) * (width - 2 * padding)
      const y = height - padding - ((p.raw - minWeight) / range) * (height - 2 * padding)
      return `${i === 0 ? 'M' : 'L'} ${x} ${y}`
    })
    return pts.join(' ')
  })()

  // Last point for the end dot
  $: lastIdx = active.length - 1
  $: lastX = active.length ? padding + (lastIdx / Math.max(active.length - 1, 1)) * (width - 2 * padding) : 0
  $: lastY = active.length
    ? height - padding - ((active[lastIdx].raw - minWeight) / range) * (height - 2 * padding)
    : 0

  // Last raw reading — kept unsmoothed so the big number matches the scale.
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
  {#if !displayPoints.length}
    <!-- Empty state: never render a "0 lbs" headline or 0/0 min-max labels. -->
    <div class="text-sm text-gray-400 text-center py-6 max-w-[280px]">
      No weigh-ins yet — log today's weight to start your trend.
    </div>
  {:else}
  <!-- Current weight + raw/trend toggle -->
  <div class="flex items-center justify-between w-full max-w-[280px] gap-2">
    <div class="text-2xl font-bold text-gray-100">
      {current} <span class="text-sm font-normal text-gray-400">{weightUnit(units)}</span>
    </div>
    {#if displayPoints.length >= 2}
      <div class="inline-flex items-center rounded-full bg-gray-700 p-0.5 text-[11px]" role="group" aria-label="Sparkline series">
        <button
          type="button"
          class="px-2 py-0.5 rounded-full transition-colors {mode === 'raw' ? 'bg-gray-500 text-white' : 'text-gray-400 hover:text-gray-200'}"
          aria-pressed={mode === 'raw'}
          onclick={() => mode = 'raw'}
        >Raw</button>
        <button
          type="button"
          class="px-2 py-0.5 rounded-full transition-colors {mode === 'trend' ? 'bg-gray-500 text-white' : 'text-gray-400 hover:text-gray-200'}"
          aria-pressed={mode === 'trend'}
          onclick={() => mode = 'trend'}
        >Trend</button>
      </div>
    {/if}
  </div>

  <!-- Sparkline SVG -->
  {#if hasSeries}
    <svg {width} {height} class="w-full max-w-[280px] mt-2" role="img" aria-label="Weight {mode} sparkline">
      <!-- Line -->
      <path
        d={pathData}
        fill="none"
        stroke={mode === 'trend' ? '#38bdf8' : '#10b981'}
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-dasharray={mode === 'trend' ? '5 3' : undefined}
      />
      <!-- End dot -->
      <circle cx={lastX} cy={lastY} r="4" fill={mode === 'trend' ? '#38bdf8' : '#10b981'} />
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

  {#if mode === 'trend'}
    <div class="text-[10px] text-gray-600 mt-0.5">Display-only EMA · headline trend comes from the server</div>
  {/if}
  {/if}
</div>

<!-- Optional: small trend indicator (hidden visually but left for reference) -->
{#if trend}
  <span class="sr-only">Trend: {trend}</span>
{/if}
