<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispLength, lengthUnit, inputLength } from '../lib/utils.js'
  import { saveDraft, loadDraft, clearDraft } from '../lib/draft.js'
  import Alert from '../components/Alert.svelte'

  const DRAFT_KEY = 'draft:measurements:form'

  function emptyForm() {
    return { date: today(), neckCm: '', chestCm: '', waistCm: '', hipsCm: '', thighCm: '', bicepCm: '', shouldersCm: '', calvesCm: '', notes: '' }
  }

  let form = $state(emptyForm())
  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let history = $state([])
  let isEdit = $state(false)
  let editDate = $state('')
  let profile = $state(null)
  let suggestion = $state(null)
  let suggestionSaving = $state(false)
  let draftRestored = $state(false)

  // Draft only the text/number fields of this form.
  function hasDraftContent(f) {
    if (!f || typeof f !== 'object') return false
    return ['neckCm', 'chestCm', 'waistCm', 'hipsCm', 'thighCm', 'bicepCm', 'shouldersCm', 'calvesCm', 'notes']
      .some((k) => String(f[k] ?? '').trim() !== '')
  }

  $effect(() => {
    const snapshot = $state.snapshot(form)
    if (hasDraftContent(snapshot)) saveDraft(DRAFT_KEY, snapshot)
  })

  async function load() {
    try {
      history = await api.listMeasurements(daysAgo(30), today())
    } catch {
      history = []
    }
  }

  async function ensureProfile() {
    if (profile) return profile
    try {
      profile = await api.getProfile()
    } catch {
      profile = null
    }
    return profile
  }

  function discardDraft() {
    clearDraft(DRAFT_KEY)
    draftRestored = false
    isEdit = false
    editDate = ''
    form = emptyForm()
  }

  onMount(() => {
    const draft = loadDraft(DRAFT_KEY)
    if (hasDraftContent(draft)) {
      form = { ...emptyForm(), ...draft }
      draftRestored = true
    }
    load()
    ensureProfile()
  })

  // Populate an input with the stored cm value converted to the display unit.
  // Blank/zero values stay blank so placeholders remain visible.
  function editValue(cm) {
    return cm ? String(dispLength(cm, store.units)) : ''
  }

  function startEdit(h) {
    error = ''
    success = ''
    suggestion = null
    clearDraft(DRAFT_KEY)
    draftRestored = false
    isEdit = true
    editDate = h.date
    form = {
      date:        h.date,
      neckCm:      editValue(h.neckCm),
      chestCm:     editValue(h.chestCm),
      waistCm:     editValue(h.waistCm),
      hipsCm:      editValue(h.hipsCm),
      thighCm:     editValue(h.thighCm),
      bicepCm:     editValue(h.bicepCm),
      shouldersCm: editValue(h.shouldersCm),
      calvesCm:    editValue(h.calvesCm),
      notes:       h.notes ?? '',
    }
  }

  function cancelEdit() {
    isEdit = false
    editDate = ''
    error = ''
    success = ''
    suggestion = null
    form = emptyForm()
  }

  // Offer to write the Navy result to biometrics for the entry we just saved.
  // The server calculation always uses the most recent measurement row, so we
  // only offer when the saved date is the latest one — otherwise the percentage
  // could belong to a different entry.
  async function offerBodyFat(date) {
    suggestion = null
    const p = await ensureProfile()
    if (!p || !p.sex || !(p.heightCm > 0)) return

    const saved = history.find((h) => h.date === date)
    if (!saved) return

    const latestDate = history.reduce((max, h) => (h.date > max ? h.date : max), '')
    if (latestDate !== date) return

    const male = String(p.sex).toLowerCase() === 'male'
    if (!(saved.neckCm > 0) || !(saved.waistCm > 0)) return
    if (!male && !(saved.hipsCm > 0)) return

    try {
      const bf = await api.getBodyFat('navy')
      if (bf && bf.bfPct > 0) suggestion = { date, bfPct: bf.bfPct }
    } catch {
      // Calculation is optional; a failure just means no suggestion chip.
    }
  }

  async function applySuggestion() {
    const s = suggestion
    if (!s) return
    suggestion = null
    suggestionSaving = true
    try {
      // POST is a preserve-on-zero upsert: sending only the date and body fat
      // leaves any other biometric fields for that day untouched.
      await api.postBiometric({ date: s.date, bodyFatPct: s.bfPct })
      success = `Body fat ${s.bfPct.toFixed(1)}% added for ${s.date}`
    } catch (e) {
      error = e.message
    } finally {
      suggestionSaving = false
    }
  }

  async function submit() {
    error = ''
    success = ''
    suggestion = null
    loading = true
    try {
      const payload = {
        date:        form.date,
        neckCm:      inputLength(form.neckCm, store.units),
        chestCm:     inputLength(form.chestCm, store.units),
        waistCm:     inputLength(form.waistCm, store.units),
        hipsCm:      inputLength(form.hipsCm, store.units),
        thighCm:     inputLength(form.thighCm, store.units),
        bicepCm:     inputLength(form.bicepCm, store.units),
        shouldersCm: inputLength(form.shouldersCm, store.units),
        calvesCm:    inputLength(form.calvesCm, store.units),
        notes:       form.notes || '',
      }
      const savedDate = payload.date
      if (isEdit) {
        // PUT is a true replace: blank fields intentionally clear the stored value.
        await api.putMeasurement(editDate, payload)
        success = 'Measurement updated'
        isEdit = false
        editDate = ''
      } else {
        await api.postMeasurement(payload)
        success = 'Measurement saved'
      }
      // Return to new-entry mode after a successful save; on failure the
      // edit state is left untouched so the user can retry.
      clearDraft(DRAFT_KEY)
      draftRestored = false
      form = emptyForm()
      await load()
      await offerBodyFat(savedDate)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  async function remove(date) {
    if (!confirm('Delete measurement for ' + date + '?')) return
    try {
      await api.deleteMeasurement(date)
      if (isEdit && editDate === date) cancelEdit()
      await load()
    } catch (e) {
      alert(`Delete failed: ${e.message || e}`)
    }
  }
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if success}<Alert type="success" message={success} />{/if}

  <div class="max-w-screen-xl mx-auto space-y-4">
  {#if draftRestored}
    <div class="text-xs text-gray-400">
      Draft restored — <button class="text-cyan-400 hover:text-cyan-300 underline" onclick={discardDraft}>Discard?</button>
    </div>
  {/if}
  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">{isEdit ? 'Edit Measurement' : 'Log Measurement'}</h3>
    <div class="space-y-2">
      <input class="input" bind:value={form.date} disabled={isEdit} />
      <input class="input" placeholder="Neck ({lengthUnit(store.units)})" bind:value={form.neckCm} />
      <input class="input" placeholder="Chest ({lengthUnit(store.units)})" bind:value={form.chestCm} />
      <input class="input" placeholder="Waist ({lengthUnit(store.units)})" bind:value={form.waistCm} />
      <input class="input" placeholder="Hips ({lengthUnit(store.units)})" bind:value={form.hipsCm} />
      <input class="input" placeholder="Thigh ({lengthUnit(store.units)})" bind:value={form.thighCm} />
      <input class="input" placeholder="Bicep ({lengthUnit(store.units)})" bind:value={form.bicepCm} />
      <input class="input" placeholder="Shoulders ({lengthUnit(store.units)})" bind:value={form.shouldersCm} />
      <input class="input" placeholder="Calves ({lengthUnit(store.units)})" bind:value={form.calvesCm} />
      <textarea class="input" placeholder="Notes" bind:value={form.notes}></textarea>
        <div>
        <button class="btn-primary" onclick={submit} disabled={loading}>{loading ? 'Saving…' : (isEdit ? 'Update' : 'Save')}</button>
        {#if isEdit}
          <button class="bg-gray-700 px-3 py-2 rounded-lg ml-2" onclick={cancelEdit}>Cancel</button>
        {/if}
        {#if suggestion}
          <div class="mt-2 inline-flex items-center gap-1 rounded-full border border-emerald-700 bg-emerald-900/40 pl-3 pr-1 py-1 text-sm text-emerald-300">
            <button class="hover:text-emerald-200" onclick={applySuggestion} disabled={suggestionSaving}>
              Auto-fill body fat ({suggestion.bfPct.toFixed(1)}%)?
            </button>
            <button class="px-2 text-gray-400 hover:text-gray-200" onclick={() => (suggestion = null)} title="Dismiss" aria-label="Dismiss">✕</button>
          </div>
        {/if}
      </div>
    </div>
  </div>

  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">History</h3>
    <div class="overflow-x-auto">
      <table class="min-w-full text-sm">
        <thead><tr class="text-left text-gray-300"><th>Date</th><th>Neck ({lengthUnit(store.units)})</th><th>Chest ({lengthUnit(store.units)})</th><th>Waist ({lengthUnit(store.units)})</th><th>Hips ({lengthUnit(store.units)})</th></tr></thead>
        <tbody>
          {#each history as h}
            <tr class="border-t border-gray-800"><td>{h.date}</td><td>{dispLength(h.neckCm, store.units)}</td><td>{dispLength(h.chestCm, store.units)}</td><td>{dispLength(h.waistCm, store.units)}</td><td>{dispLength(h.hipsCm, store.units)}</td><td class="py-2"><button class="text-gray-400 hover:text-emerald-400 mr-3" onclick={() => startEdit(h)} title="Edit">✏️</button><button class="text-gray-400 hover:text-red-400" onclick={() => remove(h.date)} title="Delete">🗑️</button></td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>
