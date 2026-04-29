<script>
  import { api } from '../lib/api.js'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { onMount } from 'svelte'
  import { today, dispLoad, loadUnit, inputLoad, distUnit, inputDist, elevUnit, inputElev } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'

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
    rest_interval: '',
    duration_min: '',
    rpe: '',
    avg_hr: '',
    max_hr: '',
    calories_burned: '',
    notes: '',
    coach_notes: '',
    exercises: [],
  })

  let loading = $state(false)
  let error = $state('')
  let success = $state('')

  // ── AI tab ───────────────────────────────────────────────
  let aiRaw = $state('')
  let aiParsed = $state(false)
  let aiParsing = $state(false)

  async function parseAI() {
    if (!aiRaw.trim()) return
    aiParsing = true
    error = ''
    try {
      const res = await api.parseWorkout(aiRaw.trim(), 'ai')
      if (res && !res.error) {
        applyParsed(res)
        aiParsed = true
      } else {
        error = 'AI parse failed: ' + (res?.error ?? 'unknown error')
      }
    } catch (e) {
      error = 'AI parse error: ' + e.message
    } finally {
      aiParsing = false
    }
  }

  // ── YAML tab ─────────────────────────────────────────────
  let yamlRaw = $state('')
  let yamlParsed = $state(false)
  let yamlParsing = $state(false)

  async function parseYAML() {
    if (!yamlRaw.trim()) return
    yamlParsing = true
    error = ''
    try {
      const res = await api.parseWorkout(yamlRaw.trim(), 'yaml')
      if (res && !res.error) {
        applyParsed(res)
        yamlParsed = true
      } else {
        error = 'YAML parse failed: ' + (res?.error ?? 'unknown error')
      }
    } catch (e) {
      error = 'YAML parse error: ' + e.message
    } finally {
      yamlParsing = false
    }
  }

  // ── Apply parsed result ───────────────────────────────────
  // res is a ParsedWorkout — all session metadata at the top level
  function applyParsed(res) {
    // Normalise focus: may be array or JSON string
    let focusStr = ''
    if (Array.isArray(res.focus) && res.focus.length > 0) {
      focusStr = res.focus.join(', ')
    } else if (typeof res.focus === 'string' && res.focus) {
      // Try to parse as JSON array in case server serialised it as string
      try {
        const parsed = JSON.parse(res.focus)
        focusStr = Array.isArray(parsed) ? parsed.join(', ') : res.focus
      } catch {
        focusStr = res.focus
      }
    }

    workout = {
      ...workout,
      date:            res.date           || today(),
      slot:            res.slot           ? String(res.slot) : workout.slot,
      title:           res.title          || workout.title,
      type:            res.type           || workout.type,
      style:           res.style          || workout.style,
      surface:         res.surface        || workout.surface,
      focus:           focusStr           || workout.focus,
      rest_interval:   res.rest_interval  || workout.rest_interval,
      duration_min:    res.duration_min   || workout.duration_min,
      rpe:             res.rpe            || workout.rpe,
      avg_hr:          res.avg_hr         || workout.avg_hr,
      max_hr:          res.max_hr         || workout.max_hr,
      calories_burned: res.calories_burned || workout.calories_burned,
      notes:           res.notes || res.raw_notes || workout.notes,
      exercises:       (res.exercises || []).map(mapExercise),
    }
  }

  // mapExercise converts a server ExerciseEntry (with nested sets[]) to flat UI form fields.
  function mapExercise(e) {
  // e.sets is []ExerciseSet = [{reps, load_kg, tut_seconds, rest_seconds}]
    const setsArr = Array.isArray(e.sets) ? e.sets : []
    const setCount = setsArr.length || ''
    const firstSet = setsArr[0] ?? {}

    // Prefer load_raw (original string like "BW", "35+35 lbs") for display
    let weightDisplay = e.load_raw || ''
    if (!weightDisplay && firstSet.load_kg > 0) {
      weightDisplay = String(firstSet.load_kg) + ' kg'
    }

    return {
      name:        e.name        || '',
      sets:        setCount,
      reps:        firstSet.reps ?? '',
      duration:    e.duration_raw || '',
      weight_lbs:  weightDisplay,
      tempo:       e.tempo        || '',
      rpe:         e.rpe          ?? '',
      pattern:     e.category     || '',
      bias:        e.bias         || '',
      distance_km: e.distance_km  ?? '',
      elevation_m: e.elevation_m  ?? '',
      pace:        e.pace         || '',
      // preserve nested sets for save round-trip
      _sets:       setsArr,
    }
  }

  function blankExercise() {
    return {
      name: '', sets: '', reps: '', duration: '', weight_lbs: '',
      tempo: '', rpe: '', pattern: '', bias: '', distance_km: '', elevation_m: '', pace: '',
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
      const payload = {
        date:            workout.date,
        slot:            String(workout.slot || '1'),
        title:           workout.title,
        duration_min:    Number(workout.duration_min)    || 0,
        calories_burned: Number(workout.calories_burned) || 0,
        raw_notes:       [workout.notes, workout.coach_notes].filter(Boolean).join('\n\n'),
        metadata: {
          type:          workout.type         || '',
          style:         workout.style        || '',
          surface:       workout.surface      || '',
          focus:         workout.focus ? workout.focus.split(',').map(s => s.trim()).filter(Boolean) : [],
          rest_interval: workout.rest_interval || '',
          rpe:           Number(workout.rpe)   || 0,
          avg_hr:        Number(workout.avg_hr) || 0,
          max_hr:        Number(workout.max_hr) || 0,
        },
        exercises: workout.exercises.map(ex => {
          // If we have preserved _sets from a parse result, use them directly
          // Otherwise build sets array from flat form fields
          let sets
          if (Array.isArray(ex._sets) && ex._sets.length > 0) {
            sets = ex._sets
          } else {
            const n = Number(ex.sets) || 1
            const reps = Number(ex.reps) || 0
            const loadNum = parseLoad(ex.weight_lbs)
            // Re-compute TUT from tempo if user typed it in the form
            let tutSeconds = 0
            if (ex.tempo && reps > 0) {
              tutSeconds = ex.tempo.split('-').map(Number).filter(n => !isNaN(n)).reduce((a, b) => a + b, 0) * reps
            }
            sets = Array.from({ length: n }, () => ({
              reps,
              load_kg: inputLoad(loadNum, store.units),
              tut_seconds: tutSeconds,
              rest_seconds: 0,
            }))
          }
          return {
            name:         ex.name     || '',
            category:     ex.pattern  || '',
            bias:         ex.bias     || '',
            tempo:        ex.tempo    || '',
            sets,
            rpe:          ex.rpe !== '' ? Number(ex.rpe) : 0,
            load_raw:     ex.weight_lbs || '',
            duration_raw: ex.duration || '',
            distance_km:  ex.distance_km !== '' ? inputDist(ex.distance_km, store.units) : 0,
            elevation_m:  ex.elevation_m !== '' ? inputElev(ex.elevation_m, store.units) : 0,
            pace:         ex.pace || '',
          }
        }),
      }
      await api.postWorkout(payload)
      success = 'Workout saved!'
      workout = {
        date: today(), slot: '1', title: '', type: 'strength', style: '', surface: '',
        focus: '', rest_interval: '', duration_min: '', rpe: '', avg_hr: '', max_hr: '',
        calories_burned: '', notes: '', coach_notes: '', exercises: [],
      }
      aiRaw = ''; aiParsed = false
      yamlRaw = ''; yamlParsed = false
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
    const load = ex.weight_lbs || ''
    const loadNum = parseLoad(load)

    let base = `${ex.name || 'Exercise'}: ${sets}×${ex.duration || reps}`
    if (load) base += ` @ ${load}`

    const extras = []
    const tut = computeTUT(ex.tempo, reps, sets)
    if (tut) extras.push(`TUT: ${tut}s`)
    const vol = loadNum > 0 ? Math.round(loadNum * (reps > 0 ? reps : 1) * sets) : 0
    if (vol > 0) extras.push(`vol: ${vol}`)
    const pct = epleyPct(loadNum, reps)
    if (pct) extras.push(`${pct}%1RM`)
    if (ex.pace) extras.push(`pace: ${ex.pace}/km`)
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
        return s + (set.load_kg > 0 && set.reps > 0 ? Math.round(set.load_kg * set.reps) : 0)
      }, 0)
    }
    const l = parseLoad(ex.weight_lbs)
    const r = Number(ex.reps) || 0
    const s = Number(ex.sets) || 0
    return acc + (l > 0 ? Math.round(l * r * s) : 0)
  }, 0))

  onMount(() => {
    // Check if editing existing workout
    if (store.editData && store.editData.type === 'workout') {
      const w = store.editData.data
      workout.date = w.date
      workout.slot = w.slot
      workout.title = w.title || ''
      tab = 'simple'  // switch to simple tab for basic editing
      clearEditData()
    }
  })
