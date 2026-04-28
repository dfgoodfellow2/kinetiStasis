<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { fmt0, fmt1, today, dispWeight, weightUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import VitalsRing from '../components/VitalsRing.svelte'

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
</script>

{#if loading}
  <Spinner />
{:else if !data}
  <p class="text-gray-400 text-center mt-8">Failed to load dashboard. Try refreshing.</p>
{:else}
  <div class="space-y-4">

    <!-- Top row: Vitals ring + Checklist -->
    <div class="grid md:grid-cols-2 gap-4">

      <Card title="Vitals">
        <div class="flex flex-col items-center gap-3">
          <VitalsRing
            consumed={data.today?.consumed?.calories ?? 0}
            target={data.today?.targets?.calories ?? 0}
            size={160}
          />
          <!-- Macro summary below ring -->
          <div class="w-full grid grid-cols-3 gap-2 text-center text-sm mt-1">
            <div>
              <div class="text-emerald-400 font-semibold">{fmt0(data.today?.consumed?.protein_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Protein</div>
            </div>
            <div>
              <div class="text-blue-400 font-semibold">{fmt0(data.today?.consumed?.carbs_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Carbs</div>
            </div>
            <div>
              <div class="text-yellow-400 font-semibold">{fmt0(data.today?.consumed?.fat_g ?? 0)}g</div>
              <div class="text-gray-500 text-xs">Fat</div>
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

    <!-- Stats row: TDEE, Readiness, Weekly -->
    <div class="grid md:grid-cols-4 gap-4">
      <Card title="TDEE">
        <div class="text-2xl font-bold text-gray-100">{fmt0(data.tdee?.observed_tdee ?? data.tdee?.estimated_tdee ?? 0)} <span class="text-sm font-normal text-gray-400">kcal</span></div>
        <div class="text-xs text-gray-500 mt-1">{data.tdee?.confidence ?? '—'} confidence · {data.tdee?.method ?? '—'}</div>
      </Card>
      <Card title="Readiness">
        {@const level = data.readiness?.level ?? 'green'}
        {@const message = data.readiness?.message ?? '—'}
        {@const velocity = data.readiness?.velocity_trend ?? 'stable'}
        {@const dotColor = level === 'green' ? 'bg-green-500' : level === 'yellow' ? 'bg-yellow-400' : 'bg-red-500'}
        {@const velocityIcon = velocity === 'improving' ? '↑' : velocity === 'declining' ? '↓' : '→'}
        {@const velocityColor = velocity === 'improving' ? 'text-green-400' : velocity === 'declining' ? 'text-red-400' : 'text-yellow-400'}
        <div class="flex items-center gap-3 mb-2">
          <span class="inline-block w-3 h-3 rounded-full flex-shrink-0 {dotColor}"></span>
          <span class="text-base font-semibold text-gray-100">{message}</span>
          <span class="ml-auto text-lg font-bold {velocityColor}">{velocityIcon}</span>
        </div>
        <div class="text-xs text-gray-500">
          Rz: {(data.readiness?.rz ?? 0).toFixed(2)} · <span class="{velocityColor}">{velocity}</span>
        </div>
        {#if data.readiness?.notes?.length}
          <ul class="mt-2 space-y-1">
            {#each data.readiness.notes as note}
              <li class="text-xs text-yellow-400">⚠ {note}</li>
            {/each}
          </ul>
        {/if}
      </Card>
      <Card title="Weekly Stats">
        <div class="text-sm text-gray-300">Avg Calories: <span class="font-semibold text-gray-100">{fmt0(data.weekly_stats?.avg_calories ?? 0)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Protein: <span class="font-semibold text-gray-100">{fmt0(data.weekly_stats?.avg_protein_g ?? 0)}g</span></div>
        <div class="text-sm text-gray-300 mt-1">Avg Weight: <span class="font-semibold text-gray-100">{dispWeight(data.weekly_stats?.avg_weight_kg ?? 0, store.units)} {weightUnit(store.units)}</span></div>
        <div class="text-sm text-gray-300 mt-1">Workouts: <span class="font-semibold text-gray-100">{data.weekly_stats?.total_workouts ?? 0}</span></div>
      </Card>
      <Card title="Personal Bests">
        <div class="text-sm text-gray-300">Grip (120d): <span class="font-semibold text-emerald-400">{dispWeight(data.grip_personal_best ?? 0, store.units)} {weightUnit(store.units)}</span></div>
      </Card>
    </div>

  </div>
{/if}
