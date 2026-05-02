<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { today, daysAgo, dispWeight, weightUnit, inputWeight, dispLength, lengthUnit, inputLength, inputLoad, dispLoad, loadUnit } from '../lib/utils.js'
  import Card from '../components/Card.svelte'
  import Spinner from '../components/Spinner.svelte'
  import Alert from '../components/Alert.svelte'

  // --- Weekly Check-in ---
  let weeklyLoading = $state(false)
  let weeklyError = $state('')
  let weeklySuccess = $state('')
  let checkinPreview = $state(null)
  // Track if check-in was saved to disable button and change tab color
  let checkinSaved = $state(false)

  async function loadCheckinPreview() {
    weeklyError = ''
    try {
      checkinPreview = await api.getCheckinPreview()
    } catch (e) {
      checkinPreview = null
      weeklyError = e.message
    }
  }

  async function acceptCheckin() {
      if (!checkinPreview || !checkinPreview.canCheckIn) return
      weeklyError = ''
      weeklySuccess = ''
      weeklyLoading = true
      try {
      await api.postCheckin({ caloriesAfter: checkinPreview.recommendedCalories })
      weeklySuccess = 'Check-in accepted'
      checkinSaved = true
      await loadCheckinPreview()
    } catch (e) {
      weeklyError = e.message
    } finally {
      weeklyLoading = false
    }
  }


  let tab = $state('daily')

  // --- Daily tab ---
  let dailyForm = $state({ date: today(), weightKg: '', gripKg: '', boltScore: '', sleepHours: '', sleepQuality: '', subjective: '', notes: '' })
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
        weightKg:       inputWeight(dailyForm.weightKg, store.units),
        // waist removed
        gripKg:         inputLoad(dailyForm.gripKg, store.units),
        boltScore:      Number(dailyForm.boltScore)    || 0,
        sleepHours:     Number(dailyForm.sleepHours)   || 0,
        sleepQuality:   Number(dailyForm.sleepQuality) || 0,
        subjectiveFeel: Number(dailyForm.subjective)    || 0,
        notes:           dailyForm.notes || '',
      })
      dailySuccess = 'Check-in saved'
      dailyForm = { date: today(), weightKg: '', gripKg: '', boltScore: '', sleepHours: '', sleepQuality: '', subjective: '', notes: '' }
    } catch (e) {
      dailyError = e.message
    } finally {
      dailyLoading = false
    }
  }

  // --- Measurements tab ---
  let measForm = $state({ date: today(), neckCm: '', chestCm: '', waistCm: '', hipsCm: '', thighCm: '', bicepCm: '', shouldersCm: '', calvesCm: '', notes: '' })
  let measLoading = $state(false)
  let measError = $state('')
  let measSuccess = $state('')
  let measHistory = $state([])
  let bfHistory = $state([])

  async function loadMeasurements() {
    try {
      measHistory = await api.listMeasurements(daysAgo(30), today())
    } catch {
      measHistory = []
    }
  }

  async function loadBFHistory() {
    try {
      const bios = await api.listBiometrics(daysAgo(30), today())
    bfHistory = bios.filter(b => b.bodyFatPct > 0).map(b => ({ date: b.date, bodyFatPct: b.bodyFatPct }))
    } catch {
      bfHistory = []
    }
  }

  async function submitMeasurement() {
    measError = ''
    measSuccess = ''
    measLoading = true
    try {
      await api.postMeasurement({
        date:         measForm.date,
        neckCm:      inputLength(measForm.neckCm, store.units),
        chestCm:     inputLength(measForm.chestCm, store.units),
        waistCm:     inputLength(measForm.waistCm, store.units),
        hipsCm:      inputLength(measForm.hipsCm, store.units),
        thighCm:     inputLength(measForm.thighCm, store.units),
        bicepCm:     inputLength(measForm.bicepCm, store.units),
        shouldersCm: inputLength(measForm.shouldersCm, store.units),
        calvesCm:    inputLength(measForm.calvesCm, store.units),
        notes:        measForm.notes || '',
      })
      measSuccess = 'Measurement saved'
      measForm = { date: today(), neckCm: '', chestCm: '', waistCm: '', hipsCm: '', thighCm: '', bicepCm: '', shouldersCm: '', calvesCm: '', notes: '' }
      await loadMeasurements()
    } catch (e) {
      measError = e.message
    } finally {
      measLoading = false
    }
  }

  // --- Body Fat tab ---
  let bfForm = $state({ date: today(), bodyFatPct: '' })
  let bfLoading = $state(false)
  let bfError = $state('')
  let bfSuccess = $state('')
  let lastMeasuredBF = $state(null)

  let bfMethod = $state('navy')
  let bfData = $state(null)

  // Body fat calculation inputs (Navy method)
  let bfCalcForm = $state({ neckCm: '', waistCm: '', hipsCm: '' })
  let calcLoading = $state(false)

  // Profile (used to determine sex-specific measurement requirements)
  let profile = $state(null)

  async function loadProfile() {
    try {
      profile = await api.getProfile()
    } catch {
      profile = null
    }
  }

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

  async function loadLastMeasuredBF() {
    try {
      const bios = await api.listBiometrics(daysAgo(90), today())
      // Find most recent with bodyFatPct > 0
    for (let i = bios.length - 1; i >= 0; i--) {
        if (bios[i].bodyFatPct > 0) {
          lastMeasuredBF = bios[i]
          break
        }
      }
    } catch {
      lastMeasuredBF = null
    }
  }

  async function submitBodyFat() {
    bfError = ''
    bfSuccess = ''
    bfLoading = true
    try {
      await api.postBiometric({
        date: bfForm.date,
        bodyFatPct: Number(bfForm.bodyFatPct) || 0,
      })
      bfSuccess = 'Body fat % saved'
        bfForm = { date: today(), bodyFatPct: '' }
      await loadLastMeasuredBF()
    } catch (e) {
      bfError = e.message
    } finally {
      bfLoading = false
    }
  }

  async function submitCalcMeasurement() {
    bfError = ''
    calcLoading = true
    try {
      // Save measurements first
      await api.postMeasurement({
        date: bfForm.date,
        neckCm: inputLength(bfCalcForm.neckCm, store.units),
        waistCm: inputLength(bfCalcForm.waistCm, store.units),
        hipsCm: inputLength(bfCalcForm.hipsCm, store.units),
      })
      // Refresh measurements list (optional) and fetch calculated body fat
      await loadMeasurements()
      bfData = await api.getBodyFat('navy')
    } catch (e) {
      bfError = e.message
    } finally {
      calcLoading = false
    }
  }

  onMount(() => {
    // Check if we're editing existing data
    if (store.editData) {
        if (store.editData.type === 'biometric') {
        const row = store.editData.data
        dailyForm = { 
          date: row.date, 
          weightKg: String(row.weightKg ?? ''),
          gripKg: String(row.gripKg ?? ''),
          boltScore: String(row.boltScore ?? ''),
          sleepHours: String(row.sleepHours ?? ''),
          sleepQuality: String(row.sleepQuality ?? ''),
          subjective: String(row.subjective ?? ''),
          notes: row.notes ?? ''
        }
        tab = 'daily'
      } else if (store.editData.type === 'measurement') {
        const row = store.editData.data
        measForm = {
          date: row.date,
          neckCm: String(row.neckCm ?? ''),
          chestCm: String(row.chestCm ?? ''),
          waistCm: String(row.waistCm ?? ''),
          hipsCm: String(row.hipsCm ?? ''),
          thighCm: String(row.thighCm ?? ''),
          bicepCm: String(row.bicepCm ?? ''),
          shouldersCm: String(row.shouldersCm ?? ''),
          calvesCm: String(row.calvesCm ?? ''),
          notes: row.notes ?? ''
        }
        tab = 'measurements'
      }
      clearEditData()
    }
    loadMeasurements()
    loadBFHistory()
    loadBodyFat()
    loadProfile()
    loadLastMeasuredBF()
    // Load weekly checkin preview
    loadCheckinPreview()
  })
