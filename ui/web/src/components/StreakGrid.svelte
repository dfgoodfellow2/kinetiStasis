<script>
  // Props
  export let dailyLogged = [] // array of ints (1 = logged, 0 = not)
  export let currentStreak = 0
  export let longestStreak = 0

  // Ensure we have exactly 30 days (take last 30 if longer, pad with zeros if shorter)
  $: days = dailyLogged.length >= 30
    ? dailyLogged.slice(-30)
    : [...Array(30 - dailyLogged.length).fill(0), ...dailyLogged]

  // Today is the last day in the 30-day window (index 29)
  const isToday = (i) => i === 29

  const tooltip = (i, logged) => `${isToday(i) ? 'Today: ' : 'Day ' + (30 - i) + ': '}${logged ? 'Logged' : 'No data'}`
</script>

<div class="flex flex-col gap-3">
  <!-- Streak numbers -->
  <div class="flex justify-between text-sm">
    <div class="text-gray-300">
      Current: <span class="font-bold text-emerald-400">{currentStreak}</span>
    </div>
    <div class="text-gray-300">
      Longest: <span class="font-bold text-yellow-400">{longestStreak}</span>
    </div>
  </div>

  <!-- 30-day grid (wraps as needed) -->
  <div class="flex flex-wrap gap-1 justify-start">
    {#each days as logged, i}
      <div class="flex flex-col items-center gap-1">
        <div
          class="w-4 h-4 rounded-sm flex-shrink-0"
          class:bg-emerald-500={!!logged}
          class:bg-gray-700={!logged}
          title={tooltip(i, logged)}
          aria-label={tooltip(i, logged)}
        ></div>
        {#if isToday(i)}
          <div class="text-[10px] text-gray-400">Today</div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Legend -->
  <div class="flex gap-4 text-xs text-gray-500">
    <div class="flex items-center gap-1">
      <div class="w-3 h-3 rounded-sm bg-emerald-500"></div>
      <span>Logged</span>
    </div>
    <div class="flex items-center gap-1">
      <div class="w-3 h-3 rounded-sm bg-gray-700"></div>
      <span>Missing</span>
    </div>
  </div>
</div>
