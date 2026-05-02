<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispLength, lengthUnit, inputLength } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'

  let form = $state({ date: today(), neck_cm: '', chest_cm: '', waist_cm: '', hips_cm: '', thigh_cm: '', bicep_cm: '', notes: '' })
  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let history = $state([])

  async function load() {
    try {
      history = await api.listMeasurements(daysAgo(30), today())
    } catch {
      history = []
    }
  }

  onMount(load)

  async function submit() {
    error = ''
    success = ''
    loading = true
    try {
      await api.postMeasurement({
        date:     form.date,
        neckCm:  inputLength(form.neck_cm, store.units),
        chestCm: inputLength(form.chest_cm, store.units),
        waistCm: inputLength(form.waist_cm, store.units),
        hipsCm:  inputLength(form.hips_cm, store.units),
        thighCm: inputLength(form.thigh_cm, store.units),
        bicepCm: inputLength(form.bicep_cm, store.units),
        notes:    form.notes || '',
      })
      success = 'Measurement saved'
      form = { date: today(), neck_cm: '', chest_cm: '', waist_cm: '', hips_cm: '', thigh_cm: '', bicep_cm: '', notes: '' }
      await load()
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if success}<Alert type="success" message={success} />{/if}

  <div class="max-w-screen-xl mx-auto space-y-4">
  <div class="bg-gray-800 p-3 rounded border border-gray-700">
    <h3 class="text-emerald-400 font-bold mb-2">Log Measurement</h3>
    <div class="space-y-2">
      <input class="input" bind:value={form.date} />
      <input class="input" placeholder="Neck ({lengthUnit(store.units)})" bind:value={form.neck_cm} />
      <input class="input" placeholder="Chest ({lengthUnit(store.units)})" bind:value={form.chest_cm} />
      <input class="input" placeholder="Waist ({lengthUnit(store.units)})" bind:value={form.waist_cm} />
      <input class="input" placeholder="Hips ({lengthUnit(store.units)})" bind:value={form.hips_cm} />
      <input class="input" placeholder="Thigh ({lengthUnit(store.units)})" bind:value={form.thigh_cm} />
      <input class="input" placeholder="Bicep ({lengthUnit(store.units)})" bind:value={form.bicep_cm} />
      <textarea class="input" placeholder="Notes" bind:value={form.notes}></textarea>
        <div>
        <button class="btn-primary" onclick={submit}>Save</button>
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
            <tr class="border-t border-gray-800"><td>{h.date}</td><td>{dispLength(h.neckCm, store.units)}</td><td>{dispLength(h.chestCm, store.units)}</td><td>{dispLength(h.waistCm, store.units)}</td><td>{dispLength(h.hipsCm, store.units)}</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>