</script>

  <div class="max-w-screen-xl mx-auto">
  <!-- Tab bar — same style as LogMeal -->
  <div class="flex space-x-2 mb-4">
    <button class={tab === 'daily' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'daily'}>Daily</button>
    <button class={tab === 'measurements' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'measurements'}>Measurements</button>
    <button class={tab === 'bodyfat' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'bodyfat'}>Body Fat %</button>
    <button class="px-3 py-2 rounded-lg {tab === 'weekly' ? 'btn-primary' : (checkinPreview?.canCheckIn && !checkinSaved ? 'bg-emerald-600 text-white font-semibold' : 'bg-gray-700')}" onclick={() => tab = 'weekly'}>Weekly Target</button>
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
          <input class="input" id="ci-weight" type="number" step="0.1" bind:value={dailyForm.weightKg} />
        </div>

        

        <div>
          <label class="text-xs text-gray-400" for="ci-grip">Grip Strength ({loadUnit(store.units)})</label>
          <input class="input" id="ci-grip" type="number" step="0.1" bind:value={dailyForm.gripKg} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-bolt">BOLT Score (s)</label>
          <input class="input" id="ci-bolt" type="number" bind:value={dailyForm.boltScore} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-sleep-hrs">Sleep (hrs)</label>
          <input class="input" id="ci-sleep-hrs" type="number" step="0.1" bind:value={dailyForm.sleepHours} />
        </div>

        <div>
          <label class="text-xs text-gray-400" for="ci-sleep-qual">Sleep Quality (1–{store.sleepQualityMax})</label>
           <input class="input" id="ci-sleep-qual" type="number" min="1" max={store.sleepQualityMax} bind:value={dailyForm.sleepQuality} />
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
            <input class="input" id="meas-neck" type="number" step="0.1" bind:value={measForm.neckCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-chest">Chest ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-chest" type="number" step="0.1" bind:value={measForm.chestCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-waist">Waist ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-waist" type="number" step="0.1" bind:value={measForm.waistCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-hips">Hips ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-hips" type="number" step="0.1" bind:value={measForm.hipsCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-thigh">Thigh ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-thigh" type="number" step="0.1" bind:value={measForm.thighCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-bicep">Bicep ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-bicep" type="number" step="0.1" bind:value={measForm.bicepCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-shoulders">Shoulders ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-shoulders" type="number" step="0.1" bind:value={measForm.shouldersCm} />
          </div>

          <div>
            <label class="text-xs text-gray-400" for="meas-calves">Calves ({lengthUnit(store.units)})</label>
            <input class="input" id="meas-calves" type="number" step="0.1" bind:value={measForm.calvesCm} />
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
      {#if bfHistory.length > 0}
        <Card title="Body Fat % History">
          <div class="overflow-x-auto">
            <table class="min-w-full text-sm">
              <thead><tr class="text-left text-gray-300"><th>Date</th><th>Body Fat %</th></tr></thead>
              <tbody>
                {#each bfHistory as h}
                  <tr class="border-t border-gray-800">
                    <td>{h.date}</td>
                    <td class="text-emerald-400 font-semibold">{((h.bodyFatPct ?? 0).toFixed(1))}%</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </Card>
      {/if}
    </div>

  <!-- Body Fat % tab -->
  {:else if tab === 'bodyfat'}
    {#if bfError}<Alert type="error" message={bfError} />{/if}
    {#if bfSuccess}<Alert type="success" message={bfSuccess} />{/if}
    
    <div class="space-y-4">
      <!-- Manual Entry -->
      <Card title="Log Body Fat %">
        <div class="space-y-3">
          <div>
            <label class="text-xs text-gray-400" for="bf-date">Date</label>
            <input class="input" id="bf-date" type="date" bind:value={bfForm.date} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="bf-pct">Body Fat %</label>
            <input class="input" id="bf-pct" type="number" step="0.1" min="1" max="60" bind:value={bfForm.bodyFatPct} placeholder="e.g. 15.5" />
          </div>
          <button class="btn-primary" onclick={submitBodyFat} disabled={bfLoading}>{bfLoading ? 'Saving…' : 'Save'}</button>
        </div>
      </Card>
      
      <!-- Last Measured -->
      {#if lastMeasuredBF}
        <Card title="Last Measured">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-2xl font-bold text-emerald-400">{((lastMeasuredBF.bodyFatPct ?? 0).toFixed(1))}<span class="text-lg text-gray-400 ml-1">%</span></div>
              <div class="text-xs text-gray-400">{lastMeasuredBF.date}</div>
            </div>
            <button class="text-sm text-cyan-400 hover:text-cyan-300" onclick={() => bfForm.date = lastMeasuredBF.date}>Use this date</button>
          </div>
        </Card>
      {/if}
      
      <!-- Calculate from Measurements -->
      <Card title="Calculate from Measurements">
        <div class="space-y-3">
          <p class="text-xs text-gray-400 mb-3">
            Enter measurements to calculate body fat % using the Navy method.
            {#if profile?.sex?.toLowerCase() === 'female'}
              Female formula uses neck, waist, and hip measurements.
            {:else}
              Male formula uses neck and waist measurements.
            {/if}
          </p>

          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="text-xs text-gray-400" for="bf-neck">Neck ({lengthUnit(store.units)})</label>
              <input class="input" id="bf-neck" type="number" step="0.1" bind:value={bfCalcForm.neckCm} />
            </div>
            <div>
              <label class="text-xs text-gray-400" for="bf-waist">Waist ({lengthUnit(store.units)})</label>
               <input class="input" id="bf-waist" type="number" step="0.1" bind:value={bfCalcForm.waistCm} />
            </div>
            {#if profile?.sex?.toLowerCase() === 'female'}
              <div class="col-span-2">
                <label class="text-xs text-gray-400" for="bf-hips">Hips ({lengthUnit(store.units)})</label>
                <input class="input" id="bf-hips" type="number" step="0.1" bind:value={bfCalcForm.hipsCm} />
              </div>
            {/if}
          </div>

          <button class="btn-primary" onclick={submitCalcMeasurement} disabled={calcLoading}>{calcLoading ? 'Calculating…' : 'Calculate & Save'}</button>
        </div>
      </Card>

      <!-- Calculated (Navy Method) -->
      {#if bfLoading}
        <Spinner />
      {:else if bfData && bfData.bf_pct > 0}
        <Card title="Calculated — Navy Method">
          <div class="space-y-3">
            <div class="text-4xl font-bold text-cyan-400">{(bfData.bf_pct ?? 0).toFixed(1)}<span class="text-lg text-gray-400 ml-1">%</span></div>
            <div class="grid grid-cols-2 gap-3 text-sm">
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400 mb-1">Lean Mass</div>
                <div class="text-lg font-semibold text-gray-100">{dispWeight(bfData.leanMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
              </div>
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400 mb-1">Fat Mass</div>
                <div class="text-lg font-semibold text-gray-100">{dispWeight(bfData.fatMassKg, store.units)} <span class="text-xs text-gray-400">{weightUnit(store.units)}</span></div>
              </div>
            </div>
            <p class="text-xs text-gray-500">Calculated using the U.S. Navy circumference method. Uses {(profile?.sex ?? '').toLowerCase() === 'female' ? 'neck + waist + hips' : 'neck + waist'} measurements.</p>
          </div>
        </Card>
      {:else}
        <Card title="Calculated — Navy Method">
          <p class="text-sm text-gray-400">
            {#if profile?.sex?.toLowerCase() === 'female'}
              Not enough data. Requires: weight + measurements (neck, waist, hips).
            {:else}
              Not enough data. Requires: weight + measurements (neck, waist).
            {/if}
          </p>
        </Card>
      {/if}
    </div>
  {:else if tab === 'weekly'}
    {#if weeklyError}<Alert type="error" message={weeklyError} />{/if}
    {#if weeklySuccess}<Alert type="success" message={weeklySuccess} />{/if}
    <div class="space-y-4">
      {#if !checkinPreview}
        <Card>
          <div class="p-4"><Spinner /></div>
        </Card>
      {:else}
        <Card title="Weekly Check-in Preview">
          <div class="space-y-3">
            <div>
            {#if checkinPreview.canCheckIn}
                <div class="text-emerald-400 font-semibold">✅ Ready for check-in</div>
              {:else}
                <div class="text-yellow-400 font-semibold">⏳ {checkinPreview.daysSinceLastCheckIn ?? 0} days since last check-in</div>
              {/if}
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400">Weight Start</div>
                <div class="text-lg font-semibold">{checkinPreview.weightStart ? checkinPreview.weightStart.toFixed(1) : (checkinPreview.weight_start ? checkinPreview.weight_start.toFixed(1) : '—')} kg</div>
              </div>
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400">Weight End</div>
                <div class="text-lg font-semibold">{checkinPreview.weightEnd ? checkinPreview.weightEnd.toFixed(1) : (checkinPreview.weight_end ? checkinPreview.weight_end.toFixed(1) : '—')} kg</div>
              </div>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400">Weight Change</div>
                <div class="text-lg font-semibold">{checkinPreview.weightChange ? checkinPreview.weightChange.toFixed(2) : (checkinPreview.weight_change ? checkinPreview.weight_change.toFixed(2) : '—')} kg</div>
              </div>
              <div class="bg-gray-700 rounded-lg p-3">
                <div class="text-xs text-gray-400">Expected vs Actual</div>
                 <div class="text-sm">Expected: {(checkinPreview.expectedWeightChange ?? 0).toFixed(2)} kg</div>
                  <div class="text-sm">Diff: {(checkinPreview.weightDiff ?? 0).toFixed(2)} kg</div>
              </div>
            </div>

            <div class="bg-gray-700 rounded-lg p-3">
              <div class="text-xs text-gray-400">Recommendation</div>
               <div class="text-sm">Reason: {checkinPreview.reason ?? '—'}</div>
                <div class="text-sm">Current Calories: {checkinPreview.caloriesBefore ?? '—'}</div>
                <div class="text-sm">Recommended: {checkinPreview.recommendedCalories ?? '—'}</div>
               <div class="text-sm">Adjustment: {checkinPreview.calorieAdjustment ? Math.round(checkinPreview.calorieAdjustment) : (checkinPreview.calorie_adjustment ? Math.round(checkinPreview.calorie_adjustment) : '—')}</div>
            </div>

            <div class="flex space-x-2">
              <button class="btn-primary" onclick={acceptCheckin} disabled={!checkinPreview.canCheckIn || weeklyLoading || checkinSaved}>{weeklyLoading ? 'Processing…' : (checkinSaved ? 'Saved' : 'Accept Changes')}</button>
              <button class="bg-gray-700 px-3 py-2 rounded-lg" disabled={!checkinPreview.canCheckIn}>Skip</button>
            </div>
          </div>
        </Card>
      {/if}
    </div>
  {/if}
</div>