</script>

<div class="max-w-2xl mx-auto">
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
    <div class="bg-gray-800 p-4 rounded-lg border border-gray-700 space-y-3">
      <label class="text-xs text-gray-400 block" for="wo-ai-input">Describe your workout</label>
      <textarea class="input" id="wo-ai-input" rows="5" placeholder="e.g. 3 sets of 8 KB deadlifts at 85 lbs tempo 2-2-0-2, then 2×10 pushups bodyweight…" bind:value={aiRaw}></textarea>
      <button class="btn-primary" onclick={parseAI} disabled={aiParsing}>{aiParsing ? 'Parsing…' : 'Parse'}</button>

      {#if aiParsed}
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
    <div class="bg-gray-800 p-4 rounded-lg border border-gray-700 space-y-3">
      <label class="text-xs text-gray-400 block" for="wo-yaml-input">Paste YAML workout definition</label>
      <textarea class="input font-mono text-xs" id="wo-yaml-input" rows="10" bind:value={yamlRaw}
        placeholder={'name: "KB Circuit"\ntype: "strength"\nexercises:\n  - name: "KB Deadlift"\n    sets: 3\n    reps: 8\n    load: "85 lbs"\n    tempo: "2-2-0-2"'}></textarea>

      <div>
        <label class="text-xs text-gray-400 block mb-1" for="wo-yaml-coach-notes">Coach's Note</label>
        <textarea class="input" id="wo-yaml-coach-notes" rows="2" placeholder="Context, feedback, how it felt…" bind:value={workout.coach_notes}></textarea>
      </div>

      <button class="btn-primary" onclick={parseYAML} disabled={yamlParsing}>{yamlParsing ? 'Parsing…' : 'Preview / Parse'}</button>

      {#if yamlParsed}
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
  {#if summaryLines.length > 0 || workout.duration_min || workout.calories_burned}
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
        {#if workout.duration_min}<span>Duration: <span class="text-white">{workout.duration_min} min</span></span>{/if}
        {#if totalVol > 0}<span>Total Vol: <span class="text-white">{dispLoad(totalVol, store.units)} {loadUnit(store.units)}</span></span>{/if}
        {#if workout.calories_burned}<span>Cal: <span class="text-white">{workout.calories_burned} kcal</span></span>{/if}
        {#if workout.rpe}<span>RPE: <span class="text-white">{workout.rpe}</span></span>{/if}
        {#if workout.avg_hr}<span>Avg HR: <span class="text-white">{workout.avg_hr} bpm</span></span>{/if}
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
      <input class="input" id="wo-rest" placeholder="e.g. 1 min" bind:value={workout.rest_interval} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-duration">Duration (min)</label>
      <input class="input" id="wo-duration" type="number" bind:value={workout.duration_min} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-rpe">Session RPE</label>
      <input class="input" id="wo-rpe" type="number" min="1" max="10" placeholder="1-10" bind:value={workout.rpe} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-avg-hr">Avg HR (bpm)</label>
      <input class="input" id="wo-avg-hr" type="number" bind:value={workout.avg_hr} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-max-hr">Max HR (bpm)</label>
      <input class="input" id="wo-max-hr" type="number" bind:value={workout.max_hr} />
    </div>
    <div>
      <label class="text-xs text-gray-400" for="wo-cal">Calories burned</label>
      <input class="input" id="wo-cal" type="number" bind:value={workout.calories_burned} />
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
      <textarea class="input" id="wo-coach-notes" rows="2" placeholder="Context, feedback, how it felt…" bind:value={workout.coach_notes}></textarea>
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
            <input class="input" id="ex-{idx}-load" bind:value={ex.weight_lbs} placeholder="BW / 85 lbs" oninput={() => ex._sets = []} />
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
            <input class="input" id="ex-{idx}-distance" type="number" step="0.1" bind:value={ex.distance_km} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-elevation">Elevation ({elevUnit(store.units)})</label>
            <input class="input" id="ex-{idx}-elevation" type="number" bind:value={ex.elevation_m} />
          </div>
        </div>
      </div>
    {/each}
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={addExercise}>+ Add Exercise</button>
  </div>
{/snippet}
