<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { fmt0, fmt1, today, dispWeight, weightUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import VitalsRing from '../components/VitalsRing.svelte'
  import WeightSparkline from '../components/WeightSparkline.svelte'

  let data = $state(null)
  let loading = $state(true)

  async function load() {
    loading = true
    try {
      data = await api.getDashboard()
    } catch (e) {
      console.error('Dashboard load failed:', e)
      data = null
    } finally {
      loading = false
    }
  }

  onMount(load)

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
      value: (data.todayBio?.gripKg ?? 0) > 0 ? `${fmt1(data.todayBio.gripKg)} kg` : 'Pending'
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

// (workout streak is now provided by the API in data.weekly_stats.current_streak)
</script>

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
            <VitalsRing
            consumed={data.today?.consumed?.calories ?? 0}
            target={data.today?.targets?.calories ?? 0}
            size={160}
            dailyLogged={data.weeklyStats?.dailyLogged ?? []}
            currentStreak={data.weeklyStats?.currentStreak ?? 0}
            longestStreak={data.weeklyStats?.longestStreak ?? 0}
          />
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
              <div class="text-cyan-400 font-semibold flex items-center gap-1">
                {data.weeklyStats?.currentStreak ?? 0}
                {#if data.weeklyStats?.todayLogged}
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
          </div>
        </Card>

        <Card title="Readiness">
          {@const level = data.readiness?.level ?? 'green'}
          {@const velocity = data.readiness?.velocityTrend ?? 'stable'}
          {@const bulbColor = level === 'green' ? 'bg-green-500' : level === 'yellow' ? 'bg-yellow-400' : 'bg-red-500'}
          {@const arrowIcon = velocity === 'improving' ? '↑' : velocity === 'declining' ? '↓' : '→'}
          {@const arrowColor = velocity === 'improving' ? 'text-green-400' : velocity === 'declining' ? 'text-red-400' : 'text-gray-400'}
          <div class="flex items-center justify-center gap-4">
              <div class="flex flex-col items-center">
                <span class="inline-block w-6 h-6 rounded-full {bulbColor} shadow-[0_0_8px_rgba(0,0,0,0.5)]"></span>
                <span class="text-xs text-gray-400 mt-1">Rz: {(data.readiness?.rz ?? 0).toFixed(2)}</span>
              </div>
            <span class="text-2xl font-bold {arrowColor}">{arrowIcon}</span>
          </div>
          <div class="text-xs text-gray-400 mt-2 text-center">Trend: <span class="font-semibold">{velocity}</span></div>
        </Card>
      </div>
    </div>

    <!-- Row 2: Weight Trend + Weekly Stats -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <Card title="Weight Trend (30 days)">
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
