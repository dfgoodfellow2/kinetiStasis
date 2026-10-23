<script>
  // Hand-rolled SVG line chart — intentionally no chart library, matching
  // the lightweight style of WeightSparkline.
  //
  // History rows (GET /v1/checkin/history):
  //   { checkInDate, observedTdee, tdeeConfidence, lookbackDays, resultingCalories }
  // The endpoint does not guarantee ordering, so sort oldest → newest here.
  let { history = [], height = 170 } = $props()

  const width = 320
  const pad = { top: 12, right: 14, bottom: 24, left: 46 }

  let rows = $derived(
    (Array.isArray(history) ? history : [])
      .filter(r => r && Number.isFinite(Number(r.observedTdee)) && Number(r.observedTdee) > 0)
      .map(r => ({
        date: String(r.checkInDate ?? ''),
        observed: Number(r.observedTdee),
        target: Number.isFinite(Number(r.resultingCalories)) && Number(r.resultingCalories) > 0
          ? Number(r.resultingCalories)
          : null,
        // New field first, legacy `confidence` second — code defensively.
        confidence: String(r.tdeeConfidence ?? r.confidence ?? ''),
        lookback: Number(r.lookbackDays ?? 0)
      }))
      .sort((a, b) => a.date.localeCompare(b.date))
  )

  // ── Confidence → visual weight ─────────────────────────────────────────────
  // high → solid dot + full-opacity line, medium → smaller faded dot,
  // low → hollow amber ring + a faint vertical band, unknown → neutral gray.
  function confLevel(c) {
    const s = c.toLowerCase()
    if (s.startsWith('high')) return 'high'
    if (s.startsWith('med')) return 'medium'
    if (s.startsWith('low')) return 'low'
    return 'unknown'
  }
  function dotProps(c) {
    switch (confLevel(c)) {
      case 'high':   return { r: 3.3, fill: '#34d399', stroke: 'none', opacity: 1, 'stroke-width': 0 }
      case 'medium': return { r: 2.9, fill: '#34d399', stroke: 'none', opacity: 0.65, 'stroke-width': 0 }
      case 'low':    return { r: 3.4, fill: '#111827', stroke: '#f59e0b', opacity: 1, 'stroke-width': 1.5 }
      default:       return { r: 2.6, fill: '#6b7280', stroke: 'none', opacity: 0.8, 'stroke-width': 0 }
    }
  }

  // ── Scale ──────────────────────────────────────────────────────────────────
  let values = $derived(
    rows.flatMap(r => [r.observed, r.target]).filter(v => Number.isFinite(v) && v > 0)
  )
  let yMin = $derived.by(() => {
    if (!values.length) return 0
    const lo = Math.min(...values)
    const hi = Math.max(...values)
    return lo - Math.max(hi - lo, 200) * 0.18
  })
  let yMax = $derived.by(() => {
    if (!values.length) return 1
    const lo = Math.min(...values)
    const hi = Math.max(...values)
    return hi + Math.max(hi - lo, 200) * 0.18
  })

  const innerW = $derived(width - pad.left - pad.right)
  const innerH = $derived(height - pad.top - pad.bottom)
  const span = $derived(Math.max(yMax - yMin, 1))

  function xFor(i) {
    if (rows.length <= 1) return pad.left + innerW / 2
    return pad.left + (i / (rows.length - 1)) * innerW
  }
  function yFor(v) {
    return pad.top + innerH - ((v - yMin) / span) * innerH
  }

  // Polyline with gaps where a series has no value.
  function linePath(key) {
    if (rows.length < 2) return ''
    let d = ''
    let pen = false
    rows.forEach((r, i) => {
      const v = r[key]
      if (!Number.isFinite(v) || v <= 0) {
        pen = false
        return
      }
      const x = xFor(i).toFixed(1)
      const y = yFor(v).toFixed(1)
      d += `${pen ? 'L' : 'M'}${x},${y}`
      pen = true
    })
    return d
  }

  // Three horizontal gridlines
  let grid = $derived([0, 0.5, 1].map(t => ({
    y: pad.top + innerH - t * innerH,
    label: Math.round(yMin + t * span)
  })))

  // Width of the low-confidence band around a point.
  let bandW = $derived(rows.length > 1 ? Math.max(6, innerW / (rows.length - 1) * 0.8) : 12)

  function fmtDate(d) {
    return d ? d.slice(5) : ''
  }
</script>

{#if rows.length === 0}
  <div class="h-24 flex items-center justify-center text-center text-sm text-gray-500 px-4">
    No check-in history yet. Accept a weekly check-in to start the trajectory.
  </div>
{:else}
  <div class="w-full">
    <svg
      width={width}
      height={height}
      viewBox="0 0 {width} {height}"
      class="w-full"
      role="img"
      aria-label="Observed TDEE and target calories over recent check-ins"
    >
      <!-- Low-confidence bands (drawn first, behind everything) -->
      {#each rows as r, i}
        {#if confLevel(r.confidence) === 'low'}
          {@const x = xFor(i)}
          <rect
            x={x - bandW / 2}
            y={pad.top}
            width={bandW}
            height={innerH}
            fill="#f59e0b"
            opacity="0.07"
          />
        {/if}
      {/each}

      <!-- Grid -->
      {#each grid as g}
        <line x1={pad.left} y1={g.y} x2={width - pad.right} y2={g.y} stroke="#374151" stroke-width="1" opacity="0.7" />
        <text x={pad.left - 6} y={g.y + 3} text-anchor="end" font-size="9" fill="#6b7280">{g.label}</text>
      {/each}

      <!-- Target calories (dashed) -->
      {#if rows.length >= 2}
        <path d={linePath('target')} fill="none" stroke="#38bdf8" stroke-width="1.6" stroke-dasharray="4 3" stroke-linecap="round" opacity="0.85" />
      {/if}

      <!-- Observed TDEE -->
      {#if rows.length >= 2}
        <path d={linePath('observed')} fill="none" stroke="#34d399" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
      {/if}

      <!-- Confidence-weighted dots -->
      {#each rows as r, i}
        {@const p = dotProps(r.confidence)}
        <circle
          cx={xFor(i)}
          cy={yFor(r.observed)}
          r={p.r}
          fill={p.fill}
          stroke={p.stroke}
          stroke-width={p['stroke-width']}
          opacity={p.opacity}
        >
          <title>{r.date}: {Math.round(r.observed)} kcal · {r.confidence || 'unknown'} confidence{r.lookback ? ` · ${r.lookback}d lookback` : ''}</title>
        </circle>
      {/each}

      <!-- Date labels (first / last only) -->
      <text x={pad.left} y={height - 7} font-size="9" fill="#6b7280" text-anchor="start">{fmtDate(rows[0].date)}</text>
      {#if rows.length > 1}
        <text x={width - pad.right} y={height - 7} font-size="9" fill="#6b7280" text-anchor="end">{fmtDate(rows[rows.length - 1].date)}</text>
      {/if}
    </svg>

    <!-- Legend -->
    <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-gray-500 mt-1 px-1">
      <span class="flex items-center gap-1">
        <span class="inline-block w-3 h-0.5 bg-emerald-400"></span>Observed TDEE
      </span>
      <span class="flex items-center gap-1">
        <span class="inline-block w-3 h-0.5 bg-sky-400 opacity-80" style="border-top: 1px dashed #38bdf8;"></span>Target kcal
      </span>
      <span class="flex items-center gap-1">
        <span class="inline-block w-2.5 h-2.5 rounded-full border border-amber-500 bg-gray-900"></span>Low confidence
      </span>
    </div>
  </div>
{/if}
