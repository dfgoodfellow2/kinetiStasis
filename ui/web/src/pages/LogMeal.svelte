<script>
  import { onMount } from 'svelte'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { api } from '../lib/api.js'
  import { today } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  let mode = $state('ai')
  let form = $state({ date: today(), calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '', mealNotes: '' })
  let loading = $state(false)
  let error = $state('')
  let success = $state('')

  let aiText = $state('')
  let aiResult = $state(null)
  let aiLoading = $state(false)

  onMount(() => {
    // Check if we're editing an existing nutrition log
    if (store.editData && store.editData.type === 'nutrition') {
      const row = store.editData.data
        form = { 
        date: row.date, 
        calories: String(row.calories ?? ''), 
        proteinG: String(row.proteinG ?? ''), 
        carbsG: String(row.carbsG ?? ''), 
        fatG: String(row.fatG ?? ''), 
        fiberG: String(row.fiberG ?? ''), 
        waterMl: String(row.waterMl ?? ''), 
        mealNotes: row.mealNotes ?? '' 
      }
      mode = 'manual'
      clearEditData()
    }
  })

  async function submitManual() {
    error = ''
    success = ''
    loading = true
    try {
      const payload = {
        date: form.date,
        calories: Number(form.calories) || 0,
        proteinG: Number(form.proteinG) || 0,
        carbsG: Number(form.carbsG) || 0,
        fatG: Number(form.fatG) || 0,
        fiberG: Number(form.fiberG) || 0,
        waterMl: Number(form.waterMl) || 0,
        mealNotes: form.mealNotes || '',
      }
      await api.postNutrition(payload)
      success = 'Saved'
      form = { date: today(), calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '', mealNotes: '' }
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function parseAI() {
    aiLoading = true
    aiResult = null
    try {
      aiResult = await api.parseMeal(aiText)
    } catch (e) {
      error = e.message
    } finally {
      aiLoading = false
    }
  }

  async function saveParsed() {
    if (!aiResult) return
    try {
      // Fix: use rawInput (camelCase) not raw_input
    const { rawInput, ...payload } = aiResult
    if (!payload.date) payload.date = today()
    // payload should already be camelCase; trust camelCase fields only
    payload.proteinG = payload.proteinG ?? 0
    payload.carbsG = payload.carbsG ?? 0
    payload.fatG = payload.fatG ?? 0
    payload.fiberG = payload.fiberG ?? 0
    payload.waterMl = payload.waterMl ?? 0
    payload.mealNotes = rawInput || payload.mealNotes || ''
    await api.postNutrition(payload)
      success = 'Saved parsed meal'
      aiResult = null
      aiText = ''
    } catch (e) {
      error = e.message
    }
  }
</script>

<div class="max-w-screen-xl mx-auto">
  <div class="flex space-x-2 mb-4">
    <button class={mode==='manual' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => mode = 'manual'}>Manual</button>
    <button class={mode==='ai' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => mode = 'ai'}>AI Parse</button>
  </div>

  {#if error}<Alert type="error" message={error} />{/if}
  {#if success}<Alert type="success" message={success} />{/if}

  {#if mode === 'manual'}
    <Card title="Manual Entry">
      <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        <!-- Date Field -->
        <div class="md:col-span-2">
          <label class="text-xs text-gray-400" for="lm-date">Date</label>
          <input class="input" id="lm-date" type="date" bind:value={form.date} />
        </div>

        <!-- Calories Field -->
        <div class="md:col-span-2">
          <label class="text-xs text-gray-400" for="lm-calories">Calories</label>
          <input class="input" id="lm-calories" type="number" placeholder="Calories" bind:value={form.calories} />
        </div>

        <!-- Macro Fields (3-column grid for Protein/Carbs/Fat) -->
        <div class="md:col-span-2">
          <div class="grid grid-cols-3 gap-3">
            <div>
              <label class="text-xs text-gray-400" for="lm-protein">Protein (g)</label>
              <input class="input" id="lm-protein" type="number" placeholder="Protein (g)" bind:value={form.proteinG} />
            </div>
            <div>
              <label class="text-xs text-gray-400" for="lm-carbs">Carbs (g)</label>
              <input class="input" id="lm-carbs" type="number" placeholder="Carbs (g)" bind:value={form.carbsG} />
            </div>
            <div>
              <label class="text-xs text-gray-400" for="lm-fat">Fat (g)</label>
              <input class="input" id="lm-fat" type="number" placeholder="Fat (g)" bind:value={form.fatG} />
            </div>
          </div>
        </div>

        <!-- Fiber/Water (2-column grid) -->
        <div class="md:col-span-2">
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-gray-400" for="lm-fiber">Fiber (g)</label>
              <input class="input" id="lm-fiber" type="number" placeholder="Fiber (g)" bind:value={form.fiberG} />
            </div>
            <div>
              <label class="text-xs text-gray-400" for="lm-water">Water (ml)</label>
              <input class="input" id="lm-water" type="number" placeholder="Water (ml)" bind:value={form.waterMl} />
            </div>
          </div>
        </div>

        <!-- Notes Field -->
        <div class="md:col-span-2">
          <label class="text-xs text-gray-400" for="lm-notes">Notes</label>
          <textarea class="input" id="lm-notes" placeholder="Notes" bind:value={form.mealNotes}></textarea>
        </div>

        <!-- Save Button -->
        <div class="md:col-span-2">
          <button class="btn-primary" onclick={submitManual} disabled={loading}>{loading? 'Saving…' : 'Save'}</button>
        </div>
      </div>
    </Card>
  {:else}
    <Card title="AI Parse">
      <textarea class="input" rows="6" placeholder="Describe your meal..." bind:value={aiText}></textarea>
        <div class="flex space-x-2 mt-2">
        <button class="btn-primary" onclick={parseAI} disabled={aiLoading}>{aiLoading? 'Parsing…' : 'Parse'}</button>
      </div>
      {#if aiLoading}<Spinner />{/if}
      {#if aiResult}
        <Card title="Parsed Result">
          <pre class="whitespace-pre-wrap">{JSON.stringify(aiResult, null, 2)}</pre>
            <div class="flex space-x-2 mt-2">
            <button class="btn-primary" onclick={saveParsed}>Save</button>
            <button class="bg-gray-700 px-3 py-2 rounded-lg" onclick={() => aiResult = null}>Discard</button>
          </div>
        </Card>
      {/if}
    </Card>
  {/if}
</div>
