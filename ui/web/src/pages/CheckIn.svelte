<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispWeight, weightUnit, inputWeight, dispLength, lengthUnit, inputLength, inputLoad, dispLoad, loadUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  let tab = $state('daily')

  // --- Daily tab ---
  let dailyForm = $state({ date: today(), weight_kg: '', grip_kg: '', bolt_score: '', sleep_hours: '', sleep_quality: '', subjective: '', notes: '' })
  let dailyLoading = $state(false)
  let dailyError = $state('')
  let dailySuccess = $state('')

  async function submitDaily() {
    dailyError = ''
    dailySuccess = ''
    dailyLoading = true
    try {
      await api.postBiometric({
        date: dailyForm.date,
        weight_kg:       inputWeight(dailyForm.weight_kg, store.units),
        // waist removed
        grip_kg:         inputLoad(dailyForm.grip_kg, store.units),
        bolt_score:      Number(dailyForm.bolt_score)    || 0,
        sleep_hours:     Number(dailyForm.sleep_hours)   || 0,
        sleep_quality:   Number(dailyForm.sleep_quality) || 0,
        subjective_feel: Number(dailyForm.subjective)    || 0,
        notes:           dailyForm.notes || '',
      })
      dailySuccess = 'Check-in saved'
      dailyForm = { date: today(), weight_kg: '', grip_kg: '', bolt_score: '', sleep_hours: '', sleep_quality: '', subjective: '', notes: '' }
    } catch (e) {
      dailyError = e.message
    } finally {
      dailyLoading = false
    }
  }

  // --- Measurements tab ---
  let measForm = $state({ date: today(), neck_cm: '', chest_cm: '', waist_cm: '', hips_cm: '', thigh_cm: '', bicep_cm: '', shoulders_cm: '', calves_cm: '', notes: '' })
  let measLoading = $state(false)
  let measError = $state('')
  let measSuccess = $state('')
  let measHistory = $state([])

  async function loadMeasurements() {
    try {
      measHistory = await api.listMeasurements(daysAgo(30), today())
    } catch {
      measHistory = []
    }
  }

  async function submitMeasurement() {
    measError = ''
    measSuccess = ''
    measLoading = true
    try {
      await api.postMeasurement({
        date:         measForm.date,
        neck_cm:      inputLength(measForm.neck_cm, store.units),
        chest_cm:     inputLength(measForm.chest_cm, store.units),
        waist_cm:     inputLength(measForm.waist_cm, store.units),
        hips_cm:      inputLength(measForm.hips_cm, store.units),
        thigh_cm:     inputLength(measForm.thigh_cm, store.units),
        bicep_cm:     inputLength(measForm.bicep_cm, store.units),
        shoulders_cm: inputLength(measForm.shoulders_cm, store.units),
        calves_cm:    inputLength(measForm.calves_cm, store.units),
        notes:        measForm.notes || '',
      })
      measSuccess = 'Measurement saved'
      measForm = { date: today(), neck_cm: '', chest_cm: '', waist_cm: '', hips_cm: '', thigh_cm: '', bicep_cm: '', shoulders_cm: '', calves_cm: '', notes: '' }
      await loadMeasurements()
    } catch (e) {
      measError = e.message
    } finally {
      measLoading = false
    }
  }

  // --- Body Fat tab ---
  let bfMethod = $state('navy')
  let bfData = $state(null)
  let bfLoading = $state(true)

  async function loadBodyFat() {
    bfLoading = true
    try {
      bfData = await api.getBodyFat(bfMethod)
    } catch {
      bfData = null
    } finally {
      bfLoading = false
    }
  }

  onMount(() => {
    // Check if we're editing existing data
    if (store.editData) {
        if (store.editData.type === 'biometric') {
        const row = store.editData.data
        dailyForm = { 
          date: row.date, 
          weight_kg: String(row.weight_kg ?? ''),
          grip_kg: String(row.grip_kg ?? ''),
          bolt_score: String(row.bolt_score ?? ''),
          sleep_hours: String(row.sleep_hours ?? ''),
          sleep_quality: String(row.sleep_quality ?? ''),
          subjective: String(row.subjective_feel ?? ''),
          notes: row.notes ?? ''
        }
        tab = 'daily'
      } else if (store.editData.type === 'measurement') {
        const row = store.editData.data
        measForm = {
          date: row.date,
          neck_cm: String(row.neck_cm ?? ''),
          chest_cm: String(row.chest_cm ?? ''),
          waist_cm: String(row.waist_cm ?? ''),
          hips_cm: String(row.hips_cm ?? ''),
          thigh_cm: String(row.thigh_cm ?? ''),
          bicep_cm: String(row.bicep_cm ?? ''),
          shoulders_cm: String(row.shoulders_cm ?? ''),
          calves_cm: String(row.calves_cm ?? ''),
          notes: row.notes ?? ''
        }
        tab = 'measurements'
      }
      clearEditData()
    }
    loadMeasurements()
    loadBodyFat()
  })
</script>

