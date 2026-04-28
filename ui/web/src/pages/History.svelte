<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store, setCurrentPage } from '../lib/stores.svelte.js'
  import { daysAgo, today, fmt0, dispWeight, weightUnit, dispLength, lengthUnit, dispLoad, loadUnit, dispDist, distUnit } from '../lib/utils.js'
  import Spinner from '../components/Spinner.svelte'

  let tab = $state('nutrition')

  // --- Nutrition ---
  let nutLogs = $state([])
  let nutLoading = $state(true)

  async function loadNutrition() {
    nutLoading = true
    try {
      nutLogs = await api.listNutrition(daysAgo(30), today())
    } catch {
      nutLogs = []
    } finally {
      nutLoading = false
    }
  }

  // --- Biometrics ---
  let bioLogs = $state([])
  let bioLoading = $state(true)

  async function loadBiometrics() {
    bioLoading = true
    try {
      bioLogs = await api.listBiometrics(daysAgo(30), today())
    } catch {
      bioLogs = []
    } finally {
      bioLoading = false
    }
  }

  // --- Workouts ---
  // API returns flat []WorkoutEntry — group by date for display
  let wrkData = $state([])
  let wrkLoading = $state(true)

  async function loadWorkouts() {
    wrkLoading = true
    try {
      const flat = await api.listWorkouts(daysAgo(30), today())
      // Group flat entries by date, preserving date-ascending order
      const map = new Map()
      for (const entry of (flat ?? [])) {
        if (!map.has(entry.date)) map.set(entry.date, [])
        map.get(entry.date).push(entry)
      }
      wrkData = Array.from(map.entries()).map(([date, workouts]) => ({ date, workouts }))
    } catch {
      wrkData = []
    } finally {
      wrkLoading = false
    }
  }

  // --- Measurements ---
  let measData = $state([])
  let measLoading = $state(true)

  async function loadMeasurements() {
    measLoading = true
    try {
      measData = await api.listMeasurements(daysAgo(30), today())
    } catch {
      measData = []
    } finally {
      measLoading = false
    }
  }

  onMount(() => {
    loadNutrition()
    loadBiometrics()
    loadWorkouts()
    loadMeasurements()
  })

  // --- Actions (edit / delete) ---
  async function editNut(row) { setCurrentPage('logmeal') }

  async function deleteNut(date) {
    if (!confirm(`Delete nutrition log for ${date}?`)) return
    try {
      await api.deleteNutrition(date)
      await loadNutrition()
    } catch (err) {
      alert(`Delete failed: ${err.message || err}`)
    }
  }

  async function editBio(row) { setCurrentPage('checkin') }

  async function deleteBio(date) {
    if (!confirm(`Delete biometrics for ${date}?`)) return
    try {
      await api.deleteBiometric(date)
      await loadBiometrics()
    } catch (err) {
      alert(`Delete failed: ${err.message || err}`)
    }
  }

  async function editW(date, slot) { setCurrentPage('workoutlog') }

  async function deleteW(date, slot) {
    if (!confirm(`Delete workout on ${date} (slot ${slot})?`)) return
    try {
      await api.deleteWorkout(date, slot)
      await loadWorkouts()
    } catch (err) {
      alert(`Delete failed: ${err.message || err}`)
    }
  }

  async function editMeas(row) {
    // Navigate to checkin with measurement data pre-filled
    // Could store in a store and navigate, or just alert for now
    setCurrentPage('checkin')
  }

  async function deleteMeas(date) {
    if (!confirm('Delete measurement for ' + date + '?')) return
    try {
      await api.deleteMeasurement(date)
      loadMeasurements()
    } catch (e) {
      console.error('Delete failed:', e)
    }
  }
</script>

