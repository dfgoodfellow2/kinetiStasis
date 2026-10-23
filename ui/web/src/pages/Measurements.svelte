<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispLength, lengthUnit, inputLength } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'

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

  async function load() {
    try {
      history = await api.listMeasurements(daysAgo(30), today())
    } catch {
      history = []
    }
  }

  onMount(load)

  // Populate an input with the stored cm value converted to the display unit.
  // Blank/zero values stay blank so placeholders remain visible.
  function editValue(cm) {
    return cm ? String(dispLength(cm, store.units)) : ''
  }

  function startEdit(h) {
    error = ''
    success = ''
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
    form = emptyForm()
  }

  async function submit() {
    error = ''
    success = ''
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
      form = emptyForm()
      await load()
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
