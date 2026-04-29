<script>
  import { onMount } from 'svelte'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { api } from '../lib/api.js'
  import { today } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  let mode = $state('ai')
  let form = $state({ date: today(), calories: '', protein: '', carbs: '', fat: '', fiber: '', water_ml: '', notes: '' })
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
        protein: String(row.protein_g ?? ''), 
        carbs: String(row.carbs_g ?? ''), 
        fat: String(row.fat_g ?? ''), 
        fiber: String(row.fiber_g ?? ''), 
        water_ml: String(row.water_ml ?? ''), 
        notes: row.meal_notes ?? '' 
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
      const payload = { ...form, calories: Number(form.calories) }
      await api.postNutrition(payload)
      success = 'Saved'
      form = { date: today(), calories: '', protein: '', carbs: '', fat: '', fiber: '', water_ml: '', notes: '' }
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
    const { raw_input, ...payload } = aiResult
    if (!payload.date) payload.date = today()
    payload.meal_notes = raw_input || ''
    await api.postNutrition(payload)
      success = 'Saved parsed meal'
      aiResult = null
      aiText = ''
    } catch (e) {
      error = e.message
    }
  }
</script>

<div class="max-w-2xl mx-auto">
  <div class="flex space-x-2 mb-4">
    <button class={mode==='manual' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => mode = 'manual'}>Manual</button>
    <button class={mode==='ai' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => mode = 'ai'}>AI Parse</button>
  </div>

  {#if error}<Alert type="error" message={error} />{/if}
  {#if success}<Alert type="success" message={success} />{/if}

  {#if mode === 'manual'}
    <Card title="Manual Entry">
      <div class="space-y-3">
        <input class="input" bind:value={form.date} />
        <input class="input" placeholder="Calories" bind:value={form.calories} />
        <input class="input" placeholder="Protein (g)" bind:value={form.protein} />
        <input class="input" placeholder="Carbs (g)" bind:value={form.carbs} />
        <input class="input" placeholder="Fat (g)" bind:value={form.fat} />
        <input class="input" placeholder="Fiber (g)" bind:value={form.fiber} />
        <input class="input" placeholder="Water (ml)" bind:value={form.water_ml} />
        <textarea class="input" placeholder="Notes" bind:value={form.notes}></textarea>
        <div class="flex">
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
