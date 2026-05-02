<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import Alert from '../components/Alert.svelte'

  let targets = $state({ calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '' })
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
        proteinG: Number(targets.proteinG) || 0,
        carbsG:   Number(targets.carbsG)   || 0,
        fatG:     Number(targets.fatG)     || 0,
        fiberG:   Number(targets.fiberG)   || 0,
        waterMl:  Number(targets.waterMl)  || 0,
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

  <div class="max-w-screen-xl mx-auto bg-gray-800 p-4 rounded border border-gray-700 space-y-3">

  {#if tdee}
    <div class="bg-gray-700 p-3 rounded text-sm space-y-1">
      <div class="text-emerald-400 font-semibold">Suggested from your data</div>
        <div>Observed TDEE: <span class="text-white font-mono">{Math.round(tdee.observedTdee ?? 0)} kcal</span></div>
        <div>Estimated TDEE: <span class="text-white font-mono">{Math.round(tdee.estimatedTdee ?? 0)} kcal</span></div>
        <div class="text-gray-400 text-xs">{tdee.confidence ?? ''} confidence · {tdee.daysOfData ?? 0} days of data</div>
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

  <div>
    <button class="btn-primary" onclick={submit} disabled={loading}>{loading ? 'Saving…' : 'Save Targets'}</button>
  </div>
</div>
