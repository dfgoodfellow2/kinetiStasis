<script>
  import { api } from '../lib/api.js'
  import { store, clearEditData } from '../lib/stores.svelte.js'
  import { onMount } from 'svelte'
  import { saveDraft, loadDraft, clearDraft } from '../lib/draft.js'
  import { today, dispLoad, loadUnit, inputLoad, kgToLbs, lbsToKg, distUnit, inputDist, elevUnit, inputElev } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'
  import AITab from '../components/AITab.svelte'
  import YAMLTab from '../components/YAMLTab.svelte'
  import InfoTip from '../components/InfoTip.svelte'

  // ── Tab (persisted) ─────────────────────────────────────
  const TAB_KEY = 'workout_tab'
  const DRAFT_KEY = 'draft:workoutlog:form'
  const VALID_TABS = ['ai', 'simple', 'yaml']
  const savedTab = typeof localStorage !== 'undefined' ? localStorage.getItem(TAB_KEY) : null
  let tab = $state(VALID_TABS.includes(savedTab) ? savedTab : 'ai')

  $effect(() => {
    localStorage.setItem(TAB_KEY, tab)
  })

  // ── Shared workout state ─────────────────────────────────
  function blankWorkout() {
    return {
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
      // Session metadata that is not edited inline but must round-trip.
      rpe: '',
      recovers: '',
      day: '',
      exercises: [],
    }
  }

  let workout = $state(blankWorkout())

  let loading = $state(false)
  let error = $state('')
  let success = $state('')
  let draftRestored = $state(false)
  // True while the form was populated from an existing workout row.
  let isEdit = $state(false)

  // ── Parse state (set when child components call onParsed) ──
  let parseReady = $state(false)

  function onParsed() {
    // AITab/YAMLTab emit canonical sets (loadKg/loadLbs); convert them into
    // editable rows so the per-set editor shows the parsed detail.
    workout.exercises = workout.exercises.map(normalizeExerciseSets)
    parseReady = true
  }

  function blankExercise() {
    return {
      name: '', sets: '', reps: '', duration: '', weightLbs: '',
      tempo: '', rpe: '', pattern: '', bias: '', pace: '',
      distanceKm: '', elevationM: '',
      phase: '', groupId: '', groupOrder: '', groupStyle: '',
      targetHrZone: '', notes: '', surface: '', met: '',
      _sets: [],
    }
  }

  function addExercise() {
    workout = { ...workout, exercises: [...workout.exercises, blankExercise()] }
  }

  function removeExercise(idx) {
    workout = { ...workout, exercises: workout.exercises.filter((_, i) => i !== idx) }
  }

  // ── Per-set editor helpers ────────────────────────────────
  // A row stores the display string the user typed plus the canonical
  // storage values, so editing one row never touches the others.
  function storedSetRow(s) {
    const loadKg = Number(s?.loadKg) || 0
    const loadLbs = Number(s?.loadLbs) || (loadKg > 0 ? kgToLbs(loadKg) : 0)
    const load = typeof s?.load === 'string' && s.load !== ''
      ? s.load
      : (loadKg > 0 ? String(dispLoad(loadKg, store.units)) : '')
    return {
      reps: s?.reps ? String(s.reps) : '',
      load,
      loadKg,
      loadLbs,
      tutSeconds: Number(s?.tutSeconds) || 0,
      restSeconds: Number(s?.restSeconds) || 0,
    }
  }

  // Seed a new row from the flat (quick) fields when no rows exist yet.
  function flatSetRow(ex) {
    const raw = String(ex?.weightLbs ?? '').trim()
    const loadKg = raw ? (inputLoad(parseLoad(raw), store.units) || 0) : 0
    return {
      reps: ex?.reps !== '' && ex?.reps !== null && ex?.reps !== undefined ? String(ex.reps) : '',
      load: raw,
      loadKg,
      loadLbs: loadKg > 0 ? kgToLbs(loadKg) : 0,
      tutSeconds: 0,
      restSeconds: 0,
    }
  }

  function normalizeExerciseSets(ex) {
    const sets = Array.isArray(ex?._sets) ? ex._sets : []
    return { ...blankExercise(), ...ex, _sets: sets.map(storedSetRow) }
  }

  // Only a change to the SET COUNT restructures rows (add/remove trailing
  // sets, preserving values already entered). Editing reps or load never
  // flattens the array.
  function syncSetCount(ex, rawCount) {
    const n = Math.max(0, Math.floor(Number(rawCount) || 0))
    const rows = Array.isArray(ex._sets) ? ex._sets.slice() : []
    if (n === rows.length) return
    if (n < rows.length) {
      ex._sets = rows.slice(0, n)
      return
    }
    const seed = rows.length > 0 ? rows[rows.length - 1] : flatSetRow(ex)
    while (rows.length < n) rows.push({ ...seed })
    ex._sets = rows
  }

  function addSet(ex) {
    const rows = Array.isArray(ex._sets) ? ex._sets.slice() : []
    const seed = rows.length > 0 ? rows[rows.length - 1] : flatSetRow(ex)
    rows.push({ ...seed })
    ex._sets = rows
    ex.sets = String(rows.length)
  }

  function removeSet(ex, i) {
    const rows = Array.isArray(ex._sets) ? ex._sets.slice() : []
    rows.splice(i, 1)
    ex._sets = rows
    ex.sets = rows.length > 0 ? String(rows.length) : ''
  }

  // The flat reps/load fields act as "apply to every set" shortcuts; they
  // never discard per-set rows.
  function applyRepsToAll(ex, value) {
    ex.reps = value
    if (Array.isArray(ex._sets) && ex._sets.length > 0) {
      ex._sets = ex._sets.map((s) => ({ ...s, reps: value }))
    }
  }

  function applyLoadToAll(ex, value) {
    ex.weightLbs = value
    const loadKg = String(value ?? '').trim() ? (inputLoad(parseLoad(value), store.units) || 0) : 0
    if (Array.isArray(ex._sets) && ex._sets.length > 0) {
      ex._sets = ex._sets.map((s) => ({
        ...s,
        load: value,
        loadKg,
        loadLbs: loadKg > 0 ? kgToLbs(loadKg) : 0,
      }))
    }
  }

  function updateSetLoad(ex, i, value) {
    const row = ex._sets?.[i]
    if (!row) return
    const loadKg = String(value ?? '').trim() ? (inputLoad(parseLoad(value), store.units) || 0) : 0
    row.load = value
    row.loadKg = loadKg
    row.loadLbs = loadKg > 0 ? kgToLbs(loadKg) : 0
  }

  // Draft the manual workout form fields (never image/base64 payloads).
  function hasExerciseContent(ex) {
    if (!ex || typeof ex !== 'object') return false
    if (Array.isArray(ex._sets) && ex._sets.length > 0) return true
    return ['name', 'sets', 'reps', 'duration', 'weightLbs', 'tempo', 'rpe', 'pattern', 'bias', 'pace',
      'distanceKm', 'elevationM', 'phase', 'groupId', 'groupOrder', 'groupStyle', 'targetHrZone', 'notes', 'surface', 'met']
      .some((k) => String(ex[k] ?? '').trim() !== '')
  }

  function hasDraftContent(w) {
    if (!w || typeof w !== 'object') return false
    const sessionFields = ['title', 'style', 'surface', 'focus', 'restInterval', 'durationMin',
      'avgHr', 'maxHr', 'caloriesBurned', 'notes', 'coachNotes']
    if (sessionFields.some((k) => String(w[k] ?? '').trim() !== '')) return true
    return Array.isArray(w.exercises) && w.exercises.some(hasExerciseContent)
  }

  $effect(() => {
    // Never draft an in-progress edit.
    if (isEdit) return
    const snapshot = $state.snapshot(workout)
    if (hasDraftContent(snapshot)) saveDraft(DRAFT_KEY, snapshot)
  })

  function discardDraft() {
    clearDraft(DRAFT_KEY)
    draftRestored = false
    parseReady = false
    isEdit = false
    workout = blankWorkout()
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
        rawNotes:        workout.notes || '',
        metadata: {
          type:          workout.type         || '',
          surface:       workout.surface      || '',
          focus:         workout.focus ? workout.focus.split(',').map(s => s.trim()).filter(Boolean) : [],
          restInterval:  workout.restInterval || '',
          rpe:           Number(workout.rpe)  || 0,
          avgHr:         Number(workout.avgHr) || 0,
          maxHr:         Number(workout.maxHr) || 0,
          recovers:      workout.recovers     || '',
          day:           Number(workout.day)  || 0,
          coachNotes:    workout.coachNotes   || '',
        },
        exercises: workout.exercises.map(ex => {
          // Editing rows are the source of truth: emit one real set per row,
          // so a 10/8/6 pyramid is stored as three different sets.
          let sets
          if (Array.isArray(ex._sets) && ex._sets.length > 0) {
            sets = ex._sets.map(s => {
              const reps = Number(s.reps) || 0
              const loadKg = Number(s.loadKg) || 0
              // Recompute TUT from tempo only when the row carries none.
              let tutSeconds = Number(s.tutSeconds) || 0
              if (tutSeconds <= 0 && ex.tempo && reps > 0) {
                tutSeconds = ex.tempo.split('-').map(Number).filter(n => !isNaN(n)).reduce((a, b) => a + b, 0) * reps
              }
              return {
                reps,
                loadKg,
                loadLbs: loadKg > 0 ? kgToLbs(loadKg) : (Number(s.loadLbs) || 0),
                tutSeconds,
                restSeconds: Number(s.restSeconds) || 0,
              }
            })
          } else {
            // No rows: fall back to the quick fields, one set each.
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
            phase:        ex.phase || '',
            groupId:      ex.groupId || '',
            groupOrder:   Number(ex.groupOrder) || 0,
            groupStyle:   ex.groupStyle || '',
            targetHrZone: Number(ex.targetHrZone) || 0,
            hrZoneTime:   ex.hrZoneTime || null,
            metValue:     Number(ex.met) || 0,
            notes:        ex.notes || '',
            surface:      ex.surface || '',
          }
        }),
      }
      await api.postWorkout(payload)
      success = 'Workout saved!'
      clearDraft(DRAFT_KEY)
      draftRestored = false
      isEdit = false
      workout = blankWorkout()
      // reset parse states in parent — child components manage their own raw/parsed state
      parseReady = false
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

  function tutSummary(ex) {
    if (!ex.tempo) return null
    const parts = String(ex.tempo).split('-').map(Number)
    if (parts.length < 2 || parts.some(isNaN)) return null
    const perRep = parts.reduce((a, b) => a + b, 0)
    const totalReps = (Array.isArray(ex._sets) && ex._sets.length > 0)
      ? ex._sets.reduce((acc, s) => acc + (Number(s.reps) || 0), 0)
      : (Number(ex.reps) || 0) * (Number(ex.sets) || 0)
    const tut = perRep * totalReps
    return tut > 0 ? tut : null
  }

  function epleyPct(load, reps) {
    const l = Number(load) || 0; const r = Number(reps) || 0
    if (l <= 0 || r <= 0) return null
    return Math.round((l / (l * (1 + r / 30))) * 100)
  }

  function exerciseLine(ex) {
    const hasRows = Array.isArray(ex._sets) && ex._sets.length > 0
    const rowReps = hasRows ? ex._sets.map(s => Number(s.reps) || 0) : []
    const sets = hasRows ? ex._sets.length : (Number(ex.sets) || 0)
    const reps = hasRows ? (rowReps[0] || 0) : (Number(ex.reps) || 0)
    // Show the real rep scheme: "10" when uniform, "10/8/6" when it varies.
    const repsText = hasRows && rowReps.some(r => r !== reps)
      ? rowReps.join('/')
      : reps
    // Load: the quick field wins, else the heaviest per-set load.
    const setLoads = hasRows ? ex._sets.map(s => Number(s.loadKg) || 0).filter(v => v > 0) : []
    const load = ex.weightLbs
      || (setLoads.length > 0 ? `${dispLoad(Math.max(...setLoads), store.units)} ${loadUnit(store.units)}` : '')
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
    const displayValue = (reps <= 0 && hasDuration) ? durationValue : repsText
    let base = `${ex.name || 'Exercise'}: ${sets}×${displayValue}`
    if (load) base += ` @ ${load}`

    const extras = []
    const tut = tutSummary(ex)
    if (tut) extras.push(`TUT: ${Math.round(tut)}s`)
    const totalReps = hasRows
      ? rowReps.reduce((a, r) => a + r, 0)
      : (reps > 0 ? reps : 1) * sets
    const vol = loadNum > 0 ? Math.round(loadNum * totalReps) : 0
    if (vol > 0) extras.push(`vol: ${vol}`)
    const pct = epleyPct(loadNum, reps)
    if (pct) extras.push(`${pct}%1RM`)
    if (ex.pace) extras.push(`pace: ${ex.pace}`)
    if (extras.length) base += ` (${extras.join(', ')})`
    if (ex.pattern) {
      const biasTag = ex.bias === 'bilateral' ? '(B)' : ex.bias === 'unilateral' ? '(U)' : ''
      base += ` [${ex.pattern}${biasTag ? ' ' + biasTag : ''}]`
    }
    if (ex.phase) base += ` [${ex.phase}]`
    if (ex.groupId) base += ` [${ex.groupId} #${ex.groupOrder || '?'}]`
    if (ex.targetHrZone) base += ` [Zone ${ex.targetHrZone}]`
    if (ex.hrZoneTime && typeof ex.hrZoneTime === 'object') {
      const parts = []
      for (let z = 1; z <= 5; z++) {
        if (ex.hrZoneTime[z] > 0) {
          parts.push(`Z${z}:${ex.hrZoneTime[z]}s`)
        }
      }
      if (parts.length > 0) base += ` [${parts.join(' ')}]`
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
      // Mark the edit before populating the form so the draft effect can never
      // snapshot the row being edited.
      isEdit = true
      // An explicit edit supersedes any half-finished manual draft.
      clearDraft(DRAFT_KEY)
      draftRestored = false

      const w = store.editData.data

      // Basic fields
      workout.date = w.date || today()
      workout.slot = String(w.slot || '1')
      workout.title = w.title || ''
      // Session type/surface live under metadata in the API model, not at the
      // top level; fall back to top-level fields for older payloads.
      workout.type = w.metadata?.type || w.type || 'strength'
      workout.style = w.metadata?.style || w.style || ''
      workout.surface = w.metadata?.surface || w.surface || ''
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
      workout.coachNotes = w.metadata?.coachNotes ?? ''
      // These parse-time session fields are not shown as inputs but must be
      // carried through an edit so saving does not erase them.
      workout.rpe = w.metadata?.rpe ? String(w.metadata.rpe) : ''
      workout.recovers = w.metadata?.recovers ?? ''
      workout.day = w.metadata?.day ? String(w.metadata.day) : ''

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
          
          // Strength: sets/reps/weight (quick fields; rows carry the real sets)
          sets: exerciseSets.length > 0 ? String(exerciseSets.length) : '',
          reps: firstExerciseSet.reps ? String(firstExerciseSet.reps) : '',
          weightLbs: firstExerciseSet.loadKg
            ? String(dispLoad(firstExerciseSet.loadKg, store.units))
            : (firstExerciseSet.loadLbs ? String(Math.round(firstExerciseSet.loadLbs)) : ''),
          
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

          // New fields
          phase:        ex.phase        || '',
          groupId:      ex.groupId      || '',
          groupOrder:   ex.groupOrder   ?? '',
          groupStyle:   ex.groupStyle   || '',
          targetHrZone: ex.targetHrZone ?? '',
          hrZoneTime:   ex.hrZoneTime   || null,
          surface:      ex.surface      || '',
          met:          ex.metValue     ?? '',

          // _sets array preserves every set (reps + load per set) for the
          // per-set editor and the save path.
          _sets: exerciseSets.map(storedSetRow)
        }
      })

      tab = 'simple'
      clearEditData()
    } else {
      // Restore a half-finished manual form. Deep-merge each exercise with
      // blankExercise() so restored entries keep the full shape the form and
      // save payload expect (newer fields are absent from older drafts).
      const draft = loadDraft(DRAFT_KEY)
      if (hasDraftContent(draft)) {
        workout = {
          ...blankWorkout(),
          ...draft,
          exercises: Array.isArray(draft.exercises)
            ? draft.exercises.map((ex) => normalizeExerciseSets(ex))
            : [],
        }
        draftRestored = true
        tab = 'simple'
      }
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

  {#if draftRestored}
    <div class="text-xs text-gray-400">
      Draft restored — <button class="text-cyan-400 hover:text-cyan-300 underline" onclick={discardDraft}>Discard?</button>
    </div>
  {/if}

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
    <div class="bg-gray-800 p-4 rounded-lg border border-gray-700 space-y-3">
      <YAMLTab bind:workout={workout} onParsed={onParsed} />
      <div>
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
      <div class="flex items-center gap-1">
        <label class="text-xs text-gray-400" for="wo-cal">Calories burned</label>
        <InfoTip label="calories burned" text="Leave 0 and the app estimates the burn for you (MET-based) from the exercises and duration." />
      </div>
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
            <input class="input" id="ex-{idx}-sets" type="number" min="0" value={ex.sets}
              oninput={(e) => { ex.sets = e.currentTarget.value; syncSetCount(ex, e.currentTarget.value) }} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-reps">Reps (all sets)</label>
            <input class="input" id="ex-{idx}-reps" type="number" min="0" value={ex.reps}
              oninput={(e) => applyRepsToAll(ex, e.currentTarget.value)} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-duration">Duration (timed)</label>
            <input class="input" id="ex-{idx}-duration" bind:value={ex.duration} placeholder="35 sec" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-load">Load ({loadUnit(store.units)}, all sets)</label>
            <input class="input" id="ex-{idx}-load" value={ex.weightLbs} placeholder="BW / 85"
              oninput={(e) => applyLoadToAll(ex, e.currentTarget.value)} />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-tempo">Tempo</label>
            <input class="input" id="ex-{idx}-tempo" bind:value={ex.tempo} placeholder="2-0-2-0" />
          </div>
          <div>
            <div class="flex items-center gap-1">
              <label class="text-xs text-gray-400" for="ex-{idx}-rpe">RPE</label>
              <InfoTip label="RPE" text="Rate of Perceived Exertion, 1-10: how hard it FELT. 5-6 = could keep going, 8 = a few reps left, 10 = nothing left." />
            </div>
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
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-phase">Phase</label>
            <select class="input" id="ex-{idx}-phase" bind:value={ex.phase}>
              <option value="">Main</option>
              <option value="warmup">Warmup</option>
              <option value="cooldown">Cooldown</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-group-id">Group ID</label>
            <input class="input" id="ex-{idx}-group-id" bind:value={ex.groupId} placeholder="optional group" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-group-order">Group Order</label>
            <input class="input" id="ex-{idx}-group-order" type="number" bind:value={ex.groupOrder} placeholder="order" />
          </div>
          <div>
            <label class="text-xs text-gray-400" for="ex-{idx}-target-hr">Target HR Zone</label>
            <select class="input" id="ex-{idx}-target-hr" bind:value={ex.targetHrZone}>
              <option value="">—</option>
              <option value="1">Zone 1</option>
              <option value="2">Zone 2</option>
              <option value="3">Zone 3</option>
              <option value="4">Zone 4</option>
              <option value="5">Zone 5</option>
            </select>
          </div>
          <div class="col-span-3">
            <label class="text-xs text-gray-400" for="ex-{idx}-notes">Notes</label>
            <textarea class="input" id="ex-{idx}-notes" rows="1" placeholder="Exercise notes…" bind:value={ex.notes}></textarea>
          </div>
        </div>
        <div class="mt-3 border-t border-gray-600 pt-2">
          <div class="flex items-center justify-between mb-1">
            <span class="text-xs text-gray-400">
              Set detail — reps and load per set
              {#if ex._sets && ex._sets.length > 0}({ex._sets.length}){/if}
            </span>
            <button type="button" class="text-xs text-emerald-400 hover:text-emerald-300" onclick={() => addSet(ex)}>+ Add set</button>
          </div>
          {#if ex._sets && ex._sets.length > 0}
            <div class="space-y-1">
              {#each ex._sets as row, sIdx}
                <div class="flex items-center gap-2">
                  <span class="text-xs text-gray-500 w-6 shrink-0">#{sIdx + 1}</span>
                  <div class="w-20 shrink-0">
                    <input class="input" type="number" min="0" placeholder="reps"
                      aria-label="Set {sIdx + 1} reps" bind:value={row.reps} />
                  </div>
                  <div class="w-28 shrink-0">
                    <input class="input" placeholder={loadUnit(store.units)}
                      aria-label="Set {sIdx + 1} load"
                      value={row.load}
                      oninput={(e) => updateSetLoad(ex, sIdx, e.currentTarget.value)} />
                  </div>
                  <button type="button" class="text-xs text-gray-500 hover:text-red-400 px-1"
                    aria-label="Remove set {sIdx + 1}" onclick={() => removeSet(ex, sIdx)}>✕</button>
                </div>
              {/each}
            </div>
          {:else}
            <p class="text-xs text-gray-500">No individual sets yet — enter a set count or add a set.</p>
          {/if}
        </div>
      </div>
    {/each}
    <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={addExercise}>+ Add Exercise</button>
  </div>
{/snippet}