<div class="max-w-4xl mx-auto">
  <!-- Tab bar -->
  <div class="flex space-x-2 mb-4">
    <button class={tab === 'nutrition' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'nutrition'}>Nutrition</button>
    <button class={tab === 'biometrics' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'biometrics'}>Biometrics</button>
    <button class={tab === 'measurements' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'measurements'}>Measurements</button>
    <button class={tab === 'workout' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'workout'}>Workout</button>
  </div>

  <!-- Nutrition tab -->
  {#if tab === 'nutrition'}
    {#if nutLoading}
      <Spinner />
    {:else if nutLogs.length === 0}
      <div class="text-gray-400">No nutrition logs in the last 30 days.</div>
    {:else}
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="text-left text-gray-300">
              <th class="py-2 pr-4">Date</th>
              <th class="pr-4">Cal</th>
              <th class="pr-4">Protein</th>
              <th class="pr-4">Carbs</th>
              <th class="pr-4">Fat</th>
              <th>Notes</th>
            </tr>
          </thead>
          <tbody>
            {#each nutLogs as row}
              <tr class="border-t border-gray-800">
                <td class="py-2 pr-4">{row.date}</td>
                <td class="pr-4">{fmt0(row.calories)}</td>
                <td class="pr-4">{fmt0(row.protein_g)}g</td>
                <td class="pr-4">{fmt0(row.carbs_g)}g</td>
                <td class="pr-4">{fmt0(row.fat_g)}g</td>
                <td class="text-gray-400 text-xs">{row.meal_notes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-2" onclick={() => editNut(row)} title="Edit">✏️</button>
                  <button class="text-gray-400 hover:text-red-400" onclick={() => deleteNut(row.date)} title="Delete">🗑️</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  <!-- Biometrics tab -->
  {:else if tab === 'biometrics'}
    {#if bioLoading}
      <Spinner />
    {:else if bioLogs.length === 0}
      <div class="text-gray-400">No biometric logs in the last 30 days.</div>
    {:else}
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
              <tr class="text-left text-gray-300">
                <th class="py-2 pr-4">Date</th>
                <th class="pr-4">Weight ({weightUnit(store.units)})</th>
                <th class="pr-4">Waist ({lengthUnit(store.units)})</th>
                <th class="pr-4">Shoulders</th>
                <th class="pr-4">Calves</th>
                <th class="pr-4">Sleep h</th>
                <th class="pr-4">Sleep Q</th>
                <th class="pr-4">Feel</th>
                <th class="pr-4">Grip (kg)</th>
                <th class="pr-4">BOLT</th>
                <th>Notes</th>
              </tr>
          </thead>
          <tbody>
            {#each bioLogs as row}
              <tr class="border-t border-gray-800">
                <td class="py-2 pr-4">{row.date}</td>
                 <td class="pr-4">{row.weight_kg ? dispWeight(row.weight_kg, store.units) : '—'}</td>
                 <td class="pr-4">{row.waist_cm ? dispLength(row.waist_cm, store.units) : '—'}</td>
                 <td class="pr-4">{row.shoulders_cm ? dispLength(row.shoulders_cm, store.units) : '—'}</td>
                 <td class="pr-4">{row.calves_cm ? dispLength(row.calves_cm, store.units) : '—'}</td>
                <td class="pr-4">{row.sleep_hours ?? '—'}</td>
                <td class="pr-4">{row.sleep_quality ?? '—'}</td>
                <td class="pr-4">{row.subjective_feel ?? '—'}</td>
                <td class="pr-4">{row.grip_kg ?? '—'}</td>
                <td class="pr-4">{row.bolt_score ?? '—'}</td>
                <td class="text-gray-400 text-xs">{row.notes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-2" onclick={() => editBio(row)} title="Edit">✏️</button>
                  <button class="text-gray-400 hover:text-red-400" onclick={() => deleteBio(row.date)} title="Delete">🗑️</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  <!-- Measurements tab -->
  {:else if tab === 'measurements'}
    {#if measLoading}
      <Spinner />
    {:else if measData.length === 0}
      <div class="text-gray-400">No measurements in the last 30 days.</div>
    {:else}
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="text-left text-gray-300">
              <th class="py-2 pr-4">Date</th>
              <th class="pr-4">Neck</th>
              <th class="pr-4">Chest</th>
              <th class="pr-4">Waist</th>
              <th class="pr-4">Hips</th>
              <th class="pr-4">Thigh</th>
              <th class="pr-4">Bicep</th>
              <th class="pr-4">Shoulders</th>
              <th class="pr-4">Calves</th>
              <th>Notes</th>
            </tr>
          </thead>
          <tbody>
            {#each measData as row}
              <tr class="border-t border-gray-800">
                <td class="py-2 pr-4">{row.date}</td>
                <td class="pr-4">{dispLength(row.neck_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.chest_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.waist_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.hips_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.thigh_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.bicep_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.shoulders_cm, store.units)}</td>
                <td class="pr-4">{dispLength(row.calves_cm, store.units)}</td>
                <td class="text-gray-400 text-xs">{row.notes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-2" onclick={() => editMeas(row)} title="Edit">✏️</button>
                  <button class="text-gray-400 hover:text-red-400" onclick={() => deleteMeas(row.date)} title="Delete">🗑️</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  <!-- Workout tab -->
  {:else if tab === 'workout'}
    {#if wrkLoading}
      <Spinner />
    {:else if wrkData.length === 0}
      <div class="text-gray-400">No workouts logged in the last 30 days.</div>
    {:else}
      <div class="space-y-3">
        {#each wrkData as day}
          <div class="bg-gray-800 p-3 rounded border border-gray-700">
            <div class="font-bold text-emerald-400 mb-2">{day.date}</div>
            {#each day.workouts as w}
              <div class="mt-2 p-2 bg-gray-700 rounded">
                <div class="font-semibold">
                  {w.title}
                  {#if w.duration_min}<span class="text-gray-400 text-sm font-normal">({w.duration_min} min)</span>{/if}
                </div>
                {#if w.exercises && w.exercises.length > 0}
                    <ul class="mt-1 ml-3 list-disc text-sm text-gray-300">
                     {#each w.exercises as ex}
                       <li>
                         {#if ex.distance_km}
                           {ex.name} — {dispDist(ex.distance_km, store.units)} {distUnit(store.units)}{ex.pace ? ` @ ${ex.pace}/km` : ''}
                         {:else if ex.sets && ex.sets.length > 0}
                           {@const n = ex.sets.length}
                           {@const reps0 = ex.sets[0]?.reps ?? 0}
                           {@const allSame = ex.sets.every(s => s.reps === reps0)}
                           {@const loadStr = ex.load_raw || (ex.sets[0]?.load_lbs ? `${dispLoad(ex.sets[0].load_lbs * 0.45359237, store.units)} ${loadUnit(store.units)}` : 'BW')}
                           {ex.name} — {n}×{allSame && reps0 ? reps0 : '?'} @ {loadStr}
                         {:else}
                           {ex.name}
                         {/if}
                       </li>
                     {/each}
                   </ul>
                {/if}
                {#if w.raw_notes}
                  <div class="mt-1 text-xs text-gray-400 whitespace-pre-wrap">{w.raw_notes}</div>
                {/if}
                <div class="mt-1">
                  <button class="text-gray-400 hover:text-emerald-400 mr-2" onclick={() => editW(day.date, w.slot)} title="Edit">✏️</button>
                  <button class="text-gray-400 hover:text-red-400" onclick={() => deleteW(day.date, w.slot)} title="Delete">🗑️</button>
                </div>
              </div>
            {/each}
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>
