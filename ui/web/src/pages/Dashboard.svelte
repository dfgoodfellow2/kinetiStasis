<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { fmt0, fmt1, today, dispWeight, weightUnit, dispLoad, loadUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import VitalsRing from '../components/VitalsRing.svelte'
  import StreakRing from '../components/StreakRing.svelte'
  import WeightSparkline from '../components/WeightSparkline.svelte'
  import Alert from '../components/Alert.svelte'
  import InfoTip from '../components/InfoTip.svelte'

  let data = $state(null)
  let loading = $state(true)
  let error = $state('')

  // Water quick-add state (session-only, one undo level).
  let waterBusy = $state(false)
  let waterError = $state('')
  let waterUndo = $state(false)

  // `silent` refreshes the payload in place without blanking the page with the
  // initial-load spinner (used after the water quick-add / undo).
  async function load(silent = false) {
    if (!silent) loading = true
    error = ''
    try {
      data = await api.getDashboard()
    } catch (e) {
      const msg = 'Dashboard load failed: ' + (e.message || e)
      if (silent) waterError = msg
      else {
        error = msg
        data = null
      }
    } finally {
      if (!silent) loading = false
    }
  }

  onMount(() => load())

  // Optimistically move the water figure so the bar responds immediately; the
  // silent refresh then reconciles with the server.
  function bumpWater(delta) {
    if (!data) return
    if (!data.today) data.today = {}
    if (!data.today.consumed) data.today.consumed = {}
    data.today.consumed.waterMl = Math.max(0, (data.today.consumed.waterMl ?? 0) + delta)
  }

  async function addWater() {
    waterError = ''
    waterUndo = false
    waterBusy = true
    try {
      // POST merges additively: +250 ml into today's log.
      await api.postNutrition({ date: today(), waterMl: 250 })
      bumpWater(250)
      waterUndo = true
      await load(true)
    } catch (e) {
      waterError = 'Water add failed: ' + (e.message || e)
    } finally {
      waterBusy = false
    }
  }

  async function undoWater() {
    waterError = ''
    waterBusy = true
    try {
      // Full-replace using the fetched day so nothing else is touched.
      const day = await api.getNutrition(today())
      if (!day) throw new Error('no nutrition log for today')
      await api.putNutrition(today(), {
        ...day,
        waterMl: Math.max(0, (day.waterMl || 0) - 250),
      })
      waterUndo = false
      await load(true)
    } catch (e) {
      waterError = 'Undo failed: ' + (e.message || e)
      waterUndo = false
    } finally {
      waterBusy = false
    }
  }

  // ── Weight trend (insight layer) ────────────────────────────────────────────
  // Contract: dashboard supplies weightTrendKg (EMA level, 0 = insufficient),
  // weightRateKgPerWeek (negative = losing), trendDays, loggingStreak,
  // longestStreak and loggedDays (60 bools, oldest→newest, last = today).
  let weightPoints = $derived(
    (data?.weightTrend ?? []).filter(p => p && isFinite(p.weightKg))
  )
  let latestRawKg = $derived.by(() => {
    if (!weightPoints.length) return data?.todayBio?.weightKg ?? 0
    const sorted = [...weightPoints].sort((a, b) => a.date.localeCompare(b.date))
    return sorted[sorted.length - 1].weightKg
  })
  let trendKg = $derived(Number(data?.weightTrendKg ?? 0))
  let rateKgWk = $derived(Number(data?.weightRateKgPerWeek ?? 0))
  let trendDays = $derived(Number(data?.trendDays ?? 0))
  let trendReady = $derived(trendKg !== 0 && trendDays >= 7)
  let goalLabel = $derived(String(data?.macros?.goalLabel ?? '').toLowerCase())

  // Colour the rate by whether it moves toward the user's goal. Falls back to
  // neutral gray when the goal is unknown or maintenance.
  function rateColor(kg) {
    if (!trendReady || kg === 0) return 'text-gray-400'
    if (goalLabel.includes('cut')) return kg < 0 ? 'text-emerald-400' : 'text-red-400'
    if (goalLabel.includes('bulk')) return kg > 0 ? 'text-emerald-400' : 'text-red-400'
    return 'text-gray-400'
  }

  // Signed display value with a true minus sign; keeps units conversion.
  function signedWeight(kg) {
    if (!isFinite(kg)) return '—'
    const v = dispWeight(Math.abs(kg), store.units)
    const sign = kg > 0 ? '+' : kg < 0 ? '−' : ''
    return `${sign}${v}`
  }

  // Prefer the new insight fields; fall back to legacy weeklyStats so the UI
  // still renders against older servers.
  let loggingStreak = $derived(Number(data?.loggingStreak ?? data?.weeklyStats?.currentStreak ?? 0))
  let longestStreak = $derived(Number(data?.longestStreak ?? data?.weeklyStats?.longestStreak ?? 0))
  let loggedDays = $derived(data?.loggedDays ?? data?.weeklyStats?.dailyLogged ?? [])

  // Water is stored in ml; display ml below 1 L, otherwise litres (same as LogMeal labels it in ml).
  function fmtWater(ml) {
    if (ml === undefined || ml === null || !isFinite(ml)) return '—'
    return ml >= 1000 ? `${fmt1(ml / 1000)} L` : `${fmt0(ml)} ml`
  }

  function waterPct(consumed, target) {
    if (!target || target <= 0) return 0
    return Math.max(0, Math.min(100, (consumed / target) * 100))
  }

  // Checklist: each item has a label, whether it's done, and a value string
let checklist = $derived(data ? [
    {
      key: 'Weight',
      done: (data.todayBio?.weightKg ?? 0) > 0,
      value: (data.todayBio?.weightKg ?? 0) > 0 ? `${dispWeight(data.todayBio.weightKg, store.units)} ${weightUnit(store.units)}` : 'Pending'
    },
    {
      key: 'Food',
      done: (data.today?.consumed?.calories ?? 0) > 0,
      value: (data.today?.consumed?.calories ?? 0) > 0 ? `${fmt0(data.today.consumed.calories)} kcal` : 'Pending'
    },
    {
      key: 'Sleep',
      done: (data.todayBio?.sleepHours ?? 0) > 0,
      value: (data.todayBio?.sleepHours ?? 0) > 0 ? `${fmt1(data.todayBio.sleepHours)} hrs` : 'Pending'
    },
    {
      key: 'Grip',
      done: (data.todayBio?.gripKg ?? 0) > 0,
      value: (data.todayBio?.gripKg ?? 0) > 0 ? `${dispLoad(data.todayBio.gripKg, store.units)} ${loadUnit(store.units)}` : 'Pending'
    },
    {
      key: 'BOLT',
      done: (data.todayBio?.boltScore ?? 0) > 0,
      value: (data.todayBio?.boltScore ?? 0) > 0 ? `${fmt0(data.todayBio.boltScore)} s` : 'Pending'
    },
    {
      key: 'Workout',
      done: data.workoutToday ?? false,
      value: (data.workoutToday ?? false) ? 'Done' : 'Pending'
    },
  ] : [])

// (workout streak is provided by the API in data.weeklyStats.currentStreak)
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if waterError}<Alert type="error" message={waterError} />{/if}

