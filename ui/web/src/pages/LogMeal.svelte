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
  let isEdit = $state(false)

  let aiText = $state('')
  let aiResult = $state(null)
  let aiLoading = $state(false)

  // Saved meals (quick add) & templates
  let savedMeals = $state([])
  let savedLoading = $state(false)
  let showSaveMeal = $state(false)
  let saveMealName = $state('')
  let savingMeal = $state(false)
  let templates = $state([])
  let templatesLoading = $state(false)
  let openTemplates = $state([])

  let recentSaved = $derived(
    [...savedMeals]
      .sort((a, b) => String(b.createdAt ?? '').localeCompare(String(a.createdAt ?? '')))
      .slice(0, 5)
  )

  async function loadSaved() {
    savedLoading = true
    try {
      savedMeals = (await api.listSaved()) || []
    } catch (e) {
      error = e.message
    } finally {
      savedLoading = false
    }
  }

  async function loadTemplates() {
    templatesLoading = true
    try {
      templates = (await api.listTemplates()) || []
    } catch (e) {
      error = e.message
    } finally {
      templatesLoading = false
    }
  }

  // Fill the manual form from a saved meal / template entry. The date is kept
  // as-is so quick-add applies to whichever day is currently selected.
  function applySavedMeal(m) {
    form = {
      ...form,
      calories: String(m.calories ?? ''),
      proteinG: String(m.proteinG ?? ''),
      carbsG: String(m.carbsG ?? ''),
      fatG: String(m.fatG ?? ''),
      fiberG: String(m.fiberG ?? ''),
    }
    success = `Filled from "${m.name}"`
    error = ''
  }

  async function saveAsMeal() {
    const name = saveMealName.trim()
    if (!name) {
      error = 'Enter a name for the saved meal'
      return
    }
    error = ''
    savingMeal = true
    try {
      // Snapshot the current form values (including anything just filled by
      // quick-add or an AI parse applied to the form).
      const created = await api.postSaved({
        name,
        calories: Number(form.calories) || 0,
        proteinG: Number(form.proteinG) || 0,
        carbsG: Number(form.carbsG) || 0,
        fatG: Number(form.fatG) || 0,
        fiberG: Number(form.fiberG) || 0,
      })
      savedMeals = [...savedMeals, created]
      saveMealName = ''
      showSaveMeal = false
      success = `Saved "${created.name || name}"`
    } catch (e) {
      error = e.message
    } finally {
      savingMeal = false
    }
  }

  async function removeSavedMeal(m) {
    if (!confirm(`Delete saved meal "${m.name}"?`)) return
    error = ''
    try {
      await api.deleteSaved(m.id)
      savedMeals = savedMeals.filter((x) => x.id !== m.id)
    } catch (e) {
      error = e.message
    }
  }

  function toggleTemplate(id) {
    openTemplates = openTemplates.includes(id)
      ? openTemplates.filter((x) => x !== id)
      : [...openTemplates, id]
  }

  onMount(() => {
    loadSaved()
    loadTemplates()
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
      isEdit = true
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
      if (isEdit) {
        // Editing replaces the whole day's log, so PUT to the date path.
        await api.putNutrition(form.date, payload)
        success = 'Updated'
        // The edit has been applied, so subsequent saves should create new logs.
        isEdit = false
        // Keep the form on the edited date so the user retains date context.
      } else {
        await api.postNutrition(payload)
        success = 'Saved'
        form = { date: today(), calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '', mealNotes: '' }
      }
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
    <Card title="Quick Add">
      <div class="flex items-start justify-between gap-2 mb-3">
        <p class="text-xs text-gray-400">
          Tap a saved meal to fill this form for {form.date || 'the selected date'}.
        </p>
        <button
          class="bg-gray-700 px-3 py-1.5 rounded-lg text-sm shrink-0"
          onclick={() => (showSaveMeal = !showSaveMeal)}
        >
          {showSaveMeal ? 'Cancel' : '+ Save as meal'}
        </button>
      </div>

      {#if showSaveMeal}
        <div class="flex gap-2 mb-3">
          <input
            class="input flex-1"
            placeholder="Meal name (e.g. Overnight oats)"
            bind:value={saveMealName}
            onkeydown={(e) => { if (e.key === 'Enter') saveAsMeal() }}
          />
          <button class="btn-primary" onclick={saveAsMeal} disabled={savingMeal}>
            {savingMeal ? 'Saving…' : 'Save'}
          </button>
        </div>
      {/if}

      {#if savedLoading}
        <p class="text-xs text-gray-400">Loading saved meals…</p>
      {:else if recentSaved.length === 0}
        <p class="text-xs text-gray-400">No saved meals yet — save one from this form</p>
      {:else}
        <div class="flex flex-wrap gap-2">
          {#each recentSaved as m (m.id)}
            <div class="inline-flex items-center bg-gray-700 rounded-full overflow-hidden">
              <button
                class="px-3 py-1.5 text-sm hover:bg-gray-600"
                title="Fill form with {m.name}"
                onclick={() => applySavedMeal(m)}
              >
                {m.name} <span class="text-gray-400">· {Math.round(m.calories ?? 0)} kcal</span>
              </button>
              <button
                class="pl-1 pr-2.5 py-1.5 text-xs text-gray-400 hover:text-red-400"
                title="Delete {m.name}"
                aria-label="Delete {m.name}"
                onclick={() => removeSavedMeal(m)}
              >
                ✕
              </button>
            </div>
          {/each}
        </div>
        {#if savedMeals.length > recentSaved.length}
          <p class="text-xs text-gray-500 mt-2">
            Showing 5 of {savedMeals.length} saved meals.
          </p>
        {/if}
      {/if}
    </Card>

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

    <Card title="Templates">
      {#if templatesLoading}
        <p class="text-xs text-gray-400">Loading templates…</p>
      {:else if templates.length === 0}
        <p class="text-xs text-gray-400">No templates yet</p>
      {:else}
        <div class="space-y-2">
          {#each templates as t (t.id)}
            <div class="border border-gray-700 rounded-lg overflow-hidden">
              <button
                class="w-full flex items-center justify-between px-3 py-2 bg-gray-700/40 hover:bg-gray-700 text-left"
                onclick={() => toggleTemplate(t.id)}
              >
                <span class="text-sm font-medium">{t.name}</span>
                <span class="text-xs text-gray-400">
                  {t.meals?.length ?? 0} meals {openTemplates.includes(t.id) ? '▾' : '▸'}
                </span>
              </button>
              {#if openTemplates.includes(t.id)}
                <div class="p-2 flex flex-col gap-1">
                  {#if !t.meals || t.meals.length === 0}
                    <p class="text-xs text-gray-400 px-1">No meals in this template</p>
                  {:else}
                    {#each t.meals as m, i (m.id ?? i)}
                      <button
                        class="flex items-center justify-between px-2 py-1.5 rounded hover:bg-gray-700 text-left"
                        onclick={() => applySavedMeal(m)}
                      >
                        <span class="text-sm">{m.name}</span>
                        <span class="text-xs text-gray-400">{Math.round(m.calories ?? 0)} kcal</span>
                      </button>
                    {/each}
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
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
