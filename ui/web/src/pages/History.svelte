<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { store, setCurrentPage, setEditData, clearEditData } from '../lib/stores.svelte.js'
  import { daysAgo, today, fmt0, dispWeight, weightUnit, dispLength, lengthUnit, dispLoad, loadUnit, dispDist, distUnit } from '../lib/utils.js'
  import Spinner from '../components/Spinner.svelte'

  let tab = $state('nutrition')

  // --- Nutrition ---
  let nutLogs = $state([])
  let nutLoading = $state(true)

  async function loadNutrition() {
    nutLoading = true
      try {
        nutLogs = (await api.listNutrition(daysAgo(30), today())).reverse()
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
        bioLogs = (await api.listBiometrics(daysAgo(30), today())).reverse()
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
        wrkData = Array.from(map.entries()).map(([date, workouts]) => ({ date, workouts })).reverse()
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
        measData = (await api.listMeasurements(daysAgo(30), today())).reverse()
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
  async function editNut(row) { 
    setEditData({ type: 'nutrition', data: row })
    setCurrentPage('logmeal') 
  }

  async function deleteNut(date) {
    if (!confirm(`Delete nutrition log for ${date}?`)) return
    try {
      await api.deleteNutrition(date)
      await loadNutrition()
    } catch (err) {
      alert(`Delete failed: ${err.message || err}`)
    }
  }

  async function editBio(row) { 
    setEditData({ type: 'biometric', data: row })
    setCurrentPage('checkin') 
  }

  async function deleteBio(date) {
    if (!confirm(`Delete biometrics for ${date}?`)) return
    try {
      await api.deleteBiometric(date)
      await loadBiometrics()
    } catch (err) {
      alert(`Delete failed: ${err.message || err}`)
    }
  }

  async function editW(w) { 
    setEditData({ type: 'workout', data: w })
    setCurrentPage('workoutlog') 
  }

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
    setEditData({ type: 'measurement', data: row })
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
    <button class={tab === 'nutrition' ? 'btn-primary' : 'btn-secondary'} onclick={() => tab = 'nutrition'}>Nutrition</button>
    <button class={tab === 'biometrics' ? 'btn-primary' : 'btn-secondary'} onclick={() => tab = 'biometrics'}>Biometrics</button>
    <button class={tab === 'measurements' ? 'btn-primary' : 'btn-secondary'} onclick={() => tab = 'measurements'}>Measurements</button>
    <button class={tab === 'workout' ? 'btn-primary' : 'btn-secondary'} onclick={() => tab = 'workout'}>Workout</button>
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
                <td class="pr-4">{fmt0(row.proteinG)}g</td>
                <td class="pr-4">{fmt0(row.carbsG)}g</td>
                <td class="pr-4">{fmt0(row.fatG)}g</td>
                <td class="text-gray-400 text-xs">{row.mealNotes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-3" onclick={() => editNut(row)} title="Edit">✏️</button>
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
                
                <th class="pr-4">Sleep h</th>
                <th class="pr-4">Sleep Q</th>
                <th class="pr-4">Feel</th>
                <th class="pr-4">Grip ({loadUnit(store.units)})</th>
                <th class="pr-4">BOLT</th>
                <th>Notes</th>
              </tr>
          </thead>
          <tbody>
            {#each bioLogs as row}
              <tr class="border-t border-gray-800">
                <td class="py-2 pr-4">{row.date}</td>
                <td class="pr-4">{row.weightKg ? dispWeight(row.weightKg, store.units) : '—'}</td>
                 
                <td class="pr-4">{row.sleepHours ?? '—'}</td>
                <td class="pr-4">{row.sleepQuality ?? '—'}</td>
                <td class="pr-4">{row.subjectiveFeel ?? '—'}</td>
                <td class="pr-4">{row.gripKg ? dispLoad(row.gripKg, store.units) : '—'}</td>
                <td class="pr-4">{row.boltScore ?? '—'}</td>
                <td class="text-gray-400 text-xs">{row.notes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-3" onclick={() => editBio(row)} title="Edit">✏️</button>
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
                <td class="pr-4">{dispLength(row.neckCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.chestCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.waistCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.hipsCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.thighCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.bicepCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.shouldersCm, store.units)}</td>
                <td class="pr-4">{dispLength(row.calvesCm, store.units)}</td>
                <td class="text-gray-400 text-xs">{row.notes ?? ''}</td>
                <td class="py-2">
                  <button class="text-gray-400 hover:text-emerald-400 mr-3" onclick={() => editMeas(row)} title="Edit">✏️</button>
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
      <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead>
            <tr class="text-left text-gray-300">
              <th class="py-2 pr-4">Date</th>
              <th class="pr-4">Title</th>
              <th class="pr-4">Duration</th>
              <th class="pr-4">Exercises</th>
              <th class="pr-4">Notes</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each wrkData as day}
              {#each day.workouts as w}
                <tr class="border-t border-gray-800">
                  <td class="py-2 pr-4">{day.date}</td>
                  <td class="pr-4 font-semibold">{w.title}</td>
                  <td class="pr-4">{w.durationMin ? w.durationMin + ' min' : '—'}</td>
                  <td class="pr-4 text-gray-400">
                    {#if w.exercises && w.exercises.length > 0}
                      {w.exercises.map(e => {
                        const sets = e.sets?.length || 0
                        const reps = e.sets?.[0]?.reps || 0

                        // Build duration fallback chain:
                        // 1. Check exercise-level duration first (e.durationRaw or e.duration)
                        // 2. If reps <= 0 (duration-based exercise) and workout has only 1 exercise,
                        //    fall back to workout duration (since it's the same as exercise duration)
                        // Prefer camelCase fields returned by the API
                        const exerciseDuration = e.durationRaw || e.duration
                        const workoutHasOneExercise = w.exercises && w.exercises.length === 1
                        const shouldUseWorkoutDuration = reps <= 0 && !exerciseDuration && workoutHasOneExercise && w.durationMin
                        const hasDuration = exerciseDuration || (shouldUseWorkoutDuration ? `${w.durationMin} min` : '')

                        if (sets > 0) {
                          if (reps <= 0 && hasDuration) {
                            return `${sets}×${hasDuration} ${e.name}`
                          } else {
                            return `${sets}×${reps} ${e.name}`
                          }
                        } else {
                          return e.name
                        }
                      }).join(', ')}
                    {:else}
                      —
                    {/if}
                  </td>
                  <td class="pr-4 text-gray-500 text-xs">{w.rawNotes || '—'}</td>
                  <td class="py-2">
                    <button class="text-gray-400 hover:text-emerald-400 mr-3" onclick={() => editW(w)} title="Edit">✏️</button>
                    <button class="text-gray-400 hover:text-red-400" onclick={() => deleteW(day.date, w.slot)} title="Delete">🗑️</button>
                  </td>
                </tr>
              {/each}
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
</div>