{#if loading}
  <Spinner />
{:else if !data}
  <p class="text-gray-400 text-center mt-8">Failed to load dashboard. Try refreshing.</p>
{:else}
  <div class="max-w-screen-xl mx-auto space-y-4">

    <!-- Row 1: Vitals + Checklist + (Targets/Readiness stacked) -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <!-- Vitals Card (same as before) -->
      <Card title="Vitals">
        <div class="flex flex-col items-center gap-3">
          <StreakRing
            loggedDays={loggedDays}
            loggingStreak={loggingStreak}
            longestStreak={longestStreak}
            size={160}
          >
            <!-- VitalsRing's own 30-day ring is disabled; StreakRing owns it. -->
            <VitalsRing
              consumed={data.today?.consumed?.calories ?? 0}
              target={data.today?.targets?.calories ?? 0}
              size={160}
              dailyLogged={[]}
            />
          </StreakRing>
          <!-- Macro summary below ring -->
          <div class="w-full grid grid-cols-4 gap-2 text-center text-sm mt-1">
              <div>
                <div class="text-emerald-400 font-semibold">{fmt0(data.today?.consumed?.proteinG ?? 0)}g</div>
                <div class="text-gray-500 text-xs">Protein</div>
              </div>
              <div>
                <div class="text-blue-400 font-semibold">{fmt1(data.today?.consumed?.carbsG ?? 0)}g</div>
                <div class="text-gray-500 text-xs">Carbs</div>
              </div>
              <div>
                <div class="text-yellow-400 font-semibold">{fmt0(data.today?.consumed?.fatG ?? 0)}g</div>
                <div class="text-gray-500 text-xs">Fat</div>
              </div>
            <div>
              <div class="font-semibold flex items-center gap-1 {loggingStreak >= 30 ? 'text-amber-400' : 'text-cyan-400'}">
                {loggingStreak}
                {#if loggedDays.length && loggedDays[loggedDays.length - 1]}
                  <span class="text-orange-400">🔥</span>
                {/if}
              </div>
              <div class="text-gray-500 text-xs">Streak</div>
            </div>
          </div>
        </div>
      </Card>

      <!-- Checklist Card (same as before) -->
      <Card title="Checklist — Today">
        <ul class="space-y-2">
          {#each checklist as item}
            <li class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="text-lg leading-none">{item.done ? '✅' : '⬜'}</span>
                <span class="text-sm text-gray-200">{item.key}</span>
              </div>
              <span class="text-xs {item.done ? 'text-emerald-400' : 'text-gray-500'}">{item.value}</span>
            </li>
          {/each}
        </ul>
      </Card>

      <!-- Targets + Readiness stacked vertically -->
      <div class="space-y-4">
        <Card title="Targets">
          <div class="space-y-3 text-sm">
            <!-- Calories -->
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Calories</span>
              <span class="font-tabular-nums font-semibold">
                {fmt0(data.today?.consumed?.calories ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.calories != null}<span class="text-orange-400">{fmt0(data.today.targets.calories)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">kcal</span>
              </span>
            </div>
            <!-- Protein (emerald-400 like Vitals) -->
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Protein</span>
              <span class="font-tabular-nums font-semibold">
                {fmt1(data.today?.consumed?.proteinG ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.proteinG != null}<span class="text-emerald-400">{fmt1(data.today.targets.proteinG)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
              </span>
            </div>
            <!-- Carbs (blue-400 like Vitals) -->
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Carbs</span>
              <span class="font-tabular-nums font-semibold">
                {fmt1(data.today?.consumed?.carbsG ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.carbsG != null}<span class="text-blue-400">{fmt1(data.today.targets.carbsG)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
              </span>
            </div>
            <!-- Fat (yellow-400 like Vitals) -->
            <div class="flex justify-between items-center">
              <span class="text-gray-400">Fat</span>
              <span class="font-tabular-nums font-semibold">
                {fmt1(data.today?.consumed?.fatG ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.fatG != null}<span class="text-yellow-400">{fmt1(data.today.targets.fatG)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
              </span>
            </div>
            <!-- Water (cyan-400 accent) -->
            <div>
              <!-- Value row: same pattern as the macro rows so columns line up -->
              <div class="flex justify-between items-center">
                <span class="text-gray-400">Water</span>
                <span class="font-tabular-nums font-semibold">
                  {fmtWater(data.today?.consumed?.waterMl ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.waterMl != null}<span class="text-cyan-400">{fmtWater(data.today.targets.waterMl)}</span>{:else}<span class="text-gray-500">—</span>{/if}
                </span>
              </div>
              <!-- Bar + quick-add chip share one line; the bar takes the rest -->
              <div class="flex items-center gap-2 mt-1.5">
                <div class="flex-1 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                  <div class="h-full bg-cyan-400 rounded-full" style="width: {waterPct(data.today?.consumed?.waterMl ?? 0, data.today?.targets?.waterMl ?? 0)}%"></div>
                </div>
                <button
                  class="text-xs leading-none bg-cyan-900/40 border border-cyan-700 text-cyan-300 hover:bg-cyan-900/70 disabled:opacity-50 disabled:cursor-not-allowed px-2 py-1 rounded-md shrink-0"
                  onclick={addWater}
                  disabled={waterBusy}
                  title="Add 250 ml of water to today"
                >+ 250 ml</button>
              </div>
              {#if waterUndo}
                <button
                  class="text-xs text-cyan-400 hover:text-cyan-300 underline mt-1.5 disabled:opacity-50"
                  onclick={undoWater}
                  disabled={waterBusy}
                >Undo last add</button>
              {/if}
            </div>
          </div>
        </Card>

        <Card>
          <div class="flex items-center gap-1.5 mb-3">
            <h2 class="text-lg font-bold text-emerald-400">Readiness</h2>
            <InfoTip label="readiness" text="How today's grip + BOLT compare to YOUR recent baseline. Small dips are normal; consecutive low days escalate to yellow/red — a red day means back off." />
          </div>
          {@const level = data.readiness?.level ?? 'green'}
          {@const velocity = data.readiness?.velocityTrend ?? 'stable'}
          {@const bulbColor = level === 'green' ? 'bg-green-500' : level === 'yellow' ? 'bg-yellow-400' : 'bg-red-500'}
          {@const arrowIcon = velocity === 'improving' ? '↑' : velocity === 'declining' ? '↓' : '→'}
          {@const arrowColor = velocity === 'improving' ? 'text-green-400' : velocity === 'declining' ? 'text-red-400' : 'text-gray-400'}
          <div class="grid grid-cols-2 gap-3">
            <!-- Left: current readiness display -->
            <div class="flex flex-col items-center justify-center gap-1">
              <div class="flex items-center justify-center gap-3">
                <div class="flex flex-col items-center">
                  <span class="inline-block w-6 h-6 rounded-full {bulbColor} shadow-[0_0_8px_rgba(0,0,0,0.5)]"></span>
                  <span class="text-xs text-gray-400 mt-1">Rz: {(data.readiness?.rz ?? 0).toFixed(2)}</span>
                </div>
                <span class="text-2xl font-bold {arrowColor}">{arrowIcon}</span>
              </div>
              <div class="text-xs text-gray-400">Trend: <span class="font-semibold">{velocity}</span></div>
            </div>
            <!-- Right: SD / EMA values -->
            <div class="border-l border-gray-700 pl-3 space-y-2 text-xs">
              <div>
                <div class="text-gray-500 mb-0.5">Grip</div>
                <div class="font-tabular-nums text-gray-200">
                  SD: <span class="font-semibold">{(data.readiness?.gripSD ?? 0).toFixed(2)}</span>
                  <span class="text-gray-600 mx-1">|</span>
                  EMA: <span class="font-semibold">{(data.readiness?.gripEMA ?? 0).toFixed(2)}</span>
                </div>
              </div>
              <div>
                <div class="text-gray-500 mb-0.5">BOLT</div>
                <div class="font-tabular-nums text-gray-200">
                  SD: <span class="font-semibold">{(data.readiness?.boltSD ?? 0).toFixed(2)}</span>
                  <span class="text-gray-600 mx-1">|</span>
                  EMA: <span class="font-semibold">{(data.readiness?.boltEMA ?? 0).toFixed(2)}</span>
                </div>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>

    <!-- Row 2: Weight Trend + Weekly Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <Card title="Weight Trend (30 days)">
        <div class="flex flex-col items-center text-center mb-2">
          <div class="flex items-center justify-center gap-1.5">
            {#if trendReady}
              <!-- Primary: server EMA trend level. Raw scale reading is secondary. -->
              <div class="text-3xl font-bold text-gray-100">
                {dispWeight(trendKg, store.units)}
                <span class="text-sm font-normal text-gray-400">{weightUnit(store.units)}</span>
              </div>
            {:else}
              <div class="text-lg font-semibold text-gray-300">
                Trend forming
                <span class="text-sm font-normal text-gray-500">({trendDays || weightPoints.length} {trendDays === 1 || (!trendDays && weightPoints.length === 1) ? 'day' : 'days'})</span>
              </div>
            {/if}
            <InfoTip label="trend weight" text="The smoothed weight the engine trusts — raw scale weight bounces with food, water, and salt." />
          </div>
          {#if trendReady}
            <div class="text-sm font-semibold {rateColor(rateKgWk)}">
              {signedWeight(rateKgWk)} {weightUnit(store.units)}/wk
            </div>
          {/if}
          <div class="text-xs text-gray-400 mt-1">
            Raw weight:
            <span class="font-semibold text-gray-200">
              {latestRawKg > 0 ? dispWeight(latestRawKg, store.units) : '—'} {weightUnit(store.units)}
            </span>
          </div>
        </div>
        <WeightSparkline points={data.weightTrend ?? []} units={store.units} />
      </Card>

      <Card title="Weekly Stats">
        <div class="text-sm text-gray-300">Avg Calories: <span class="font-semibold text-gray-100">{fmt0(data.weeklyStats?.avgCalories ?? 0)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Protein: <span class="font-semibold text-gray-100">{fmt0(data.weeklyStats?.avgProteinG ?? 0)}g</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Weight: <span class="font-semibold text-gray-100">{dispWeight(data.weeklyStats?.avgWeightKg ?? 0, store.units)} {weightUnit(store.units)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Workouts: <span class="font-semibold text-gray-100">{data.weeklyStats?.totalWorkouts ?? 0}</span></div>
      </Card>
    </div>

  </div>
{/if}
