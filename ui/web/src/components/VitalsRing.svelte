<script>
  // Props: consumed calories, target, size, stroke, and streak data
  let { 
    consumed = 0, 
    target = 0, 
    size = 140, 
    stroke = 12,
    dailyLogged = [],
    currentStreak = 0,
    longestStreak = 0
  } = $props()

  // Inner calorie ring
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

  // Streak ring settings - outer ring
  const streakStroke = 6
  const streakGapDeg = 2 // gap degrees between segments
  const segments = 30
  const segmentDeg = 360 / segments
  const streakRadius = $derived(size / 2 + stroke / 2 + 8)
  const streakInnerRadius = $derived(streakRadius - streakStroke)

  // Ensure we have exactly 30 days (pad with zeros on the left)
  let days = $derived(dailyLogged.length >= segments ? dailyLogged.slice(-segments) : [...Array(segments - dailyLogged.length).fill(0), ...dailyLogged])
</script>

<svg width={size + 40} height={size + 40} viewBox="0 0 {size + 40} {size + 40}" class="block mx-auto">
  <defs>
    <filter id="glassShadow" x="-50%" y="-50%" width="200%" height="200%">
      <feDropShadow dx="0" dy="4" stdDeviation="8" flood-color="#000" flood-opacity="0.35"/>
    </filter>
  </defs>
  <g transform="translate({(size + 40)/2},{(size + 40)/2})">
    <!-- OUTER STREAK RING - 30 arc segments -->
    <!-- Background ring for streaks -->
    <circle r={(streakRadius + streakInnerRadius) / 2} fill="none" stroke="#1f2937" stroke-width={streakRadius - streakInnerRadius} />

    <!-- Logged segments -->
    {#each days as logged, i}
      {@const startAngle = (i * segmentDeg) - 90 - (segmentDeg - streakGapDeg) / 2}
      {@const endAngle = startAngle + (segmentDeg - streakGapDeg)}
      {@const startRad = startAngle * Math.PI / 180}
      {@const endRad = endAngle * Math.PI / 180}
      {@const x1 = Math.cos(startRad) * streakInnerRadius}
      {@const y1 = Math.sin(startRad) * streakInnerRadius}
      {@const x2 = Math.cos(endRad) * streakInnerRadius}
      {@const y2 = Math.sin(endRad) * streakInnerRadius}
      {@const x3 = Math.cos(endRad) * streakRadius}
      {@const y3 = Math.sin(endRad) * streakRadius}
      {@const x4 = Math.cos(startRad) * streakRadius}
      {@const y4 = Math.sin(startRad) * streakRadius}
      {@const isToday = i === (segments - 1)}
      {#if logged}
        <path 
          d="M {x1} {y1} A {streakInnerRadius} {streakInnerRadius} 0 0 1 {x2} {y2} L {x3} {y3} A {streakRadius} {streakRadius} 0 0 0 {x4} {y4} Z"
          fill={isToday ? '#22d3ee' : '#06b6d4'}
          opacity={isToday ? 1 : 0.85}
        />
      {/if}
    {/each}

    <!-- Background track for calorie ring -->
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
