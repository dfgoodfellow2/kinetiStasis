<script>
  import { api } from '../lib/api.js'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { onMount } from 'svelte'
  import { today, dispLoad, loadUnit, inputLoad, distUnit, inputDist, elevUnit, inputElev } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'
  import AITab from '../components/AITab.svelte'
  import YAMLTab from '../components/YAMLTab.svelte'

  // ── Tab (persisted) ─────────────────────────────────────
  const TAB_KEY = 'workout_tab'
  const VALID_TABS = ['ai', 'simple', 'yaml']
  const savedTab = typeof localStorage !== 'undefined' ? localStorage.getItem(TAB_KEY) : null
  let tab = $state(VALID_TABS.includes(savedTab) ? savedTab : 'ai')

  $effect(() => {
    localStorage.setItem(TAB_KEY, tab)
  })

  // ── Shared workout state ─────────────────────────────────
  let workout = $state({
    date: today(),
    slot: '1',
    title: '',
    type: 'strength',
    style: '',
    surface: '',
    focus: '',
    restInterval: '',
    durationMin: '',

    avgHr: '',
    maxHr: '',
    caloriesBurned: '',
    notes: '',
    coachNotes: '',
    exercises: [],
  })

  let loading = $state(false)
  let error = $state('')
  let success = $state('')

  // ── Parse state (set when child components call onParsed) ──
  let parseReady = $state(false)

  function onParsed() {
    parseReady = true
  }

  function blankExercise() {
    return {
      name: '', sets: '', reps: '', duration: '', weightLbs: '',
    tempo: '', rpe: '', pattern: '', bias: '', pace: '',
    distanceKm: '', elevationM: '',
      _sets: [],
    }
  }

  function addExercise() {
    workout = { ...workout, exercises: [...workout.exercises, blankExercise()] }
  }

  function removeExercise(idx) {
    workout = { ...workout, exercises: workout.exercises.filter((_, i) => i !== idx) }
  }

  // ── Save ─────────────────────────────────────────────────
  async function saveWorkout() {
    error = ''
    success = ''
    loading = true
    try {
      // save in progress
      const calVal = workout.caloriesBurned
      const payload = {
        date:            workout.date,
        slot:            String(workout.slot || '1'),
        title:           workout.title,
        durationMin:     Number(workout.durationMin)    || 0,
        caloriesBurned:  Number(calVal) || 0,
        rawNotes:        [workout.notes, workout.coachNotes].filter(Boolean).join('\n\n'),
        metadata: {
          type:          workout.type         || '',
          style:         workout.style        || '',
          surface:       workout.surface      || '',
          focus:         workout.focus ? workout.focus.split(',').map(s => s.trim()).filter(Boolean) : [],
          restInterval:  workout.restInterval || '',
          avgHr:         Number(workout.avgHr) || 0,
          maxHr:         Number(workout.maxHr) || 0,
        },
        exercises: workout.exercises.map(ex => {
          // If we have preserved _sets from a parse result, use them directly
          // Otherwise build sets array from flat form fields
          let sets
           if (Array.isArray(ex._sets) && ex._sets.length > 0) {
            // sanitize preserved _sets to match ExerciseSet model (camelCase)
            sets = ex._sets.map(s => ({
              reps: s.reps || 0,
              loadKg: s.loadKg || 0,
              loadLbs: s.loadLbs || 0,
              tutSeconds: s.tutSeconds || 0,
              restSeconds: s.restSeconds || 0,
            }))
            } else {
            const n = Number(ex.sets) || 1
            const reps = Number(ex.reps) || 0
            const loadNum = parseLoad(ex.weightLbs)
            // Re-compute TUT from tempo if user typed it in the form
            let tutSeconds = 0
            if (ex.tempo && reps > 0) {
              tutSeconds = ex.tempo.split('-').map(Number).filter(n => !isNaN(n)).reduce((a, b) => a + b, 0) * reps
            }
            sets = Array.from({ length: n }, () => ({
              reps,
              loadKg: inputLoad(loadNum, store.units),
              tutSeconds: tutSeconds,
              restSeconds: 0,
            }))
          }
           return {
              name:         ex.name     || '',
              category:     ex.pattern  || '',
              bias:         ex.bias     || '',
              tempo:        ex.tempo    || '',
              sets,
              rpe:          ex.rpe !== '' ? Number(ex.rpe) : 0,
              loadRaw:      ex.weightLbs || '',
              durationRaw:  ex.duration || '',
              distanceKm:   ex.distanceKm !== '' ? inputDist(ex.distanceKm, store.units) : 0,
              elevationM:   ex.elevationM !== '' ? inputElev(ex.elevationM, store.units) : 0,
              pace:         ex.pace || '',
            }
         }),
      }
      await api.postWorkout(payload)
      success = 'Workout saved!'
       workout = {
         date: today(), slot: '1', title: '', type: 'strength', style: '', surface: '',
         focus: '', restInterval: '', durationMin: '', avgHr: '', maxHr: '',
         caloriesBurned: '', notes: '', coachNotes: '', exercises: [],
       }
      // reset parse states in parent — child components manage their own raw/parsed state
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  // ── Live summary helpers ──────────────────────────────────
  // parseLoad mirrors the Go service: "BW"→0, "35+35 lbs"→70
  function parseLoad(load) {
    if (!load || String(load).toUpperCase() === 'BW') return 0
    return String(load)
      .replace(/lbs?/i, '')
      .split('+')
      .map(s => parseFloat(s.trim()))
      .filter(n => !isNaN(n))
      .reduce((a, b) => a + b, 0)
  }

  function computeTUT(tempo, reps, sets) {
    if (!tempo) return null
    const parts = String(tempo).split('-').map(Number)
    if (parts.length < 2 || parts.some(isNaN)) return null
    return parts.reduce((a, b) => a + b, 0) * (Number(reps) || 0) * (Number(sets) || 0)
  }

  function epleyPct(load, reps) {
    const l = Number(load) || 0; const r = Number(reps) || 0
    if (l <= 0 || r <= 0) return null
    return Math.round((l / (l * (1 + r / 30))) * 100)
  }

  function exerciseLine(ex) {
    // sets/reps: prefer _sets array length if available
    const sets = (Array.isArray(ex._sets) && ex._sets.length > 0)
      ? ex._sets.length
      : (Number(ex.sets) || 0)
    const reps = (Array.isArray(ex._sets) && ex._sets[0])
      ? ex._sets[0].reps
      : (Number(ex.reps) || 0)
    const load = ex.weightLbs || ''
    const loadNum = parseLoad(load)

    // Build duration fallback chain similar to History view:
    // 1. Prefer exercise-level duration if present (check camelCase and snake_case)
    // 2. If reps <= 0 (timed exercise) and workout has only 1 exercise, fall back to workout.durationMin
    // Check both camelCase (new) and snake_case (old DB format)
    const durationValue = ex.durationRaw || ex.duration || ''
    const workoutHasOneExercise = workout.exercises && workout.exercises.length === 1
    const shouldUseWorkoutDuration = reps <= 0 && !durationValue && workoutHasOneExercise && workout.durationMin
    const hasDuration = durationValue || (shouldUseWorkoutDuration ? `${workout.durationMin} min` : '')
    // Priority: duration if reps <= 0, else reps
    const displayValue = (reps <= 0 && hasDuration) ? durationValue : reps
    let base = `${ex.name || 'Exercise'}: ${sets}×${displayValue}`
    if (load) base += ` @ ${load}`

    const extras = []
    const tut = computeTUT(ex.tempo, reps, sets)
    if (tut) extras.push(`TUT: ${tut}s`)
    const vol = loadNum > 0 ? Math.round(loadNum * (reps > 0 ? reps : 1) * sets) : 0
    if (vol > 0) extras.push(`vol: ${vol}`)
    const pct = epleyPct(loadNum, reps)
    if (pct) extras.push(`${pct}%1RM`)
    if (ex.pace) extras.push(`pace: ${ex.pace}`)
    if (extras.length) base += ` (${extras.join(', ')})`
    if (ex.pattern) {
      const biasTag = ex.bias === 'bilateral' ? '(B)' : ex.bias === 'unilateral' ? '(U)' : ''
      base += ` [${ex.pattern}${biasTag ? ' ' + biasTag : ''}]`
    }
    if (ex.rpe !== '' && ex.rpe !== null && ex.rpe !== undefined) base += ` RPE ${ex.rpe}`
    return base
  }

  let summaryLines = $derived(workout.exercises.map(exerciseLine))
  let totalVol = $derived(workout.exercises.reduce((acc, ex) => {
    // Use _sets if available for accurate volume
    if (Array.isArray(ex._sets) && ex._sets.length > 0) {
      return acc + ex._sets.reduce((s, set) => {
        return s + (set.loadKg > 0 && set.reps > 0 ? Math.round(set.loadKg * set.reps) : 0)
      }, 0)
    }
    const l = parseLoad(ex.weightLbs)
    const r = Number(ex.reps) || 0
    const s = Number(ex.sets) || 0
    return acc + (l > 0 ? Math.round(l * r * s) : 0)
  }, 0))

  onMount(() => {
    // Check if editing existing workout
    if (store.editData && store.editData.type === 'workout') {
      const w = store.editData.data

      // Basic fields
      workout.date = w.date || today()
      workout.slot = String(w.slot || '1')
      workout.title = w.title || ''
      workout.type = w.type || 'strength'
      workout.style = w.style || ''
      workout.surface = w.surface || ''
      // Focus is stored under metadata.focus as an array - join into a display string
      workout.focus = Array.isArray(w.metadata?.focus)
        ? w.metadata.focus.join(', ')
        : (w.metadata?.focus ?? w.focus ?? '')
      workout.restInterval = w.metadata?.restInterval ?? w.restInterval ?? ''
      workout.durationMin = String(w.durationMin ?? '')
      // session-level RPE removed: no assignment to workout.rpe
      // avg_hr/max_hr live under metadata in the API model
      workout.avgHr = w.metadata?.avgHr ? String(w.metadata.avgHr) : (w.metadata?.avgHr ? String(w.metadata.avgHr) : '')
      workout.maxHr = w.metadata?.maxHr ? String(w.metadata.maxHr) : (w.metadata?.maxHr ? String(w.metadata.maxHr) : '')
      workout.caloriesBurned = String(w.caloriesBurned ?? '')
      workout.notes = w.rawNotes ?? ''
      workout.coachNotes = w.coachNotes ?? ''

      // Map ALL exercise fields from API to simple tab format
      workout.exercises = (w.exercises || []).map(ex => {
        const exerciseSets = ex.sets || []
        const firstExerciseSet = exerciseSets[0] || {}
        return {
          // Basic fields
          name: ex.name || '',
          // pattern is the movement pattern (squat, push, pull, hinge)
          pattern: ex.category || '',
          // UI type field - leave empty for simple tab
          type: '',
          surface: ex.surface || '',
          notes: ex.notes || '',
          
          // Strength: sets/reps/weight
          sets: exerciseSets.length > 0 ? String(exerciseSets.length) : '',
          reps: firstExerciseSet.reps ? String(firstExerciseSet.reps) : '',
          weightLbs: firstExerciseSet.loadLbs ? String(Math.round(firstExerciseSet.loadLbs)) : '',
          
          // Conditioning: distance/elevation/pace/duration (JSON uses snake_case)
          distanceKm: ex.distanceKm ?? '',
          elevationM: ex.elevationM ?? '',
          pace: ex.pace || '',
          duration: ex.durationRaw || ex.duration || '',
          
          // Load info
          load: ex.loadRaw || (firstExerciseSet.loadLbs ? `${Math.round(firstExerciseSet.loadLbs)} lbs` : ''),

          // RPE and Tempo
          rpe: String(ex.rpe || ''),
          tempo: ex.tempo || '',

          // Bias
          bias: ex.bias || '',
          
          // _sets array preserves all set data for save
           _sets: exerciseSets.map(s => ({
            loadKg: s.loadKg ?? 0,
            loadLbs: s.loadLbs ?? 0,
            reps: s.reps || 0,
            // strip any set-level RPE (ExerciseSet model does not include RPE)
            tempo: s.tempo || '',
            duration: s.duration ?? '',
          }))
        }
      })

      tab = 'simple'
      clearEditData()
    }
  })
</script>

    <div class="max-w-screen-xl mx-auto">
  <!-- Tab bar -->
  <div class="flex space-x-2 mb-4">
    <button class={tab === 'ai'     ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'ai'}>AI</button>
    <button class={tab === 'simple' ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'simple'}>Simple</button>
    <button class={tab === 'yaml'   ? 'btn-primary' : 'bg-gray-700 px-3 py-2 rounded-lg'} onclick={() => tab = 'yaml'}>YAML</button>
  </div>

  {#if error}<Alert type="error" message={error} />{/if}
  {#if success}<Alert type="success" message={success} />{/if}

  <!-- ═══════════════════════ AI TAB ═══════════════════════ -->
  {#if tab === 'ai'}
    <div>
      <AITab bind:workout={workout} onParsed={onParsed} />
      {#if parseReady}
        <div class="border-t border-gray-700 pt-3 space-y-3">
          <p class="text-xs text-gray-400 uppercase">Parsed — edit before saving</p>
          {@render sessionFields()}
          {@render exerciseList()}
          <button class="btn-primary" onclick={saveWorkout} disabled={loading}>{loading ? 'Saving…' : 'Save Workout'}</button>
        </div>
      {/if}
    </div>

  <!-- ═══════════════════════ SIMPLE TAB ═══════════════════ -->
  {:else if tab === 'simple'}
    <div class="bg-gray-800 p-4 rounded-lg border border-gray-700 space-y-3">
      {@render sessionFields()}
      {@render exerciseList()}
      <button class="btn-primary" onclick={saveWorkout} disabled={loading}>{loading ? 'Saving…' : 'Save Workout'}</button>
    </div>

  <!-- ═══════════════════════ YAML TAB ═══════════════════════ -->
  {:else if tab === 'yaml'}
    <div>
      <YAMLTab bind:workout={workout} onParsed={onParsed} />
        <div class="mt-2">
        <label class="text-xs text-gray-400 block mb-1" for="wo-yaml-coach-notes">Coach's Note</label>
        <textarea class="input" id="wo-yaml-coach-notes" rows="2" placeholder="Context, feedback, how it felt…" bind:value={workout.coachNotes}></textarea>
      </div>
      {#if parseReady}
        <div class="border-t border-gray-700 pt-3 space-y-3">
          <p class="text-xs text-gray-400 uppercase">Parsed — edit before saving</p>
          {@render sessionFields()}
          {@render exerciseList()}
          <button class="btn-primary" onclick={saveWorkout} disabled={loading}>{loading ? 'Saving…' : 'Save Workout'}</button>
        </div>
      {/if}
    </div>
  {/if}

  <!-- ═══════════════════════ LIVE SUMMARY ══════════════════ -->
  {#if summaryLines.length > 0 || workout.durationMin || workout.caloriesBurned}
    <div class="mt-4 bg-gray-800 p-4 rounded-lg border border-gray-700">
      <p class="text-sm font-semibold text-gray-300 mb-2">Summary</p>
      {#if summaryLines.length > 0}
        <ul class="space-y-1 mb-3">
          {#each summaryLines as line}
            <li class="text-sm text-gray-200 font-mono">{line}</li>
          {/each}
        </ul>
      {/if}
      <div class="flex flex-wrap gap-4 text-sm text-gray-400">
        {#if workout.type}<span>Type: <span class="text-white">{workout.type}</span></span>{/if}
        {#if workout.style}<span>Style: <span class="text-white">{workout.style}</span></span>{/if}
        {#if workout.durationMin}<span>Duration: <span class="text-white">{workout.durationMin} min</span></span>{/if}
        {#if totalVol > 0}<span>Total Vol: <span class="text-white">{dispLoad(totalVol, store.units)} {loadUnit(store.units)}</span></span>{/if}
        {#if workout.caloriesBurned}<span>Cal: <span class="text-white">{workout.caloriesBurned} kcal</span></span>{/if}
        <!-- session-level RPE removed -->
        {#if workout.avgHr}<span>Avg HR: <span class="text-white">{workout.avgHr} bpm</span></span>{/if}
        {#if workout.focus}<span>Focus: <span class="text-white">{workout.focus}</span></span>{/if}
      </div>
    </div>
  {/if}
</div>

<!-- ═══════════════════════ SNIPPETS ══════════════════════════ -->
{#snippet sessionFields()}
    <div class="grid grid-cols-2 gap-3">
    <div>
      <label class="text-xs text-gray-400" for="wo-date">Date</label>
      <input class="input" id="wo-date" type="date" bind:value={workout.date} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-slot">Slot</label>
      <input class="input" id="wo-slot" type="text" placeholder="AM / PM / 1 / 2" bind:value={workout.slot} />
    </div>
    <div class="col-span-2">
      <label class="text-xs text-gray-400" for="wo-title">Workout Name</label>
      <input class="input" id="wo-title" placeholder="e.g. Hinge + Push + Pull" bind:value={workout.title} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-type">Type</label>
      <select class="input" id="wo-type" bind:value={workout.type}>
        <option value="strength">Strength</option>
        <option value="conditioning">Conditioning</option>
        <option value="hybrid">Hybrid</option>
      </select>
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-style">Style</label>
      <input class="input" id="wo-style" placeholder="circuit / emom / amrap / hiit" bind:value={workout.style} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-surface">Surface</label>
      <input class="input" id="wo-surface" placeholder="gym / outdoor / home" bind:value={workout.surface} />
    </div>
      <div>
      <label class="text-xs text-gray-400" for="wo-rest">Rest Interval</label>
      <input class="input" id="wo-rest" placeholder="e.g. 1 min" bind:value={workout.restInterval} />
    </div>
      <div>
      <label class="text-xs text-gray-400" for="wo-duration">Duration (min)</label>
      <input class="input" id="wo-duration" type="number" bind:value={workout.durationMin} />
    </div>
    <!-- session-level RPE removed -->
    <div>
      <label class="text-xs text-gray-400" for="wo-avg-hr">Avg HR (bpm)</label>
      <input class="input" id="wo-avg-hr" type="number" bind:value={workout.avgHr} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-max-hr">Max HR (bpm)</label>
      <input class="input" id="wo-max-hr" type="number" bind:value={workout.maxHr} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-cal">Calories burned</label>
      <input class="input" id="wo-cal" type="number" bind:value={workout.caloriesBurned} />
    </div>
    <div class="col-span-2">
      <label class="text-xs text-gray-400" for="wo-focus">Focus (movement patterns)</label>
      <input class="input" id="wo-focus" placeholder="Squat(B), Hinge(U)…" bind:value={workout.focus} />
    </div>
    <div class="col-span-2">
      <label class="text-xs text-gray-400" for="wo-notes">Notes</label>
      <textarea class="input" id="wo-notes" rows="2" placeholder="Optional notes…" bind:value={workout.notes}></textarea>
    </div>
    <div class="col-span-2">
      <label class="text-xs text-gray-400" for="wo-coach-notes">Coach's Note</label>
      <textarea class="input" id="wo-coach-notes" rows="2" placeholder="Context, feedback, how it felt…" bind:value={workout.coachNotes}></textarea>
    </div>
  </div>
{/snippet}

{#snippet exerciseList()}
  <div>
    <p class="text-sm font-semibold text-emerald-400 mb-2">Exercises</p>
    {#each workout.exercises as ex, idx}
      <div class="bg-gray-700 border border-gray-600 rounded-lg p-3 mb-3">
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm font-medium text-white">{ex.name || 'Exercise ' + (idx + 1)}</span>
          <button class="text-xs text-gray-500 hover:text-red-400" onclick={() => removeExercise(idx)}>✕ Remove</button>
        </div>
        <div class="grid grid-cols-3 gap-2">
          <div class="col-span-3">
            <label class="text-xs text-gray-400" for="ex-{idx}-name">Name</label>
            <input class="input" id="ex-{idx}-name" bind:value={ex.name} placeholder="e.g. KB Deadlift" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-sets">Sets</label>
            <input class="input" id="ex-{idx}-sets" type="number" bind:value={ex.sets} oninput={() => ex._sets = []} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-reps">Reps</label>
            <input class="input" id="ex-{idx}-reps" type="number" bind:value={ex.reps} oninput={() => ex._sets = []} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-duration">Duration (timed)</label>
            <input class="input" id="ex-{idx}-duration" bind:value={ex.duration} placeholder="35 sec" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-load">Load ({loadUnit(store.units)})</label>
            <input class="input" id="ex-{idx}-load" bind:value={ex.weightLbs} placeholder="BW / 85 lbs" oninput={() => ex._sets = []} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-tempo">Tempo</label>
            <input class="input" id="ex-{idx}-tempo" bind:value={ex.tempo} placeholder="2-0-2-0" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-rpe">RPE</label>
            <input class="input" id="ex-{idx}-rpe" type="number" min="1" max="10" bind:value={ex.rpe} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-pattern">Pattern</label>
            <select class="input" id="ex-{idx}-pattern" bind:value={ex.pattern}>
              <option value="">—</option>
              <option value="squat">Squat</option>
              <option value="hinge">Hinge</option>
              <option value="push">Push</option>
              <option value="pull">Pull</option>
              <option value="conditioning">Conditioning</option>
              <option value="core">Core</option>
              <option value="carry">Carry</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-bias">Bias</label>
            <select class="input" id="ex-{idx}-bias" bind:value={ex.bias}>
              <option value="">—</option>
              <option value="bilateral">Bilateral (B)</option>
              <option value="unilateral">Unilateral (U)</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-pace">Pace (min/km)</label>
            <input class="input" id="ex-{idx}-pace" bind:value={ex.pace} placeholder="5:30" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-distance">Distance ({distUnit(store.units)})</label>
            <input class="input" id="ex-{idx}-distance" type="number" step="0.1" bind:value={ex.distanceKm} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-elevation">Elevation ({elevUnit(store.units)})</label>
            <input class="input" id="ex-{idx}-elevation" type="number" bind:value={ex.elevationM} />
          </div>
        </div>
      </div>
    {/each}
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={addExercise}>+ Add Exercise</button>
  </div>
{/snippet}
