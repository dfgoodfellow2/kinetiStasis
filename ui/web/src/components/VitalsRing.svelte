<script>
  // Props: consumed calories and target calories for today
  let { consumed = 0, target = 0, size = 140, stroke = 12 } = $props()

  let radius = $derived((size - stroke) / 2)
  let circumference = $derived(2 * Math.PI * radius)

  // Clamp to 0–200% so a large surplus still renders cleanly
  let percent = $derived(target > 0 ? Math.max(0, Math.min(200, (consumed / target) * 100)) : 0)
  let dash = $derived(circumference * (percent / 100))

  // Color gradient: red (empty) → orange → yellow → lime → neon green (at/over target)
  let color = $derived(
    target === 0    ? '#3b82f6'
    : percent >= 100 ? '#39ff14'
    : percent >= 90  ? '#4ade80'
    : percent >= 75  ? '#84cc16'
    : percent >= 60  ? '#a3e635'
    : percent >= 45  ? '#eab308'
    : percent >= 30  ? '#f59e0b'
    : percent >= 15  ? '#f97316'
    :                  '#ef4444'
  )
</script>

<svg width={size} height={size} viewBox="0 0 {size} {size}" class="block mx-auto">
  <defs>
    <filter id="glassShadow" x="-50%" y="-50%" width="200%" height="200%">
      <feDropShadow dx="0" dy="4" stdDeviation="8" flood-color="#000" flood-opacity="0.35"/>
    </filter>
  </defs>
  <g transform="translate({size/2},{size/2})">
    <!-- Background track -->
    <circle r={radius} fill="none" stroke="#374151" stroke-width={stroke} opacity="0.8" />

    <!-- Progress ring -->
    <circle
      r={radius}
      fill="none"
      stroke={color}
      stroke-width={stroke}
      stroke-linecap="round"
      stroke-dasharray="{dash} {circumference - dash}"
      stroke-dashoffset="0"
      transform="rotate(-90)"
      filter="url(#glassShadow)"
    />

    <!-- Center text -->
    <text x="0" y="-8" text-anchor="middle" font-size="22" fill="#f3f4f6" font-weight="700">
      {consumed > 0 ? Math.round(consumed) : '--'}
    </text>
    <text x="0" y="10" text-anchor="middle" font-size="10" fill="#9ca3af">Consumed Today</text>
    {#if target > 0}
      <text x="0" y="26" text-anchor="middle" font-size="9" fill="#6b7280">Target: {target}</text>
    {/if}
  </g>
</svg>
