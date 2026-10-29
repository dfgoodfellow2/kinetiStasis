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

  // Group-sharing state for owned templates. Shared (non-owned) templates
  // arrive with a sharedBy name and are badge-only; owned ones expose
  // share/unshare controls backed by these groups.
  let myGroups = $state([])
  let groupsLoaded = $state(false)
  let sharePickerFor = $state(null)
  let sharingId = $state(null)
  let showGroupsHint = $derived(groupsLoaded && myGroups.length === 0 && templates.length > 0)

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

  async function loadGroups() {
    try {
      const list = await api.listGroups()
      myGroups = Array.isArray(list) ? list : []
    } catch {
      // Template sharing is optional; a failed group read falls back to the
      // private-template experience instead of blocking the log form.
      myGroups = []
    } finally {
      groupsLoaded = true
    }
  }

  function templateGroupName(t) {
    return myGroups.find((g) => g.id === t.groupId)?.name || 'your group'
  }

  // One group → share straight away. Two or more → open the inline picker so
  // the destination is explicit without a blocking window.prompt.
  function startShare(t) {
    if (myGroups.length === 1) {
      shareTemplateToGroup(t, myGroups[0])
      return
    }
    sharePickerFor = sharePickerFor === t.id ? null : t.id
  }

  async function shareTemplateToGroup(t, g) {
    error = ''
    success = ''
    sharingId = t.id
    try {
      await api.shareTemplate(t.id, g.id)
      await loadTemplates()
      success = `Shared "${t.name}" with ${g.name}`
    } catch (e) {
      error = e.message
    } finally {
      sharingId = null
      sharePickerFor = null
    }
  }

  async function unshareOwnedTemplate(t) {
    error = ''
    success = ''
    sharingId = t.id
    try {
      await api.unshareTemplate(t.id)
      await loadTemplates()
      success = `"${t.name}" is private again`
    } catch (e) {
      error = e.message
    } finally {
      sharingId = null
    }
  }

  // Append a saved meal's description to the form notes. Descriptions stack
  // with newlines so a template/quick-add never clobbers what the user typed,
  // and an already-present description is not duplicated.
  function appendNote(existing, note) {
    const next = (note ?? '').trim()
    const current = existing ?? ''
    if (!next) return current
    if (current.includes(next)) return current
    return current ? current + '\n' + next : next
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
      mealNotes: appendNote(form.mealNotes, m.notes),
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
        notes: form.mealNotes || '',
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
    loadGroups()
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
  //
  // iPhone cameras default to HEIC, which Safari — and especially standalone
  // PWA mode — may refuse to decode, with behaviour that varies by iOS version.
  // The photo input solves this up front: its restrictive `accept` list
  // (image/jpeg,image/png,image/webp) means iOS's Photos/camera pipeline
  // transcodes HEIC to JPEG at pick time, natively, exactly as native apps get
  // HEIC support from the OS codecs. We therefore receive an already-compatible
  // JPEG and never see raw HEIC. Because of that OS-side transcode, the user no
  // longer needs the Settings → Camera → Formats → 'Most Compatible' setting.
  //
  // This decode chain stays as the safety net for anything that still slips
  // through (webp/png variants, odd iOS builds, edge-case files). Each route
  // covers a different failure mode:
  //   1. createImageBitmap (honours EXIF orientation; newest, most reliable)
  //   2. blob object URL + <img> (the historical path; works on older Safari)
  //   3. FileReader data URL + <img> (some iOS builds decode data URLs when
  //      blob URLs fail)
  // Only if all three fail do we surface an actionable error to the user.
  function downscaleImage(file, maxDim = 1024, quality = 0.85) {
    function encode(source, sourceWidth, sourceHeight) {
      const scale = Math.min(1, maxDim / Math.max(sourceWidth, sourceHeight))
      const width = Math.max(1, Math.round(sourceWidth * scale))
      const height = Math.max(1, Math.round(sourceHeight * scale))
      const canvas = document.createElement('canvas')
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      if (!ctx) throw new Error('Could not process image')
      ctx.drawImage(source, 0, 0, width, height)
      const dataUrl = canvas.toDataURL('image/jpeg', quality)
      const base64 = dataUrl.replace(/^data:image\/(?:jpeg|jpg);base64,/, '')
      return { imageBase64: base64, mimeType: 'image/jpeg', previewUrl: dataUrl }
    }

    function loadImageElement(src) {
      return new Promise((resolve, reject) => {
        const img = new Image()
        img.onload = () => resolve(img)
        img.onerror = () => reject(new Error('image decode failed'))
        img.src = src
      })
    }

    // Attempt 1: createImageBitmap. Feature-detect first; older Safari also
    // rejects the options argument, so retry with defaults.
    async function viaBitmap() {
      if (typeof createImageBitmap !== 'function') {
        throw new Error('createImageBitmap unavailable')
      }
      let bitmap
      try {
        bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
      } catch {
        bitmap = await createImageBitmap(file)
      }
      try {
        return encode(bitmap, bitmap.width, bitmap.height)
      } finally {
        if (typeof bitmap.close === 'function') bitmap.close()
      }
    }

    // Attempt 2: blob object URL (must be revoked on every path).
    async function viaBlobUrl() {
      const objectUrl = URL.createObjectURL(file)
      try {
        const img = await loadImageElement(objectUrl)
        return encode(img, img.naturalWidth, img.naturalHeight)
      } finally {
        URL.revokeObjectURL(objectUrl)
      }
    }

    // Attempt 3: FileReader data URL.
    async function viaDataUrl() {
      const dataUrl = await new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => {
          if (typeof reader.result === 'string') resolve(reader.result)
          else reject(new Error('unexpected FileReader result'))
        }
        reader.onerror = () => reject(reader.error || new Error('FileReader failed'))
        reader.readAsDataURL(file)
      })
      const img = await loadImageElement(dataUrl)
      return encode(img, img.naturalWidth, img.naturalHeight)
    }

    return (async () => {
      const attempts = [viaBitmap, viaBlobUrl, viaDataUrl]
      for (const attempt of attempts) {
        try {
          return await attempt()
        } catch {
          // Fall through to the next decode strategy.
        }
      }
      throw new Error("Couldn't process that photo on this device. iPhone tip: Settings → Camera → Formats → 'Most Compatible', or pick the photo from Photos instead of the camera.")
    })()
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
              <div class="flex items-center gap-1 bg-gray-700/40 hover:bg-gray-700">
                <button
                  class="flex-1 min-w-0 flex items-center justify-between px-3 py-2 text-left"
                  onclick={() => toggleTemplate(t.id)}
                >
                  <span class="text-sm font-medium truncate">{t.name}</span>
                  <span class="text-xs text-gray-400 shrink-0">
                    {t.meals?.length ?? 0} meals {openTemplates.includes(t.id) ? '▾' : '▸'}
                  </span>
                </button>
                <div class="flex items-center gap-1 pr-2 shrink-0">
                  {#if t.sharedBy}
                    <span class="inline-flex items-center rounded-full border border-gray-600 bg-gray-700 px-2 py-0.5 text-[11px] font-medium text-gray-300">
                      shared by {t.sharedBy}
                    </span>
                  {:else if t.groupId}
                    <span class="hidden sm:inline text-[11px] text-emerald-300" title="Shared with {templateGroupName(t)}">shared</span>
                    <button
                      class="text-xs text-gray-400 hover:text-red-400 px-1.5 py-1"
                      title="Stop sharing {t.name}"
                      onclick={() => unshareOwnedTemplate(t)}
                      disabled={sharingId === t.id}
                    >
                      Unshare
                    </button>
                  {:else if groupsLoaded && myGroups.length > 0}
                    <button
                      class="text-xs text-cyan-400 hover:text-cyan-300 px-1.5 py-1"
                      title="Share {t.name} with a group"
                      onclick={() => startShare(t)}
                      disabled={sharingId === t.id}
                    >
                      Share
                    </button>
                  {/if}
                </div>
              </div>
              {#if sharePickerFor === t.id && !t.sharedBy && !t.groupId}
                <div class="flex flex-wrap items-center gap-2 px-3 py-2 border-t border-gray-700 bg-gray-900/40 text-xs">
                  <span class="text-gray-400">Share with</span>
                  {#each myGroups as g (g.id)}
                    <button
                      class="rounded-full border border-gray-600 bg-gray-700 px-2.5 py-1 hover:border-emerald-600 hover:text-emerald-300"
                      onclick={() => shareTemplateToGroup(t, g)}
                      disabled={sharingId === t.id}
                    >
                      {g.name}
                    </button>
                  {/each}
                  <button class="ml-auto text-gray-500 hover:text-gray-300" onclick={() => (sharePickerFor = null)}>
                    Cancel
                  </button>
                </div>
              {/if}
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
        {#if showGroupsHint}
          <p class="text-xs text-gray-500 mt-2">Create a group to share meal templates</p>
        {/if}
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
          accept="image/jpeg,image/png,image/webp"
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
