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
      done: (data.today_bio?.weight_kg ?? 0) > 0,
      value: (data.today_bio?.weight_kg ?? 0) > 0 ? `${dispWeight(data.today_bio.weight_kg, store.units)} ${weightUnit(store.units)}` : 'Pending'
    },
    {
      key: 'Food',
      done: (data.today?.consumed?.calories ?? 0) > 0,
      value: (data.today?.consumed?.calories ?? 0) > 0 ? `${fmt0(data.today.consumed.calories)} kcal` : 'Pending'
    },
    {
      key: 'Sleep',
      done: (data.today_bio?.sleep_hours ?? 0) > 0,
      value: (data.today_bio?.sleep_hours ?? 0) > 0 ? `${fmt1(data.today_bio.sleep_hours)} hrs` : 'Pending'
    },
    {
      key: 'Grip',
      done: (data.today_bio?.grip_kg ?? 0) > 0,
      value: (data.today_bio?.grip_kg ?? 0) > 0 ? `${fmt1(data.today_bio.grip_kg)} kg` : 'Pending'
    },
    {
      key: 'BOLT',
      done: (data.today_bio?.bolt_score ?? 0) > 0,
      value: (data.today_bio?.bolt_score ?? 0) > 0 ? `${fmt0(data.today_bio.bolt_score)} s` : 'Pending'
    },
    {
      key: 'Workout',
      done: data.workout_today ?? false,
      value: (data.workout_today ?? false) ? 'Done' : 'Pending'
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

    <!-- Top row: Vitals ring + Checklist -->
    <div class="grid md:grid-cols-2 gap-4">

      <Card title="Vitals">
        <div class="flex flex-col items-center gap-3">
          <VitalsRing
            consumed={data.today?.consumed?.calories ?? 0}
            target={data.today?.targets?.calories ?? 0}
            size={160}
            dailyLogged={data.weekly_stats?.daily_logged ?? []}
            currentStreak={data.weekly_stats?.current_streak ?? 0}
            longestStreak={data.weekly_stats?.longest_streak ?? 0}
          />
          <!-- Macro summary below ring -->
          <div class="w-full grid grid-cols-4 gap-2 text-center text-sm mt-1">
            <div>
              <div class="text-emerald-400 font-semibold">{fmt0(data.today?.consumed?.protein_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Protein</div>
            </div>
            <div>
              <div class="text-blue-400 font-semibold">{fmt1(data.today?.consumed?.carbs_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Carbs</div>
            </div>
            <div>
              <div class="text-yellow-400 font-semibold">{fmt0(data.today?.consumed?.fat_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Fat</div>
            </div>
            <div>
              <div class="text-cyan-400 font-semibold flex items-center gap-1">
                {data.weekly_stats?.current_streak ?? 0}
                {#if data.weekly_stats?.today_logged}
                  <span class="text-orange-400">🔥</span>
                {/if}
              </div>
              <div class="text-gray-500 text-xs">Streak</div>
            </div>
          </div>
        </div>
      </Card>

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

    </div>

    <!-- Stats row: Targets, Readiness, Weekly -->
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
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
              {fmt1(data.today?.consumed?.protein_g ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.protein_g != null}<span class="text-emerald-400">{fmt1(data.today.targets.protein_g)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
            </span>
          </div>
          <!-- Carbs (blue-400 like Vitals) -->
          <div class="flex justify-between items-center">
            <span class="text-gray-400">Carbs</span>
            <span class="font-tabular-nums font-semibold">
              {fmt1(data.today?.consumed?.carbs_g ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.carbs_g != null}<span class="text-blue-400">{fmt1(data.today.targets.carbs_g)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
            </span>
          </div>
          <!-- Fat (yellow-400 like Vitals) -->
          <div class="flex justify-between items-center">
            <span class="text-gray-400">Fat</span>
            <span class="font-tabular-nums font-semibold">
              {fmt1(data.today?.consumed?.fat_g ?? 0)}<span class="text-gray-500"> / </span>{#if data.today?.targets?.fat_g != null}<span class="text-yellow-400">{fmt1(data.today.targets.fat_g)}</span>{:else}<span class="text-gray-500">—</span>{/if}<span class="text-gray-500 text-xs ml-1">g</span>
            </span>
          </div>
        </div>
      </Card>
      <Card title="Readiness">
        {@const level = data.readiness?.level ?? 'green'}
        {@const velocity = data.readiness?.velocity_trend ?? 'stable'}
        {@const bulbColor = level === 'green' ? 'bg-green-500' : level === 'yellow' ? 'bg-yellow-400' : 'bg-red-500'}
        {@const arrowIcon = velocity === 'improving' ? '↑' : velocity === 'declining' ? '↓' : '→'}
        {@const arrowColor = velocity === 'improving' ? 'text-green-400' : velocity === 'declining' ? 'text-red-400' : 'text-gray-400'}
        <div class="flex items-center justify-center gap-4">
          <!-- Colored bulb indicator -->
          <div class="flex flex-col items-center">
            <span class="inline-block w-6 h-6 rounded-full {bulbColor} shadow-[0_0_8px_rgba(0,0,0,0.5)]"></span>
            <span class="text-xs text-gray-400 mt-1">Rz: {(data.readiness?.rz ?? 0).toFixed(2)}</span>
          </div>
          <!-- Direction arrow -->
          <span class="text-2xl font-bold {arrowColor}">{arrowIcon}</span>
        </div>
        <div class="text-xs text-gray-400 mt-2 text-center">Trend: <span class="font-semibold">{velocity}</span></div>
      </Card>
      <Card title="Weekly Stats">
        <div class="text-sm text-gray-300">Avg Calories: <span class="font-semibold text-gray-100">{fmt0(data.weekly_stats?.avg_calories ?? 0)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Protein: <span class="font-semibold text-gray-100">{fmt0(data.weekly_stats?.avg_protein_g ?? 0)}g</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Weight: <span class="font-semibold text-gray-100">{dispWeight(data.weekly_stats?.avg_weight_kg ?? 0, store.units)} {weightUnit(store.units)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Workouts: <span class="font-semibold text-gray-100">{data.weekly_stats?.total_workouts ?? 0}</span></div>
        
      </Card>
      
    </div>

    <!-- Weight Trend row -->
    <div class="grid gap-4">
      <Card title="Weight Trend (30 days)">
        <WeightSparkline points={data.weight_trend ?? []} units={store.units} />
      </Card>
    </div>

  </div>
{/if}
