<script>
  import { onMount } from 'svelte'
  import { store, clearEditData, setCurrentPage } from '../lib/stores.svelte.js'
  import { api } from '../lib/api.js'
  import { today } from '../lib/utils.js'
  import { saveDraft, loadDraft, clearDraft } from '../lib/draft.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  const DRAFT_KEY = 'draft:logmeal:manual'

  function emptyForm() {
    return { date: today(), calories: '', proteinG: '', carbsG: '', fatG: '', fiberG: '', waterMl: '', mealNotes: '' }
  }

  let mode = $state('ai')
  let form = $state(emptyForm())
  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let isEdit = $state(false)
  let editDate = $state(null)
  let draftRestored = $state(false)

  let aiText = $state('')
  let aiResult = $state(null)
  let aiLoading = $state(false)
  // Photo attachment for the AI tab: { imageBase64, mimeType, previewUrl }
  let aiImage = $state(null)
  let aiImageLoading = $state(false)
  let canParse = $derived(aiText.trim().length > 0 || !!aiImage)

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

  // Draft the manual form only (never the AI photo/base64 state).
  function hasDraftContent(f) {
    if (!f || typeof f !== 'object') return false
    return ['calories', 'proteinG', 'carbsG', 'fatG', 'fiberG', 'waterMl', 'mealNotes']
      .some((k) => String(f[k] ?? '').trim() !== '')
  }

  $effect(() => {
    // Never draft an in-progress edit or the AI/YAML tab forms.
    if (isEdit || mode !== 'manual') return
    const snapshot = $state.snapshot(form)
    if (hasDraftContent(snapshot)) saveDraft(DRAFT_KEY, snapshot)
  })

  function discardDraft() {
    clearDraft(DRAFT_KEY)
    draftRestored = false
    isEdit = false
    editDate = null
    form = emptyForm()
    mode = 'manual'
  }

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
    // Restore an unsaved manual draft before editData (edit wins).
    const draft = loadDraft(DRAFT_KEY)
    if (hasDraftContent(draft)) {
      form = { ...emptyForm(), ...draft }
      draftRestored = true
      mode = 'manual'
    }
    // Check if we're editing an existing nutrition log
    if (store.editData && store.editData.type === 'nutrition') {
      clearDraft(DRAFT_KEY)
      draftRestored = false
      const row = store.editData.data
      // Mark the edit (and pin its original date) before touching the form so
      // the draft effect can never snapshot an edit-in-progress.
      isEdit = true
      editDate = row.date
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

  // True when the given date already has a nutrition log. POST merges additively,
  // so callers must confirm before writing over an existing day. A 404 is the only
  // "no log yet" signal; any other failure propagates rather than silently merging.
  async function logExistsFor(date) {
    try {
      await api.getNutrition(date)
      return true
    } catch (e) {
      if (e.message === 'nutrition log not found') return false
      throw e
    }
  }

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
      const editing = editDate !== null
      if (editing) {
        // Editing replaces the whole day's log, so PUT to the ORIGINAL row's
        // date — a date field changed mid-edit must not retarget the write.
        await api.putNutrition(editDate, payload)
        // The edit originated in History, so return there as its natural
        // completion. Leaving the form open on the edited date would silently
        // re-arm Save as a new-entry POST, which additively merges into the
        // day we just replaced.
        clearDraft(DRAFT_KEY)
        draftRestored = false
        isEdit = false
        editDate = null
        setCurrentPage('history')
        return
      } else {
        if (await logExistsFor(form.date)) {
          if (!confirm(`A log already exists for ${form.date} — saving will ADD to it. Continue?`)) {
            return
          }
        }
        await api.postNutrition(payload)
        success = 'Saved'
        form = emptyForm()
      }
      clearDraft(DRAFT_KEY)
      draftRestored = false
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  // Downscale a photo to <=1024px on its long edge and re-encode as JPEG.
  // Resolves { imageBase64, mimeType, previewUrl }; base64 is raw (no prefix).
  function downscaleImage(file, maxDim = 1024, quality = 0.85) {
    return new Promise((resolve, reject) => {
      const objectUrl = URL.createObjectURL(file)
      const img = new Image()
      img.onload = () => {
        try {
          const scale = Math.min(1, maxDim / Math.max(img.naturalWidth, img.naturalHeight))
          const width = Math.max(1, Math.round(img.naturalWidth * scale))
          const height = Math.max(1, Math.round(img.naturalHeight * scale))
          const canvas = document.createElement('canvas')
          canvas.width = width
          canvas.height = height
          const ctx = canvas.getContext('2d')
          if (!ctx) throw new Error('Could not process image')
          ctx.drawImage(img, 0, 0, width, height)
          const dataUrl = canvas.toDataURL('image/jpeg', quality)
          const base64 = dataUrl.replace(/^data:image\/(?:jpeg|jpg);base64,/, '')
          resolve({ imageBase64: base64, mimeType: 'image/jpeg', previewUrl: dataUrl })
        } catch (e) {
          reject(e)
        } finally {
          URL.revokeObjectURL(objectUrl)
        }
      }
      img.onerror = () => {
        URL.revokeObjectURL(objectUrl)
        reject(new Error('Could not read that image'))
      }
      img.src = objectUrl
    })
  }

  async function onPhotoSelected(event) {
    const file = event.target.files?.[0]
    // Reset so choosing the same file again still fires a change event.
    event.target.value = ''
    if (!file) return
    error = ''
    success = ''
    aiImageLoading = true
    try {
      aiImage = await downscaleImage(file)
    } catch (e) {
      error = e.message
    } finally {
      aiImageLoading = false
    }
  }

  function removeImage() {
    aiImage = null
  }

  async function parseAI() {
    if (!canParse) {
      error = 'describe your meal or attach a photo'
      return
    }
    error = ''
    success = ''
    aiLoading = true
    aiResult = null
    try {
      const payload = {}
      if (aiText.trim()) payload.text = aiText
      if (aiImage) {
        payload.imageBase64 = aiImage.imageBase64
        payload.mimeType = aiImage.mimeType
      }
      // Images are parsed and discarded server-side; Web Share Target integration is planned v2.
      aiResult = await api.parseMeal(payload)
      // Clear only after a successful parse.
      aiImage = null
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

  {#if draftRestored && mode === 'manual'}
    <div class="mb-3 text-xs text-gray-400">
      Draft restored — <button class="text-cyan-400 hover:text-cyan-300 underline" onclick={discardDraft}>Discard?</button>
    </div>
  {/if}

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

      {#if aiImage}
        <div class="relative inline-block mt-3">
          <img
            src={aiImage.previewUrl}
            alt="Meal preview"
            class="w-24 h-24 object-cover rounded-lg border border-gray-700"
          />
          <button
            type="button"
            class="absolute -top-2 -right-2 w-6 h-6 rounded-full bg-gray-900 border border-gray-600 text-xs text-gray-300 hover:text-red-400"
            title="Remove photo"
            aria-label="Remove photo"
            onclick={removeImage}
          >
            ✕
          </button>
        </div>
      {/if}

      <div class="flex flex-wrap items-center gap-2 mt-2">
        <input
          id="lm-photo"
          class="hidden"
          type="file"
          accept="image/*"
          capture="environment"
          onchange={onPhotoSelected}
        />
        <label
          for="lm-photo"
          class="bg-gray-700 px-3 py-2 rounded-lg text-sm cursor-pointer hover:bg-gray-600"
        >
          📷 {aiImage ? 'Replace photo' : 'Attach photo'}
        </label>
        <button
          class="btn-primary"
          onclick={parseAI}
          disabled={aiLoading || aiImageLoading || !canParse}
        >
          {aiLoading ? 'Parsing…' : 'Parse'}
        </button>
        {#if aiImageLoading}
          <span class="text-xs text-gray-400">Preparing photo…</span>
        {/if}
      </div>

      {#if !canParse && !aiImageLoading}
        <p class="text-xs text-gray-400 mt-2">describe your meal or attach a photo</p>
      {/if}

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
