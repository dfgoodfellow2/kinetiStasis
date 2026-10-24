<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { autoDistribute as calculateMacros } from '../lib/macros.js'
  import Alert from '../components/Alert.svelte'
  import InfoTip from '../components/InfoTip.svelte'

  let targets = $state({ calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '', eatBackExercise: false })
  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let tdee = $state(null)
  // Insight layer: target-change timeline.
  let targetHistory = $state([])
  let historyLoading = $state(false)
  let savedTargets = $state(null)

  async function loadTargetHistory() {
    historyLoading = true
    try {
      targetHistory = (await api.listTargetHistory()) ?? []
    } catch {
      // Endpoint may not exist on older servers — degrade to empty state.
      targetHistory = []
    } finally {
      historyLoading = false
    }
  }

  onMount(async () => {
    try {
      const t = await api.getTargets()
      if (t) {
        targets = { ...targets, ...t }
        savedTargets = { ...targets }
      }
    } catch (e) {
      error = 'Failed to load targets: ' + (e.message || e)
    }
    try {
      tdee = await api.getTDEE(30)
    } catch (e) {
      if (!error) error = 'Failed to load TDEE suggestion: ' + (e.message || e)
    }
    loadTargetHistory()
  })

  async function submit() {
    error = ''
    success = ''
    loading = true
    try {
      const payload = {
        calories:  Number(targets.calories)  || 0,
        proteinG: Number(targets.proteinG) || 0,
        carbsG:   Number(targets.carbsG)   || 0,
        fatG:     Number(targets.fatG)     || 0,
        fiberG:   Number(targets.fiberG)   || 0,
        waterMl:  Number(targets.waterMl)  || 0,
        eatBackExercise: Boolean(targets.eatBackExercise),
      }
      const saved = await api.putTargets(payload)
      savedTargets = saved ?? payload
      success = 'Targets updated'
      await loadTargetHistory()
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  let distLoading = $state(false)

  // ── Timeline helpers ────────────────────────────────────────────────────────
  // GET /targets/history returns snapshots of the *previous* values, newest
  // first. So the "new" side of entry i is the next-newer snapshot, and for
  // the head it is the live targets. That reconstructs each change cleanly.
  function num(v) {
    const n = Number(v)
    return Number.isFinite(n) ? n : 0
  }

  function waterLabel(ml) {
    const n = num(ml)
    return n >= 1000 ? `${(n / 1000).toFixed(1).replace(/\.0$/, '')} L` : `${Math.round(n)} ml`
  }

  function deltaClass(d) {
    if (d < 0) return 'text-emerald-400'
    if (d > 0) return 'text-amber-400'
    return 'text-gray-500'
  }

  function signed(n) {
    return n > 0 ? `+${n}` : `${n}`
  }

  let timeline = $derived.by(() => {
    const rows = Array.isArray(targetHistory) ? targetHistory : []
    if (!rows.length) return []
    return rows.slice(0, 10).map((snap, i) => {
      const newer = i === 0 ? savedTargets : rows[i - 1]
      if (!newer) return null
      const fromCal = num(snap?.calories)
      const toCal = num(newer?.calories)
      return {
        date: snap?.effectiveDate ?? '',
        fromCal,
        toCal,
        delta: Math.round(toCal - fromCal),
        waterChanged: num(newer?.waterMl) !== num(snap?.waterMl),
        waterTo: num(newer?.waterMl),
        eatBackChanged: Boolean(newer?.eatBackExercise) !== Boolean(snap?.eatBackExercise),
        eatBackOn: Boolean(newer?.eatBackExercise)
      }
    }).filter(Boolean)
  })

  async function autoDistribute() {
    error = ''
    success = ''
    const cal = Number(targets.calories)
    if (!cal || cal <= 0) {
      error = 'Set a calorie target first'
      return
    }
    distLoading = true
    try {
      const result = await calculateMacros(cal)
      targets = {
        ...targets,
        proteinG: String(result.proteinG),
        carbsG: String(result.carbsG),
        fatG: String(result.fatG),
        fiberG: String(result.fiberG),
        waterMl: String(result.waterMl),
      }
      success = 'Macros auto-calculated'
    } catch (e) {
      error = e.message
    } finally {
      distLoading = false
    }
  }
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if success}<Alert type="success" message={success} />{/if}

  <div class="max-w-screen-xl mx-auto bg-gray-800 p-4 rounded border border-gray-700 space-y-3">

{#if tdee}
    <div class="bg-gray-700 p-3 rounded text-sm space-y-1">
      <div class="text-emerald-400 font-semibold">Suggested from your data</div>
        <div>Observed TDEE: <span class="text-white font-mono">{Math.round(tdee.observedTdee ?? 0)} kcal</span></div>
        <div>Estimated TDEE: <span class="text-white font-mono">{Math.round(tdee.estimatedTdee ?? 0)} kcal</span></div>
        <div class="text-gray-400 text-xs">{tdee.confidence ?? ''} confidence · {tdee.daysOfData ?? 0} days of data</div>
    <button
      class="mt-2 text-xs text-emerald-400 underline"
      onclick={() => { targets.calories = String(Math.round(tdee.observedTdee ?? tdee.estimatedTdee ?? 0)) }}
    >Use observed TDEE as calorie target</button>
  </div>
{/if}

  <button class="bg-cyan-700 px-3 py-2 rounded-lg text-sm hover:bg-cyan-600 w-full mb-3" onclick={autoDistribute} disabled={distLoading}>
    {distLoading ? 'Calculating…' : '🤖 Auto-Calculate Macros'}
  </button>

  <div class="grid grid-cols-2 gap-3">
    <div>
      <label class="text-xs text-gray-400" for="tgt-calories">Calories (kcal)</label>
      <input class="input" id="tgt-calories" type="number" placeholder="e.g. 2200" bind:value={targets.calories} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-protein">Protein (g)</label>
       <input class="input" id="tgt-protein" type="number" placeholder="e.g. 180" bind:value={targets.proteinG} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-carbs">Carbs (g)</label>
       <input class="input" id="tgt-carbs" type="number" placeholder="e.g. 220" bind:value={targets.carbsG} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-fat">Fat (g)</label>
       <input class="input" id="tgt-fat" type="number" placeholder="e.g. 70" bind:value={targets.fatG} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-fiber">Fiber (g)</label>
       <input class="input" id="tgt-fiber" type="number" placeholder="e.g. 30" bind:value={targets.fiberG} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-water">Water (ml)</label>
       <input class="input" id="tgt-water" type="number" placeholder="e.g. 2500" bind:value={targets.waterMl} />
    </div>
  </div>

  <div class="flex items-center gap-1">
    <label class="flex items-center space-x-2 text-sm text-gray-300">
      <input type="checkbox" bind:checked={targets.eatBackExercise} />
      <span>Eat back exercise calories</span>
    </label>
    <InfoTip label="eat-back" text="When on, calories burned in workouts are added back to your daily target. Off = burn them and don't replace them." />
  </div>

  <div>
    <button class="btn-primary" onclick={submit} disabled={loading}>{loading ? 'Saving…' : 'Save Targets'}</button>
  </div>
</div>

<div class="max-w-screen-xl mx-auto mt-4 bg-gray-800 p-4 rounded border border-gray-700">
  <h3 class="text-sm font-bold text-gray-300 mb-3">Target change timeline</h3>
  {#if historyLoading}
    <div class="text-sm text-gray-500">Loading…</div>
  {:else if timeline.length === 0}
    <p class="text-sm text-gray-500">No target changes yet — save a new target to start the timeline.</p>
  {:else}
    <ul class="space-y-2">
      {#each timeline as row}
        <li class="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm border-t border-gray-700/60 first:border-t-0 first:pt-0 pt-2">
          <span class="font-mono text-xs text-gray-500 shrink-0">{row.date}</span>
          <span class="font-mono font-semibold {deltaClass(row.delta)} shrink-0">{signed(row.delta)} kcal</span>
          <span class="text-xs text-gray-500 font-mono">{Math.round(row.fromCal)} → <span class="text-gray-300">{Math.round(row.toCal)}</span></span>
          {#if row.waterChanged}
            <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-cyan-500/15 text-cyan-400">water→{waterLabel(row.waterTo)}</span>
          {/if}
          {#if row.eatBackChanged}
            <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-violet-500/15 text-violet-300">eat-back {row.eatBackOn ? 'on' : 'off'}</span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
