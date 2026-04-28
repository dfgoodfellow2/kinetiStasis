<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import Alert from '../components/Alert.svelte'

  let targets = $state({ calories: '', protein_g: '', carbs_g: '', fat_g: '', fiber_g: '', water_ml: '' })
  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let tdee = $state(null)

  onMount(async () => {
    try {
      const t = await api.getTargets()
      if (t) targets = { ...targets, ...t }
    } catch {}
    try {
      const res = await api.getTDEE(30)
      tdee = res
    } catch {}
  })

  async function submit() {
    error = ''
    success = ''
    loading = true
    try {
      await api.putTargets({
        calories:  Number(targets.calories)  || 0,
        protein_g: Number(targets.protein_g) || 0,
        carbs_g:   Number(targets.carbs_g)   || 0,
        fat_g:     Number(targets.fat_g)     || 0,
        fiber_g:   Number(targets.fiber_g)   || 0,
        water_ml:  Number(targets.water_ml)  || 0,
      })
      success = 'Targets updated'
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if success}<Alert type="success" message={success} />{/if}

<div class="max-w-2xl mx-auto bg-gray-800 p-4 rounded border border-gray-700 space-y-3">

  {#if tdee}
    <div class="bg-gray-700 p-3 rounded text-sm space-y-1">
      <div class="text-emerald-400 font-semibold">Suggested from your data</div>
      <div>Observed TDEE: <span class="text-white font-mono">{Math.round(tdee.observed_tdee ?? 0)} kcal</span></div>
      <div>Estimated TDEE: <span class="text-white font-mono">{Math.round(tdee.estimated_tdee ?? 0)} kcal</span></div>
      <div class="text-gray-400 text-xs">{tdee.confidence ?? ''} confidence · {tdee.days_of_data ?? 0} days of data</div>
      <button
        class="mt-2 text-xs text-emerald-400 underline"
        onclick={() => { targets.calories = String(Math.round(tdee.observed_tdee ?? tdee.estimated_tdee ?? 0)) }}
      >Use observed TDEE as calorie target</button>
    </div>
  {/if}

  <div class="grid grid-cols-2 gap-3">
    <div>
      <label class="text-xs text-gray-400" for="tgt-calories">Calories (kcal)</label>
      <input class="input" id="tgt-calories" type="number" placeholder="e.g. 2200" bind:value={targets.calories} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-protein">Protein (g)</label>
      <input class="input" id="tgt-protein" type="number" placeholder="e.g. 180" bind:value={targets.protein_g} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-carbs">Carbs (g)</label>
      <input class="input" id="tgt-carbs" type="number" placeholder="e.g. 220" bind:value={targets.carbs_g} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-fat">Fat (g)</label>
      <input class="input" id="tgt-fat" type="number" placeholder="e.g. 70" bind:value={targets.fat_g} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-fiber">Fiber (g)</label>
      <input class="input" id="tgt-fiber" type="number" placeholder="e.g. 30" bind:value={targets.fiber_g} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="tgt-water">Water (ml)</label>
      <input class="input" id="tgt-water" type="number" placeholder="e.g. 2500" bind:value={targets.water_ml} />
    </div>
  </div>

  <div>
    <button class="btn-primary" onclick={submit} disabled={loading}>{loading ? 'Saving…' : 'Save Targets'}</button>
  </div>
</div>