<div class="max-w-2xl mx-auto">
  <!-- Tab bar — same style as LogMeal -->
  <div class="flex space-x-2 mb-4">
    <button class={tab === 'daily' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'daily'}>Daily</button>
    <button class={tab === 'measurements' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'measurements'}>Measurements</button>
    <button class={tab === 'bodyfat' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'bodyfat'}>Body Fat %</button>
  </div>

  <!-- Daily tab -->
  {#if tab === 'daily'}
    {#if dailyError}<Alert type="error" message={dailyError} />{/if}
    {#if dailySuccess}<Alert type="success" message={dailySuccess} />{/if}
    <div class="bg-gray-800 p-4 rounded-lg border border-gray-700 space-y-4">
      <h3 class="text-emerald-400 font-bold text-lg">Daily Check-in</h3>
      <div class="grid grid-cols-2 gap-3">
        <div class="col-span-2">
          <label class="text-xs text-gray-400" for="ci-date">Date</label>
          <input class="input" id="ci-date" type="date" bind:value={dailyForm.date} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-weight">Weight ({weightUnit(store.units)})</label>
          <input class="input" id="ci-weight" type="number" step="0.1" bind:value={dailyForm.weight_kg} />
        </div>

        

        <div>
          <label class="text-xs text-gray-400" for="ci-grip">Grip Strength ({loadUnit(store.units)})</label>
          <input class="input" id="ci-grip" type="number" step="0.1" bind:value={dailyForm.grip_kg} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-bolt">BOLT Score (s)</label>
          <input class="input" id="ci-bolt" type="number" bind:value={dailyForm.bolt_score} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-sleep-hrs">Sleep (hrs)</label>
          <input class="input" id="ci-sleep-hrs" type="number" step="0.1" bind:value={dailyForm.sleep_hours} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-sleep-qual">Sleep Quality (1–{store.sleepQualityMax})</label>
          <input class="input" id="ci-sleep-qual" type="number" min="1" max={store.sleepQualityMax} bind:value={dailyForm.sleep_quality} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-subjective">Subjective Feel (1–10)</label>
          <input class="input" id="ci-subjective" type="number" min="1" max="10" bind:value={dailyForm.subjective} />
        </div>

        <div class="col-span-2">
          <label class="text-xs text-gray-400" for="ci-notes">Notes</label>
          <textarea class="input" id="ci-notes" rows="2" bind:value={dailyForm.notes}></textarea>
        </div>

        <div class="col-span-2">
          <button class="btn-primary" onclick={submitDaily} disabled={dailyLoading}>{dailyLoading ? 'Saving…' : 'Save Check-in'}</button>
        </div>
      </div>
    </div>

  <!-- Measurements tab -->
  {:else if tab === 'measurements'}
    {#if measError}<Alert type="error" message={measError} />{/if}
    {#if measSuccess}<Alert type="success" message={measSuccess} />{/if}
    <div class="space-y-4">
      <Card title="Log Measurement">
        <div class="grid grid-cols-2 gap-3">
          <div class="col-span-2">
            <label class="text-xs text-gray-400" for="meas-date">Date</label>
            <input class="input" id="meas-date" type="date" bind:value={measForm.date} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-neck">Neck ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-neck" type="number" step="0.1" bind:value={measForm.neck_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-chest">Chest ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-chest" type="number" step="0.1" bind:value={measForm.chest_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-waist">Waist ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-waist" type="number" step="0.1" bind:value={measForm.waist_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-hips">Hips ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-hips" type="number" step="0.1" bind:value={measForm.hips_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-thigh">Thigh ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-thigh" type="number" step="0.1" bind:value={measForm.thigh_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-bicep">Bicep ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-bicep" type="number" step="0.1" bind:value={measForm.bicep_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-shoulders">Shoulders ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-shoulders" type="number" step="0.1" bind:value={measForm.shoulders_cm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-calves">Calves ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-calves" type="number" step="0.1" bind:value={measForm.calves_cm} />
          </div>

          <div class="col-span-2">
            <label class="text-xs text-gray-400" for="meas-notes">Notes</label>
            <textarea class="input" id="meas-notes" rows="2" bind:value={measForm.notes}></textarea>
          </div>

          <div class="col-span-2">
            <button class="btn-primary" onclick={submitMeasurement} disabled={measLoading}>{measLoading ? 'Saving…' : 'Save'}</button>
          </div>
        </div>
      </Card>

      <Card title="History (30d)">
        <div class="overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead><tr class="text-left text-gray-300"><th>Date</th><th>Neck ({lengthUnit(store.units)})</th><th>Chest ({lengthUnit(store.units)})</th><th>Waist ({lengthUnit(store.units)})</th><th>Hips ({lengthUnit(store.units)})</th></tr></thead>
            <tbody>
              {#each measHistory as h}
                <tr class="border-t border-gray-800"><td>{h.date}</td><td>{dispLength(h.neck_cm, store.units)}</td><td>{dispLength(h.chest_cm, store.units)}</td><td>{dispLength(h.waist_cm, store.units)}</td><td>{dispLength(h.hips_cm, store.units)}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      </Card>
    </div>

  <!-- Body Fat % tab -->
  {:else if tab === 'bodyfat'}
    {#if bfLoading}
      <Spinner />
    {:else}
      <Card title="Body Fat % — Navy Method">
        {#if bfData && bfData.bf_pct > 0}
          <div class="space-y-3">
            <div class="text-4xl font-bold text-emerald-400">{bfData.bf_pct.toFixed(1)}<span class="text-lg text-gray-400 ml-1">%</span></div>
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400 mb-1">Lean Mass</div>
                <div class="text-lg font-semibold text-gray-100">{dispWeight(bfData.lean_mass_kg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
              </div>
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400 mb-1">Fat Mass</div>
                <div class="text-lg font-semibold text-gray-100">{dispWeight(bfData.fat_mass_kg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
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
    {/if}
  {/if}
</div>
