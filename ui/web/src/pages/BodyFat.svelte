<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispWeight, weightUnit, dispLength, lengthUnit, inputLength } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  let data = $state(null)
  let loading = $state(true)
  let profile = $state(null)

  // Calculator inputs (Navy method). Prefilled from the latest measurement row
  // that has the fields the formula needs; the user still taps Calculate.
  let calcForm = $state({ date: today(), neckCm: '', waistCm: '', hipsCm: '' })
  let calcPrefilled = $state(false)
  let calcLoading = $state(false)
  let calcError = $state('')
  let calcSuccess = $state('')

  let isFemale = $derived(String(profile?.sex ?? '').toLowerCase() !== 'male')

  async function load() {
    loading = true
    try {
      data = await api.getBodyFat('navy')
    } catch {
      data = null
    } finally {
      loading = false
    }
  }

  async function loadProfileAndPrefill() {
    try {
      profile = await api.getProfile()
    } catch {
      profile = null
    }

    const male = String(profile?.sex ?? '').toLowerCase() === 'male'
    // If we could not load the profile, accept any row with neck + waist and
    // let the user fill hips themselves.
    const needHips = profile?.sex ? !male : false

    try {
      const rows = (await api.listMeasurements(daysAgo(365), today())) || []
      let latest = null
      for (const m of rows) {
        if (!(m.neckCm > 0) || !(m.waistCm > 0)) continue
        if (needHips && !(m.hipsCm > 0)) continue
        if (!latest || String(m.date) > String(latest.date)) latest = m
      }
      if (latest) {
        calcForm = {
          date: latest.date,
          neckCm: String(dispLength(latest.neckCm, store.units)),
          waistCm: String(dispLength(latest.waistCm, store.units)),
          hipsCm: latest.hipsCm > 0 ? String(dispLength(latest.hipsCm, store.units)) : '',
        }
        calcPrefilled = true
      }
    } catch {
      // Prefill is a convenience; leave the form empty on failure.
    }
  }

  onMount(() => {
    load()
    loadProfileAndPrefill()
  })

  async function calculate() {
    calcError = ''
    calcSuccess = ''
    const neck = Number(calcForm.neckCm)
    const waist = Number(calcForm.waistCm)
    const hips = Number(calcForm.hipsCm)
    if (!(neck > 0) || !(waist > 0)) {
      calcError = 'Enter neck and waist measurements'
      return
    }
    if (isFemale && !(hips > 0)) {
      calcError = 'Hips are required for the female Navy formula'
      return
    }

    calcLoading = true
    try {
      // Persist the measurements first; GET /v1/calc/bodyfat does the math
      // server-side using the most recent measurement row.
      await api.postMeasurement({
        date: calcForm.date,
        neckCm: inputLength(calcForm.neckCm, store.units),
        waistCm: inputLength(calcForm.waistCm, store.units),
        hipsCm: inputLength(calcForm.hipsCm, store.units),
      })
      data = await api.getBodyFat('navy')
      calcSuccess = 'Measurements saved — body fat updated'
    } catch (e) {
      calcError = e.message
    } finally {
      calcLoading = false
    }
  }
</script>

{#if loading}
  <Spinner />
{:else}
  <div class="space-y-4">
    <Card title="Body Fat % — Navy Method">
      {#if data && data.bfPct > 0}
        <div class="space-y-3">
          <div class="text-4xl font-bold text-emerald-400">{(data.bfPct ?? 0).toFixed(1)}<span class="text-lg text-gray-400 ml-1">%</span></div>
          <div class="grid grid-cols-2 gap-3 text-sm">
            <div class="bg-gray-700 rounded-lg p-3">
              <div class="text-xs text-gray-400 mb-1">Lean Mass</div>
              <div class="text-lg font-semibold text-gray-100">{dispWeight(data.leanMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
            </div>
            <div class="bg-gray-700 rounded-lg p-3">
              <div class="text-xs text-gray-400 mb-1">Fat Mass</div>
              <div class="text-lg font-semibold text-gray-100">{dispWeight(data.fatMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
            </div>
          </div>
          <p class="text-xs text-gray-500">Calculated using the U.S. Navy circumference method from your most recent weight and body measurements.</p>
        </div>
      {:else}
        <div class="space-y-3">
          <p class="text-sm text-gray-300">Not enough data to calculate. The Navy method requires:</p>
          <ul class="text-sm space-y-1 text-gray-400">
            <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Weight logged in a Daily check-in</li>
            <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Neck, Waist &amp; Hips logged in the Measurements tab</li>
            <li class="flex items-center gap-2"><span class="text-yellow-400">⚠</span> Height and Sex set in your Profile</li>
          </ul>
          <p class="text-xs text-gray-500">Once all three are logged, your body fat % will appear here automatically.</p>
        </div>
      {/if}
    </Card>

    <Card title="Calculate from Measurements">
      {#if calcError}<Alert type="error" message={calcError} />{/if}
      {#if calcSuccess}<Alert type="success" message={calcSuccess} />{/if}
      <div class="space-y-3">
        {#if calcPrefilled}
          <p class="text-xs text-gray-400">Prefilled from your latest measurements — edit or tap Calculate.</p>
        {:else}
          <p class="text-xs text-gray-400">Enter measurements to calculate body fat % using the Navy method.</p>
        {/if}

        <div class="grid grid-cols-2 gap-3">
          <div class="col-span-2">
            <label class="text-xs text-gray-400" for="bf-date">Date</label>
            <input class="input" id="bf-date" type="date" bind:value={calcForm.date} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="bf-neck">Neck ({lengthUnit(store.units)})</label>
            <input class="input" id="bf-neck" type="number" step="0.1" bind:value={calcForm.neckCm} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="bf-waist">Waist ({lengthUnit(store.units)})</label>
            <input class="input" id="bf-waist" type="number" step="0.1" bind:value={calcForm.waistCm} />
          </div>
          <div class="col-span-2">
            <label class="text-xs text-gray-400" for="bf-hips">Hips ({lengthUnit(store.units)}){isFemale ? '' : ' — optional for male formula'}</label>
            <input class="input" id="bf-hips" type="number" step="0.1" bind:value={calcForm.hipsCm} />
          </div>
        </div>

        <button class="btn-primary" onclick={calculate} disabled={calcLoading}>{calcLoading ? 'Calculating…' : 'Calculate'}</button>
      </div>
    </Card>
  </div>
{/if}
