<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { daysAgo, today, dispLoad, loadUnit } from '../lib/utils.js'
  import Spinner from '../components/Spinner.svelte'

  let data = $state([])
  let loading = $state(true)

  async function load() {
    loading = true
    try {
      data = await api.listWorkouts(daysAgo(30), today())
    } catch (e) {
      data = []
    } finally {
      loading = false
    }
  }

  onMount(load)
</script>

{#if loading}
  <Spinner />
{:else}
  <div class="space-y-3">
    {#each data as day}
      <div class="bg-gray-800 p-3 rounded border border-gray-700">
        <div class="font-bold">{day.date}</div>
        {#each day.workouts as w}
          <div class="mt-2 p-2 bg-gray-700 rounded">
            <div class="font-semibold">{w.title} ({w.duration_min} min)</div>
            {#if w.exercises}
              <ul class="mt-1 ml-3 list-disc">
                {#each w.exercises as ex}
                  <li>{ex.name} — {ex.sets}x{ex.reps} @ {ex.load_raw ? ex.load_raw : `${dispLoad(ex.load_kg, store.units)} ${loadUnit(store.units)}`}</li>
                {/each}
              </ul>
            {/if}
          </div>
        {/each}
      </div>
    {/each}
  </div>
{/if}
